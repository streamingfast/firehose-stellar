// Package devstack manages the test chain (stellar/quickstart docker
// container) from Go. TestMain calls Reset/Up before scenarios run and
// Down when they finish, so plain `go test` drives the full lifecycle.
//
// The actual containers + Dockerfile + entrypoint config live under
// test/scripts/dev/. This package shells `docker compose` via
// exec.Command — same orchestration the bash scripts do, just in Go.
package devstack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Config controls the quickstart lifecycle. Optional fields have
// reasonable defaults — call DefaultConfig() and tweak.
type Config struct {
	// ComposeFile is the absolute path to test/scripts/dev/docker-compose.yml.
	// Empty triggers an upward search from cwd.
	ComposeFile string

	// ProjectName maps to docker compose's COMPOSE_PROJECT_NAME — also
	// the prefix for container names and the network name.
	ProjectName string

	// DataRoot is the host dir used for compose logs + captive-core
	// working files. Defaults to <testRoot>/.data, where <testRoot> is
	// the directory three levels above ComposeFile — i.e. the repo's
	// test/ dir when ComposeFile is test/scripts/dev/docker-compose.yml.
	// Not the repo root; the name reflects the actual derivation.
	DataRoot string

	// QuickstartImage overrides docker-compose.yml's
	// stellar/quickstart:testing default.
	QuickstartImage string

	// HorizonPort is where the host reaches quickstart's horizon /
	// soroban-rpc / friendbot.
	HorizonPort int

	// SorobanReadyTimeout caps how long Up waits for soroban-rpc and
	// friendbot to come online.
	SorobanReadyTimeout time.Duration

	// PollInterval is the poll cadence for the readiness probes.
	PollInterval time.Duration

	// Debug streams docker compose's stdout/stderr to Stderr instead
	// of writing it to LogFile.
	Debug bool

	// LogFile collects compose stdout/stderr when Debug=false.
	// Defaults to <DataRoot>/compose.log. Truncated on each Up().
	LogFile string

	// Stdout / Stderr receive progress lines (`>> starting quickstart`).
	Stdout io.Writer
	Stderr io.Writer
}

// DefaultConfig returns a Config with timeout/output defaults filled.
// ComposeFile + DataRoot resolved at New() time.
func DefaultConfig() Config {
	return Config{
		ProjectName:         "battlefield-stellar",
		HorizonPort:         8000,
		SorobanReadyTimeout: 120 * time.Second,
		PollInterval:        2 * time.Second,
		Stdout:              os.Stderr,
		Stderr:              os.Stderr,
	}
}

// Stack drives one quickstart lifecycle.
type Stack struct {
	cfg      Config
	testRoot string
}

// New validates cfg, resolves ComposeFile + DataRoot + LogFile if blank,
// and returns a ready Stack.
func New(cfg Config) (*Stack, error) {
	if cfg.ProjectName == "" {
		cfg.ProjectName = "battlefield-stellar"
	}
	if cfg.HorizonPort == 0 {
		cfg.HorizonPort = 8000
	}
	if cfg.SorobanReadyTimeout == 0 {
		cfg.SorobanReadyTimeout = 120 * time.Second
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stderr
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}

	if cfg.ComposeFile == "" {
		found, err := findComposeFile()
		if err != nil {
			return nil, err
		}
		cfg.ComposeFile = found
	}

	testRoot := filepath.Dir(filepath.Dir(filepath.Dir(cfg.ComposeFile)))
	if cfg.DataRoot == "" {
		cfg.DataRoot = filepath.Join(testRoot, ".data")
	}
	if err := os.MkdirAll(cfg.DataRoot, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir data root: %w", err)
	}
	if cfg.LogFile == "" {
		cfg.LogFile = filepath.Join(cfg.DataRoot, "compose.log")
	}

	return &Stack{cfg: cfg, testRoot: testRoot}, nil
}

// ComposeFile returns the resolved docker-compose.yml path.
func (s *Stack) ComposeFile() string { return s.cfg.ComposeFile }

// DataRoot returns the resolved host data directory.
func (s *Stack) DataRoot() string { return s.cfg.DataRoot }

// Up starts quickstart and waits for soroban-rpc + friendbot to come
// online. Idempotent.
func (s *Stack) Up(ctx context.Context) error {
	if err := s.requireDocker(ctx); err != nil {
		return err
	}
	if err := s.truncateLogFile(); err != nil {
		return err
	}

	s.logf(">> starting quickstart (logs: %s)", s.cfg.LogFile)
	if err := s.compose(ctx, "up", "-d", "--build", "quickstart"); err != nil {
		return fmt.Errorf("compose up quickstart: %w", err)
	}

	if _, err := s.waitForSoroban(ctx); err != nil {
		return err
	}

	// Friendbot lives behind horizon's reverse proxy; the proxy returns
	// 502 Bad Gateway for ~10-30s after a fresh boot even though
	// soroban-rpc is already ticking. Wait until friendbot answers
	// with its own 4xx (not a gateway error).
	if err := s.waitForFriendbot(ctx); err != nil {
		return err
	}

	s.logf(">> quickstart ready (horizon http://localhost:%d)", s.cfg.HorizonPort)
	return nil
}

// Down tears quickstart down. Idempotent.
func (s *Stack) Down(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		s.logf(">> docker not present; nothing to do")
		return nil
	}
	s.logf(">> stopping quickstart")
	return s.compose(ctx, "down", "--remove-orphans")
}

// Reset is Down + Up. Used between runs so each scenario gets a clean
// deterministic chain.
func (s *Stack) Reset(ctx context.Context) error {
	_ = s.Down(ctx)
	return s.Up(ctx)
}

