// Package fix carries one-shot maintenance commands. fix-block-hashes
// rewrites merged-blocks produced by the v1 RPC fetcher, which encoded
// pbstellar.Block.Hash and pbstellar.Block.Header.PreviousLedgerHash
// incorrectly (and therefore the bstream Block.Id / ParentId derived
// from them). Every other field on the block is preserved — only the
// four hash fields are replaced using the bytes captive-core returns
// for the same ledger sequence.
package fix

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/dstore"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-core/cmd/tools/check"
	"github.com/streamingfast/firehose-core/types"
	"github.com/streamingfast/firehose-stellar/captivecore"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
)

// NewToolsFixBlockHashesCmd builds the cobra command. The flag set
// mirrors `firestellar fetch captive-core` so an operator already
// running v2 can re-use the same configuration to drive the rewrite.
func NewToolsFixBlockHashesCmd(logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix-block-hashes <src_merged_blocks_store> <dest_merged_blocks_store> <block_range>",
		Short: "Re-derive ledger Hash and PreviousLedgerHash on v1-fetcher merged blocks using captive-core, keeping everything else intact.",
		Long: `The legacy RPC fetcher wrote bad bytes for the stellar Block.Hash and
Block.Header.PreviousLedgerHash fields (and therefore the bstream
Block.Id / ParentId derived from them). This command boots a local
captive-core subprocess, walks the source merged-blocks store, replaces
the four hash fields with the captive-core values, and writes a fresh
merged-blocks file per 100-block bundle to the destination store. All
other fields (transactions, events, header coin counts, application
order, etc.) are preserved as-is from the source block.

The block range must start at a 100-block boundary so the destination
bundle layout stays aligned with the rest of the store.`,
		Args: cobra.ExactArgs(3),
		RunE: runFixBlockHashesE(logger),
	}

	cmd.Flags().String("stellar-core-bin", "/usr/bin/stellar-core", "path to stellar-core binary")
	cmd.Flags().String("stellar-core-conf", "", "path to stellar-core config file (empty = use bundled SDF default for the network; required for custom)")
	cmd.Flags().String("stellar-core-network", "mainnet", "stellar network (mainnet, testnet, or custom)")
	cmd.Flags().String("stellar-core-network-passphrase", "", "override network passphrase (required for custom; overrides --stellar-core-network when set)")
	cmd.Flags().StringSlice("stellar-core-history-archive-urls", nil, "override history archive URLs (required for custom; overrides --stellar-core-network when set)")
	cmd.Flags().String("stellar-core-log-level", "info", "log level for stellar-core subprocess (debug, info, warn, error)")
	cmd.Flags().String("stellar-core-storage-path", "", "captive-core working dir; empty = a fresh temp dir is created at start")

	return cmd
}

func runFixBlockHashesE(logger *zap.Logger) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		srcStore, err := dstore.NewDBinStore(args[0])
		if err != nil {
			return fmt.Errorf("creating source merged-blocks store: %w", err)
		}

		destStore, err := dstore.NewDBinStore(args[1])
		if err != nil {
			return fmt.Errorf("creating destination merged-blocks store: %w", err)
		}

		blockRange, err := types.GetBlockRangeFromArg(args[2])
		if err != nil {
			return fmt.Errorf("parsing block range: %w", err)
		}
		if !blockRange.IsResolved() {
			return fmt.Errorf("block range must be closed (got %s)", blockRange.String())
		}
		if blockRange.Start%100 != 0 {
			return fmt.Errorf("block range start %d is not aligned to a 100-block bundle boundary", blockRange.Start)
		}

		ccCfg, err := captiveCoreConfigFromFlags(cmd, logger)
		if err != nil {
			return err
		}

		backend, err := captivecore.New(ccCfg)
		if err != nil {
			return fmt.Errorf("starting captive-core backend: %w", err)
		}
		defer backend.Close()

		startBlock := uint64(blockRange.Start)
		stopBlock := blockRange.MustGetStopBlock()

		logger.Info("preparing captive-core range",
			zap.Uint64("start_block", startBlock),
			zap.Uint64("stop_block", stopBlock),
		)
		if err := backend.PrepareRange(ctx, startBlock); err != nil {
			return fmt.Errorf("captive-core prepare range: %w", err)
		}

		mergeWriter := &firecore.MergedBlocksWriter{
			Store:      destStore,
			TweakBlock: func(b *pbbstream.Block) (*pbbstream.Block, error) { return b, nil },
			Logger:     logger,
		}

		walkErr := srcStore.Walk(ctx, check.WalkBlockPrefix(blockRange, 100), func(filename string) error {
			fileStart := firecore.MustParseUint64(filename)
			if fileStart > stopBlock {
				return dstore.StopIteration
			}
			if fileStart+100 <= startBlock {
				return nil
			}

			logger.Info("processing merged-blocks file", zap.String("filename", filename))

			rc, err := srcStore.OpenObject(ctx, filename)
			if err != nil {
				return fmt.Errorf("opening %s: %w", filename, err)
			}
			defer rc.Close()

			br, err := bstream.NewDBinBlockReader(rc)
			if err != nil {
				return fmt.Errorf("creating block reader for %s: %w", filename, err)
			}

			for {
				blk, err := br.Read()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					return fmt.Errorf("reading block from %s: %w", filename, err)
				}

				if blk.Number < startBlock {
					continue
				}
				if blk.Number > stopBlock {
					break
				}

				fixed, err := fixBlockHashes(ctx, backend, blk, logger)
				if err != nil {
					return fmt.Errorf("fixing block %d (%s): %w", blk.Number, blk.Id, err)
				}

				if err := mergeWriter.ProcessBlock(fixed, nil); err != nil {
					if errors.Is(err, io.EOF) {
						return dstore.StopIteration
					}
					return fmt.Errorf("write fixed block %d: %w", fixed.Number, err)
				}
			}

			return nil
		})

		if walkErr != nil && !errors.Is(walkErr, io.EOF) {
			return walkErr
		}

		return nil
	}
}

