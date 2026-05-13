// Cobra wrapper around the captivecore package. All meaningful logic
// lives in github.com/streamingfast/firehose-stellar/captivecore — this
// file just parses flags into captivecore.Config and runs the
// PrepareRange + GetBlock loop that firecore expects.
package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/streamingfast/cli/sflags"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-core/blockpoller"
	"github.com/streamingfast/firehose-stellar/captivecore"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

func NewFetchCaptiveCoreCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "captive-core <first-streamable-block>",
		Short: "fetch blocks from stellar captive core",
		Args:  cobra.ExactArgs(1),
		RunE:  fetchCaptiveCoreRunE(logger, tracer),
	}

	cmd.Flags().String("stellar-core-bin", "/usr/bin/stellar-core", "path to stellar-core binary")
	cmd.Flags().String("stellar-core-conf", "", "path to stellar-core config file (empty = use bundled SDF default for the network; required for custom)")
	cmd.Flags().String("stellar-core-network", "testnet", "stellar network (mainnet, testnet, or custom)")
	cmd.Flags().String("stellar-core-network-passphrase", "", "override network passphrase (required for custom; overrides the value derived from --stellar-core-network when set)")
	cmd.Flags().StringSlice("stellar-core-history-archive-urls", nil, "override history archive URLs (required for custom; overrides the values derived from --stellar-core-network when set)")
	cmd.Flags().String("stellar-core-log-level", "info", "log level for stellar-core subprocess (debug, info, warn, error)")

	return cmd
}

func fetchCaptiveCoreRunE(logger *zap.Logger, _ logging.Tracer) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) error {
		startBlock, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse first streamable block %s: %w", args[0], err)
		}

		logLevel, err := parseStellarCoreLogLevel(sflags.MustGetString(cmd, "stellar-core-log-level"))
		if err != nil {
			return err
		}

		// Build the captivecore.Config from flags. ResolveNetwork fills
		// defaults for mainnet/testnet; explicit overrides still win
		// because we re-apply them after the call.
		cfg := captivecore.Config{
			BinaryPath:          sflags.MustGetString(cmd, "stellar-core-bin"),
			StellarCoreConfPath: sflags.MustGetString(cmd, "stellar-core-conf"),
			LogLevel:            logLevel,
			Logger:              logger,
		}
		if err := cfg.ResolveNetwork(sflags.MustGetString(cmd, "stellar-core-network")); err != nil {
			return err
		}
		if pass := sflags.MustGetString(cmd, "stellar-core-network-passphrase"); pass != "" {
			cfg.NetworkPassphrase = pass
		}
		if urls := sflags.MustGetStringSlice(cmd, "stellar-core-history-archive-urls"); len(urls) > 0 {
			cfg.HistoryArchiveURLs = urls
		}

		// For custom networks, the bundled toml data is nil. The user
		// must supply --stellar-core-conf in that case (captivecore
		// validation also enforces this).
		if cfg.StellarCoreConfPath == "" && cfg.DefaultTomlData == nil {
			return fmt.Errorf("--stellar-core-conf is required for custom network (no bundled default)")
		}

		backend, err := captivecore.New(cfg)
		if err != nil {
			return err
		}
		defer backend.Close()

		handler := blockpoller.NewFireBlockHandler("type.googleapis.com/sf.stellar.type.v1.Block")
		handler.Init()

		ctx := cmd.Context()
		if err := backend.PrepareRange(ctx, startBlock); err != nil {
			return err
		}

		seq := startBlock
		for {
			if err := ctx.Err(); err != nil {
				return err
			}

			blk, err := backend.GetBlock(ctx, seq)
			if err != nil {
				return fmt.Errorf("get block %d: %w", seq, err)
			}

			logger.Info("processing block", zap.Uint64("seq", seq), zap.String("hash", blk.Id))
			if err := handler.Handle(blk); err != nil {
				return fmt.Errorf("handling block %d: %w", blk.Number, err)
			}

			seq++
		}
	}
}

// parseStellarCoreLogLevel translates the CLI flag string into a
// logrus.Level. Kept here so the cmd shim is self-contained.
func parseStellarCoreLogLevel(s string) (logrus.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return logrus.DebugLevel, nil
	case "info":
		return logrus.InfoLevel, nil
	case "warn", "warning":
		return logrus.WarnLevel, nil
	case "error":
		return logrus.ErrorLevel, nil
	default:
		return 0, fmt.Errorf("invalid stellar-core log level %q (want debug|info|warn|error)", s)
	}
}