// IsRunning reports whether the quickstart container is currently up.
func (s *Stack) IsRunning(ctx context.Context) (bool, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return false, nil
	}
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", s.cfg.ComposeFile, "ps", "-q", "--status=running")
	cmd.Env = append(os.Environ(), s.composeEnv()...)
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return false, nil // treat any failure as "not running"
	}
	return strings.TrimSpace(out.String()) != "", nil
}

func (s *Stack) requireDocker(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found: install Docker Desktop or the docker CLI")
	}
	cmd := exec.CommandContext(ctx, "docker", "compose", "version")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("'docker compose' plugin not found: %w", err)
	}
	return nil
}

func (s *Stack) truncateLogFile() error {
	f, err := os.Create(s.cfg.LogFile)
	if err != nil {
		return fmt.Errorf("truncate log file: %w", err)
	}
	return f.Close()
}

// composeEnv is the env-var slice docker compose needs.
func (s *Stack) composeEnv() []string {
	env := []string{
		"COMPOSE_PROJECT_NAME=" + s.cfg.ProjectName,
		"BATTLEFIELD_DATA_DIR=" + s.cfg.DataRoot,
	}
	if s.cfg.QuickstartImage != "" {
		env = append(env, "QUICKSTART_IMAGE="+s.cfg.QuickstartImage)
	}
	if s.cfg.HorizonPort != 0 && s.cfg.HorizonPort != 8000 {
		env = append(env, "QUICKSTART_HORIZON_PORT="+strconv.Itoa(s.cfg.HorizonPort))
	}
	return env
}

// compose runs `docker compose <args>`, routing output to the log file
// (or stderr when Debug=true).
func (s *Stack) compose(ctx context.Context, args ...string) error {
	full := append([]string{"compose", "-f", s.cfg.ComposeFile}, args...)
	cmd := exec.CommandContext(ctx, "docker", full...)
	cmd.Env = append(os.Environ(), s.composeEnv()...)

	if s.cfg.Debug {
		cmd.Stdout = s.cfg.Stderr
		cmd.Stderr = s.cfg.Stderr
		return cmd.Run()
	}

	logF, err := os.OpenFile(s.cfg.LogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer logF.Close()
	cmd.Stdout = logF
	cmd.Stderr = logF
	return cmd.Run()
}

// waitForSoroban polls getLatestLedger until the chain is producing
// ledgers (>=2). Returns the latest ledger seen.
func (s *Stack) waitForSoroban(ctx context.Context) (uint64, error) {
	deadline := time.Now().Add(s.cfg.SorobanReadyTimeout)
	url := fmt.Sprintf("http://localhost:%d/soroban/rpc", s.cfg.HorizonPort)
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"getLatestLedger"}`)

	s.log(">> waiting for soroban-rpc")
	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		latest, err := sorobanLatestLedger(ctx, url, body)
		if err == nil && latest >= 2 {
			s.logf(" ✓ (ledger %d)", latest)
			return latest, nil
		}
		lastErr = err
		s.log(".")
		time.Sleep(s.cfg.PollInterval)
	}
	return 0, fmt.Errorf("soroban-rpc didn't come online within %s (last err: %v); see %s",
		s.cfg.SorobanReadyTimeout, lastErr, s.cfg.LogFile)
}

// waitForFriendbot polls horizon's friendbot endpoint until it returns
// a non-5xx response. 502 means horizon's reverse proxy can't reach
// friendbot yet; 4xx means friendbot is actually responding.
func (s *Stack) waitForFriendbot(ctx context.Context) error {
	deadline := time.Now().Add(s.cfg.SorobanReadyTimeout)
	url := fmt.Sprintf("http://localhost:%d/friendbot", s.cfg.HorizonPort)

	s.log(">> waiting for friendbot")
	var lastStatus int
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err == nil {
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				lastStatus = resp.StatusCode
				resp.Body.Close()
				if resp.StatusCode < 500 {
					s.logf(" ✓ (status %d)", resp.StatusCode)
					return nil
				}
			}
		}
		s.log(".")
		time.Sleep(s.cfg.PollInterval)
	}
	return fmt.Errorf("friendbot didn't come online within %s (last status: %d); see %s",
		s.cfg.SorobanReadyTimeout, lastStatus, s.cfg.LogFile)
}

func sorobanLatestLedger(ctx context.Context, url string, body []byte) (uint64, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	var out struct {
		Result struct {
			Sequence uint64 `json:"sequence"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	return out.Result.Sequence, nil
}

// findComposeFile locates test/scripts/dev/docker-compose.yml. Tries:
//  1. walking upward from cwd for test/scripts/dev/docker-compose.yml
//  2. walking upward from cwd for scripts/dev/docker-compose.yml (when
//     cwd is already inside test/, since go test sets cwd to pkg dir)
//  3. runtime.Caller fallback (this file's known location)
func findComposeFile() (string, error) {
	if cwd, err := os.Getwd(); err == nil {
		if found := walkUpFor(cwd, filepath.Join("test", "scripts", "dev", "docker-compose.yml")); found != "" {
			return found, nil
		}
		if found := walkUpFor(cwd, filepath.Join("scripts", "dev", "docker-compose.yml")); found != "" {
			return found, nil
		}
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		// file is .../test/lib/devstack/devstack.go — test root three up.
		testRoot := filepath.Dir(filepath.Dir(filepath.Dir(file)))
		candidate := filepath.Join(testRoot, "scripts", "dev", "docker-compose.yml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not locate test/scripts/dev/docker-compose.yml")
}

func walkUpFor(start, rel string) string {
	dir := start
	for {
		candidate := filepath.Join(dir, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func (s *Stack) log(msg string)          { fmt.Fprint(s.cfg.Stdout, msg) }
func (s *Stack) logf(f string, a ...any) { fmt.Fprintf(s.cfg.Stdout, f+"\n", a...) }