// fixBlockHashes pulls the same ledger from captive-core, swaps the two
// broken fields on the stellar payload, and updates the bstream Id /
// ParentId to the hex encoding of those bytes. Everything else on the
// v1 block (transactions, events, header coin counts, version, etc.)
// is kept verbatim.
func fixBlockHashes(ctx context.Context, backend *captivecore.Backend, srcBlk *pbbstream.Block, logger *zap.Logger) (*pbbstream.Block, error) {
	ccBlk, err := backend.GetBlock(ctx, srcBlk.Number)
	if err != nil {
		return nil, fmt.Errorf("captive-core get block: %w", err)
	}
	if ccBlk.Number != srcBlk.Number {
		return nil, fmt.Errorf("captive-core block mismatch: requested %d, got %d", srcBlk.Number, ccBlk.Number)
	}

	var srcStellar pbstellar.Block
	if err := srcBlk.Payload.UnmarshalTo(&srcStellar); err != nil {
		return nil, fmt.Errorf("unmarshal src payload: %w", err)
	}
	if srcStellar.Header == nil {
		return nil, fmt.Errorf("src block %d has nil header", srcBlk.Number)
	}

	var ccStellar pbstellar.Block
	if err := ccBlk.Payload.UnmarshalTo(&ccStellar); err != nil {
		return nil, fmt.Errorf("unmarshal captive-core payload: %w", err)
	}
	if ccStellar.Header == nil {
		return nil, fmt.Errorf("captive-core block %d has nil header", ccBlk.Number)
	}

	srcStellar.Hash = ccStellar.Hash
	srcStellar.Header.PreviousLedgerHash = ccStellar.Header.PreviousLedgerHash

	newPayload, err := anypb.New(&srcStellar)
	if err != nil {
		return nil, fmt.Errorf("repacking payload: %w", err)
	}

	fixed := &pbbstream.Block{
		Number:    srcBlk.Number,
		Id:        hex.EncodeToString(srcStellar.Hash),
		ParentId:  hex.EncodeToString(srcStellar.Header.PreviousLedgerHash),
		Timestamp: srcBlk.Timestamp,
		LibNum:    srcBlk.LibNum,
		ParentNum: srcBlk.ParentNum,
		Payload:   newPayload,
	}

	logger.Debug("fixed block hashes",
		zap.Uint64("num", fixed.Number),
		zap.String("old_id", srcBlk.Id),
		zap.String("new_id", fixed.Id),
		zap.String("old_parent_id", srcBlk.ParentId),
		zap.String("new_parent_id", fixed.ParentId),
	)

	return fixed, nil
}

func captiveCoreConfigFromFlags(cmd *cobra.Command, logger *zap.Logger) (captivecore.Config, error) {
	logLevel, err := parseLogrusLevel(sflags.MustGetString(cmd, "stellar-core-log-level"))
	if err != nil {
		return captivecore.Config{}, err
	}

	cfg := captivecore.Config{
		BinaryPath:          sflags.MustGetString(cmd, "stellar-core-bin"),
		StellarCoreConfPath: sflags.MustGetString(cmd, "stellar-core-conf"),
		StoragePath:         sflags.MustGetString(cmd, "stellar-core-storage-path"),
		LogLevel:            logLevel,
		Logger:              logger,
	}
	if err := cfg.ResolveNetwork(sflags.MustGetString(cmd, "stellar-core-network")); err != nil {
		return captivecore.Config{}, err
	}
	if pass := sflags.MustGetString(cmd, "stellar-core-network-passphrase"); pass != "" {
		cfg.NetworkPassphrase = pass
	}
	if urls := sflags.MustGetStringSlice(cmd, "stellar-core-history-archive-urls"); len(urls) > 0 {
		cfg.HistoryArchiveURLs = urls
	}
	if cfg.StellarCoreConfPath == "" && cfg.DefaultTomlData == nil {
		return captivecore.Config{}, fmt.Errorf("--stellar-core-conf is required for custom network (no bundled default)")
	}
	return cfg, nil
}

func parseLogrusLevel(s string) (logrus.Level, error) {
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
