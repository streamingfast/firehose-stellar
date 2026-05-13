// main_test.go drives the dev-stack lifecycle from `go test`.
//
// Both fetchers (rpc poller + captive-core) run in-process via the
// rpc and captivecore library packages. The only container managed
// here is stellar/quickstart — the chain itself.
//
// stellar-core (the C++ binary) is required on PATH for the captive-
// core fetcher. macOS: `brew install stellar/sdf/stellar-core`. Linux:
// SDF apt repo.
//
// Env vars:
//
//	BATTLEFIELD_MANAGE_STACK=0   skip docker-compose lifecycle entirely
//	                             (assume quickstart already running)
//	AUTO_RESET=0                 reuse the current chain instead of
//	                             restarting quickstart for a clean slate
//	KEEP_RUNNING=1               leave quickstart up after tests finish
//	SKIP_TESTS=1                 bring quickstart up, exit without
//	                             running tests
//	STELLAR_CORE_BIN=<path>      override stellar-core binary location
//	                             (default: exec.LookPath("stellar-core"))
//	DEBUG=1                      stream docker compose output to stderr
package scenarios

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/streamingfast/firehose-stellar/captivecore"
	"github.com/streamingfast/firehose-stellar/test/lib/devstack"
	"github.com/streamingfast/firehose-stellar/test/lib/firehose"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// sharedFetchers is populated by TestMain and consumed by newRunner. Both
// in-process fetchers are constructed once per `go test` invocation and
// shared across every scenario — important for captive-core, whose
// stellar-core subprocess startup costs 5-30s.
var sharedFetchers []firehose.Fetcher

// standaloneNetworkPassphrase is the passphrase stellar/quickstart uses
// in --local mode. Pinned here so the in-process fetchers can recompute
// transaction hashes — must match the chain or all hash assertions break.
const standaloneNetworkPassphrase = "Standalone Network ; February 2017"

// rpcEndpoint is where the in-process rpc fetcher reaches soroban-rpc.
// Goes through the port the quickstart container exposes (8000 by
// default, see devstack.Config.HorizonPort).
const rpcEndpoint = "http://localhost:8000/soroban/rpc"

// historyArchiveURL is the SDF history server quickstart runs. Captive-
// core needs at least one archive URL to catch up — for a local
// standalone network, this is the bundled archive on the quickstart
// container.
const historyArchiveURL = "http://localhost:1570"

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	ctx := context.Background()

	// Stack lifecycle. With MANAGE_STACK=0 the caller is responsible
	// for keeping quickstart running.
	if envBool("BATTLEFIELD_MANAGE_STACK", true) {
		cfg := devstack.DefaultConfig()
		cfg.Debug = envBool("DEBUG", false)

		stack, err := devstack.New(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "devstack init: %v\n", err)
			return 2
		}

		exitCode := 2
		defer func() {
			if envBool("KEEP_RUNNING", false) {
				fmt.Fprintln(os.Stderr, ">> KEEP_RUNNING=1: leaving quickstart up")
				return
			}
			if err := stack.Down(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "quickstart down: %v\n", err)
			}
			_ = exitCode
		}()

		if envBool("AUTO_RESET", true) {
			fmt.Fprintln(os.Stderr, ">> resetting quickstart (clean chain)")
			if err := stack.Reset(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "quickstart reset: %v\n", err)
				return 2
			}
		} else {
			running, _ := stack.IsRunning(ctx)
			if running {
				fmt.Fprintln(os.Stderr, ">> reusing running quickstart")
			} else {
				fmt.Fprintln(os.Stderr, ">> bringing up quickstart")
				if err := stack.Up(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "quickstart up: %v\n", err)
					return 2
				}
			}
		}

		if envBool("SKIP_TESTS", false) {
			fmt.Fprintln(os.Stderr, ">> SKIP_TESTS=1, exiting after quickstart boot")
			return 0
		}

		exitCode = runWithFetchers(m)
		return exitCode
	}

	// MANAGE_STACK=0 path: trust quickstart is already up.
	return runWithFetchers(m)
}

