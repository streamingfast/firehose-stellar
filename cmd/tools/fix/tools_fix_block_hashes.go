// Package fix carries one-shot maintenance commands. fix-block-hashes
// rewrites merged-blocks produced by the v1 RPC fetcher, which stored
// 48 bytes of garbage in pbstellar.Block.Hash and
// pbstellar.Block.Header.PreviousLedgerHash because it decoded the
// stellar-rpc hex hash as base64. The original 32-byte hash is
// recoverable per-block by re-encoding the garbage bytes back to
// base64 (yielding the original 64-char hex string) and hex-decoding
// that — no external source needed. See ConvertBrokenHash below.
//
// pbbstream.Block.Id and ParentId on v1 blocks are accidentally the
// correct hex hash strings already (the v1 convertBlock did
// base64.Encode(base64.Decode(hex)) = hex, an identity round-trip on
// 64-char base64 alphabet input), so they are kept and the fix
// cross-checks against them.
package fix

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/dstore"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-core/cmd/tools/check"
	"github.com/streamingfast/firehose-core/types"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
)

const stellarHashLen = 32

// NewToolsFixBlockHashesCmd builds the cobra command.
func NewToolsFixBlockHashesCmd(logger *zap.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "fix-block-hashes <src_merged_blocks_store> <dest_merged_blocks_store> <block_range>",
		Short: "Recover ledger Hash and PreviousLedgerHash on v1-fetcher merged blocks by reversing the bad base64-decode-of-hex encoding.",
		Long: `The legacy RPC fetcher decoded stellar-rpc's hex ledger hash as base64,
which silently succeeded (hex chars are a subset of the base64 alphabet)
and stored 48 bytes of garbage in pbstellar.Block.Hash and
pbstellar.Block.Header.PreviousLedgerHash. The original 32-byte hash is
recoverable per-block by base64-encoding the garbage bytes (which
returns the original 64-char hex string thanks to base64
round-tripping) and then hex-decoding the result. No external data
source is required.

The bstream Block.Id / ParentId were accidentally correct on v1 blocks
(same round-trip identity), so this command uses them as a
cross-check: the recovered hash must equal hex.Decode(src.Id), else
the command bails on the block.

The block range must start at a 100-block boundary so the destination
bundle layout stays aligned with the rest of the store. Non-overlapping
ranges can run in parallel — there is no shared state.`,
		Args: cobra.ExactArgs(3),
		RunE: runFixBlockHashesE(logger),
	}
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

		startBlock := uint64(blockRange.Start)
		stopBlock := blockRange.MustGetStopBlock()

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

				fixed, err := FixBlock(blk)
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

// FixBlock applies the byte-level recovery to one v1 block. Exported so
// the test suite and ad-hoc tooling can reuse it.
func FixBlock(src *pbbstream.Block) (*pbbstream.Block, error) {
	var srcStellar pbstellar.Block
	if err := src.Payload.UnmarshalTo(&srcStellar); err != nil {
		return nil, fmt.Errorf("unmarshal src payload: %w", err)
	}
	if srcStellar.Header == nil {
		return nil, fmt.Errorf("src block %d has nil header", src.Number)
	}

	recoveredHash, err := ConvertBrokenHash(srcStellar.Hash)
	if err != nil {
		return nil, fmt.Errorf("recovering ledger hash: %w", err)
	}
	recoveredPrev, err := ConvertBrokenHash(srcStellar.Header.PreviousLedgerHash)
	if err != nil {
		return nil, fmt.Errorf("recovering previous-ledger hash: %w", err)
	}

	// Cross-check: v1 bstream.Id/ParentId are the correct hex strings
	// (round-trip identity). If they disagree with our recovery, something
	// about the input is not the shape we expect — fail loud.
	if got := hex.EncodeToString(recoveredHash); got != src.Id {
		return nil, fmt.Errorf("recovered hash %s does not match bstream Id %s for block %d", got, src.Id, src.Number)
	}
	if got := hex.EncodeToString(recoveredPrev); got != src.ParentId {
		return nil, fmt.Errorf("recovered previous hash %s does not match bstream ParentId %s for block %d", got, src.ParentId, src.Number)
	}

	srcStellar.Hash = recoveredHash
	srcStellar.Header.PreviousLedgerHash = recoveredPrev

	newPayload, err := anypb.New(&srcStellar)
	if err != nil {
		return nil, fmt.Errorf("repacking payload: %w", err)
	}

	return &pbbstream.Block{
		Number:    src.Number,
		Id:        src.Id,
		ParentId:  src.ParentId,
		Timestamp: src.Timestamp,
		LibNum:    src.LibNum,
		ParentNum: src.ParentNum,
		Payload:   newPayload,
	}, nil
}

// ConvertBrokenHash reverses the v1 fetcher's accidental
// base64.Decode(hex_string) by base64-encoding the stored garbage
// bytes (giving back the original hex string) and hex-decoding the
// result. Returns an error if the recovered hash is not 32 bytes,
// which is the only valid length for a Stellar ledger hash.
func ConvertBrokenHash(broken []byte) ([]byte, error) {
	hexStr := base64.StdEncoding.EncodeToString(broken)
	recovered, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("hex-decoding recovered string %q: %w", hexStr, err)
	}
	if len(recovered) != stellarHashLen {
		return nil, fmt.Errorf("recovered hash length %d, expected %d", len(recovered), stellarHashLen)
	}
	return recovered, nil
}