// runWithFetchers constructs the two in-process fetchers (rpc + captive-
// core), publishes them to scenarios via the sharedFetchers package var,
// runs the suite, and tears the fetchers down on exit. Quickstart must
// already be healthy at this point.
func runWithFetchers(m *testing.M) int {
	// Redirect chatty fetcher logs (zap from rpc.Fetcher, logrus from
	// stellar-core subprocess) to a file under .data/. Tail it from
	// another terminal if you need to debug:
	//   tail -f test/.data/fetchers.log
	logFile, err := openFetcherLogFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "open fetcher log file: %v\n", err)
		return 2
	}
	// Don't defer Close — stellar-core subprocess flushes log lines
	// asynchronously after our Close() returns, racing with this. OS
	// reclaims the fd on process exit anyway.
	fmt.Fprintf(os.Stderr, ">> fetcher logs → %s\n", logFile.Name())

	logger := newFileLogger(logFile)

	rpcF, err := firehose.NewInProcessRPCFetcher(firehose.InProcessRPCConfig{
		Name:              "poller",
		RPCEndpoint:       rpcEndpoint,
		NetworkPassphrase: standaloneNetworkPassphrase,
		Logger:            logger,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "build in-process rpc fetcher: %v\n", err)
		return 2
	}

	ccBin, err := resolveStellarCoreBinary()
	if err != nil {
		// stellar-core unavailable on host → poller only. Tests will
		// skip the cross-fetcher diff (only 1 fetcher configured) but
		// snapshot assertion still runs.
		fmt.Fprintf(os.Stderr, ">> stellar-core not found (%v); running poller only\n", err)
		sharedFetchers = []firehose.Fetcher{rpcF}
		defer rpcF.Close()
		return m.Run()
	}

	ccF, err := firehose.NewInProcessCaptiveCoreFetcher("captive-core", captivecore.Config{
		BinaryPath:          ccBin,
		NetworkPassphrase:   standaloneNetworkPassphrase,
		HistoryArchiveURLs:  []string{historyArchiveURL},
		StellarCoreConfPath: lookupFollowerToml(),
		// Keep stellar-core's working files (sqlite db, buckets, tmp)
		// under test/.data/captive-core/ alongside the chain state.
		StoragePath: captiveCoreStoragePath(),
		LogOutput:   logFile,
		Logger:      logger,
	})
	if err != nil {
		// Non-fatal — fall back to poller only.
		fmt.Fprintf(os.Stderr, ">> captive-core fetcher init failed (%v); running poller only\n", err)
		sharedFetchers = []firehose.Fetcher{rpcF}
		defer rpcF.Close()
		return m.Run()
	}

	sharedFetchers = []firehose.Fetcher{rpcF, ccF}
	defer func() {
		_ = rpcF.Close()
		_ = ccF.Close()
	}()
	return m.Run()
}

// openFetcherLogFile creates (or truncates) the file where stellar-core
// and rpc.Fetcher zap logs land. Lives under <data-root>/fetchers.log
// so it sits next to compose.log and captive-core/ working files.
func openFetcherLogFile() (*os.File, error) {
	dataRoot := os.Getenv("BATTLEFIELD_DATA_DIR")
	if dataRoot == "" {
		dataRoot = "../.data"
	}
	abs, err := filepath.Abs(dataRoot)
	if err != nil {
		abs = dataRoot
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return os.Create(filepath.Join(abs, "fetchers.log"))
}

// newFileLogger builds a zap logger that writes to the given file in
// dev-friendly text format (timestamps, level colors stripped).
func newFileLogger(w *os.File) *zap.Logger {
	encCfg := zap.NewDevelopmentEncoderConfig()
	encCfg.EncodeLevel = zapcore.CapitalLevelEncoder
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encCfg),
		zapcore.AddSync(w),
		zapcore.DebugLevel,
	)
	return zap.New(core)
}

// resolveStellarCoreBinary picks the stellar-core binary location. Env
// override wins; otherwise consult $PATH.
func resolveStellarCoreBinary() (string, error) {
	if v := os.Getenv("STELLAR_CORE_BIN"); v != "" {
		return v, nil
	}
	return exec.LookPath("stellar-core")
}

// captiveCoreStoragePath returns the data root that ledgerbackend will
// use as the parent for its `captive-core/` working directory. The SDK
// always appends `/captive-core` to whatever path is passed, so we hand
// over .data directly and end up with .data/captive-core/ — not
// .data/captive-core/captive-core/.
func captiveCoreStoragePath() string {
	dataRoot := os.Getenv("BATTLEFIELD_DATA_DIR")
	if dataRoot == "" {
		// go test sets cwd to package dir (test/scenarios). Resolve
		// ../.data which sits at test/.data, matching devstack.
		dataRoot = "../.data"
	}
	abs, err := filepath.Abs(dataRoot)
	if err != nil {
		return dataRoot
	}
	_ = os.MkdirAll(abs, 0o755)
	return abs
}

// lookupFollowerToml returns the path to a captive-core toml that peers
// with quickstart from the host (localhost + docker-compose-exposed
// ports). go test sets cwd to the package dir, so the canonical
// relative path is ../scripts/dev/configs/...
func lookupFollowerToml() string {
	for _, candidate := range []string{
		"../scripts/dev/configs/captive-core-follower-host.cfg",
		"./scripts/dev/configs/captive-core-follower-host.cfg",
		"test/scripts/dev/configs/captive-core-follower-host.cfg",
	} {
		if _, err := os.Stat(candidate); err == nil {
			if abs, err := filepath.Abs(candidate); err == nil {
				return abs
			}
			return candidate
		}
	}
	return ""
}

// envBool parses a 0/1 (or true/false) env var. Missing or unparseable
// → fallback.
func envBool(name string, fallback bool) bool {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
