// tool-compare-merged-blocks diffs two stellar merged-block stores
// over a block range. Modeled on firehose-core's compare-blocks but
// scoped to merged-block (100-block) bundles and aware of the legacy
// v1 RPC fetcher's broken hash encoding (see cmd/tools/fix).
//
// Either side can be flagged as "broken" via --sanitize-reference /
// --sanitize-current. A sanitized side has its pbstellar.Block.Hash
// and Header.PreviousLedgerHash recovered via fix.ConvertBrokenHash,
// with pbbstream.Block.Id / ParentId recomputed as hex of those bytes
// and the Payload re-marshalled, before the comparison runs.
package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/firehose-core/cmd/tools/check"
	fctypes "github.com/streamingfast/firehose-core/types"
	"github.com/streamingfast/firehose-stellar/cmd/tools/fix"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const mergedBundleSize = uint64(100)

func NewToolCompareMergedBlocksCmd(logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool-compare-merged-blocks <reference_store> <current_store> <block_range>",
		Short: "Compare stellar merged-block bundles across two stores over a block range",
		Long: `Walks 100-block merged-block bundles in both stores over the given
range and reports any blocks that differ. Useful for validating a new
fetcher (e.g. captive-core) against the existing stored output.

Either side can be flagged as broken via --sanitize-reference or
--sanitize-current. A sanitized side has its pbstellar.Block.Hash and
Header.PreviousLedgerHash run through fix.ConvertBrokenHash (the same
recovery used by the fix-block-hashes tool), and its bstream Id /
ParentId regenerated as hex of those recovered bytes, before the
comparison. Use this to compare legacy v1-RPC-fetcher output against
correctly-hashed blocks.

Arguments:
  reference_store  dstore URL (gs://, file://, ...) — left side
  current_store    dstore URL — right side
  block_range      e.g. "100:200", "0:16000000", or single block "60132634"`,
		Args: cobra.ExactArgs(3),
		RunE: runCompareMergedBlocksE(logger),
		Example: `firestellar tool-compare-merged-blocks \
    gs://old-v1-store/stellar-mainnet/v1 \
    gs://captive-core-store/stellar-mainnet/v2 \
    60132600:60132700 \
    --sanitize-reference`,
	}

	cmd.Flags().Bool("sanitize-reference", false, "treat reference store as legacy v1-RPC output: recover Hash / PreviousLedgerHash via fix.ConvertBrokenHash before comparing")
	cmd.Flags().Bool("sanitize-current", false, "treat current store as legacy v1-RPC output: recover Hash / PreviousLedgerHash via fix.ConvertBrokenHash before comparing")
	cmd.Flags().Bool("diff", false, "print JSON diff (protojson) of each differing block")
	cmd.Flags().Bool("stop-on-first-diff", false, "stop walking as soon as the first differing block is found")

	return cmd
}

func runCompareMergedBlocksE(logger *zap.Logger) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		refStore, err := dstore.NewDBinStore(args[0])
		if err != nil {
			return fmt.Errorf("creating reference store: %w", err)
		}
		curStore, err := dstore.NewDBinStore(args[1])
		if err != nil {
			return fmt.Errorf("creating current store: %w", err)
		}

		blockRange, err := fctypes.GetBlockRangeFromArg(args[2])
		if err != nil {
			return fmt.Errorf("parsing block range: %w", err)
		}
		// Allow single-block argument: "60132634" -> [60132634, 60132635)
		if blockRange.IsOpen() && blockRange.Start >= 0 {
			n := uint64(blockRange.Start)
			blockRange = fctypes.NewClosedRange(int64(n), n+1)
		}
		if !blockRange.IsResolved() {
			return fmt.Errorf("block range must be closed (got %s)", blockRange.String())
		}

		sanitizeRef := sflags.MustGetBool(cmd, "sanitize-reference")
		sanitizeCur := sflags.MustGetBool(cmd, "sanitize-current")
		showDiff := sflags.MustGetBool(cmd, "diff")
		stopOnFirstDiff := sflags.MustGetBool(cmd, "stop-on-first-diff")

		stopBlock := blockRange.MustGetStopBlock()
		startBlock := uint64(blockRange.Start)

		fmt.Printf("Comparing merged blocks [%d, %d)\n", startBlock, stopBlock)
		fmt.Printf("  Reference: %s%s\n", args[0], sanitizeNote(sanitizeRef))
		fmt.Printf("  Current:   %s%s\n", args[1], sanitizeNote(sanitizeCur))

		var totalCompared, totalDifferent, totalMissingInCur, totalMissingInRef int
		var stopErr = errors.New("stop-on-first-diff")

		walkErr := refStore.Walk(ctx, check.WalkBlockPrefix(blockRange, mergedBundleSize), func(filename string) error {
			fileStart, err := strconv.ParseUint(filename, 10, 64)
			if err != nil {
				// Non-bundle file (one-block file etc); skip.
				return nil
			}
			if fileStart >= stopBlock {
				return dstore.StopIteration
			}
			// Bundle overlap with requested range.
			if fileStart+mergedBundleSize <= startBlock {
				return nil
			}

			var (
				wg      sync.WaitGroup
				refMap  map[uint64]*pbbstream.Block
				curMap  map[uint64]*pbbstream.Block
				refErr  error
				curErr  error
			)
			wg.Add(2)
			go func() {
				defer wg.Done()
				refMap, refErr = readMergedBundle(ctx, refStore, filename, startBlock, stopBlock, sanitizeRef)
			}()
			go func() {
				defer wg.Done()
				exists, existsErr := curStore.FileExists(ctx, filename)
				if existsErr != nil {
					curErr = fmt.Errorf("checking current bundle %s: %w", filename, existsErr)
					return
				}
				if !exists {
					curMap = map[uint64]*pbbstream.Block{}
					return
				}
				curMap, curErr = readMergedBundle(ctx, curStore, filename, startBlock, stopBlock, sanitizeCur)
			}()
			wg.Wait()
			if refErr != nil {
				return fmt.Errorf("reading reference bundle %s: %w", filename, refErr)
			}
			if curErr != nil {
				return fmt.Errorf("reading current bundle %s: %w", filename, curErr)
			}

			// Compare every reference block to its current counterpart.
			for blockNum := max(startBlock, fileStart); blockNum < min(stopBlock, fileStart+mergedBundleSize); blockNum++ {
				refBlk, refOK := refMap[blockNum]
				curBlk, curOK := curMap[blockNum]

				switch {
				case !refOK && !curOK:
					continue
				case !refOK:
					totalMissingInRef++
					fmt.Printf("- Block %d missing in reference (present in current)\n", blockNum)
				case !curOK:
					totalMissingInCur++
					fmt.Printf("- Block %d missing in current (present in reference)\n", blockNum)
				default:
					totalCompared++
					diffs, refStellar, curStellar := compareSingleBlock(refBlk, curBlk)
					if len(diffs) == 0 {
						continue
					}
					totalDifferent++
					shortRef := refBlk.Id
					if len(shortRef) > 12 {
						shortRef = shortRef[:12] + "..."
					}
					fmt.Printf("- Block %d differs (ref id=%s): %d field(s)\n", blockNum, shortRef, len(diffs))
					for _, d := range diffs {
						fmt.Printf("    · %s\n", d)
					}
					if showDiff {
						printJSONDiff(blockNum, refStellar, curStellar)
					}
					if stopOnFirstDiff {
						return stopErr
					}
				}
			}
			return nil
		})
		if walkErr != nil && !errors.Is(walkErr, stopErr) {
			return fmt.Errorf("walking reference bundles: %w", walkErr)
		}

		fmt.Println()
		fmt.Printf("Summary: %d compared, %d different, %d missing in current, %d missing in reference\n",
			totalCompared, totalDifferent, totalMissingInCur, totalMissingInRef)
		if totalDifferent == 0 && totalMissingInCur == 0 && totalMissingInRef == 0 {
			fmt.Println("✅ Block ranges match.")
		}

		// Silence unused-import warning when logger has no call sites yet.
		_ = logger
		return nil
	}
}

func sanitizeNote(on bool) string {
	if on {
		return "  (sanitize: ConvertBrokenHash)"
	}
	return ""
}

// readMergedBundle reads a 100-block merged file and returns a map
// keyed by block number. Blocks outside [startBlock, stopBlock) are
// dropped. When sanitize is true, broken Hash / PreviousLedgerHash are
// recovered and bstream Id/ParentId rewritten before the block lands
// in the map.
func readMergedBundle(ctx context.Context, store dstore.Store, filename string, startBlock, stopBlock uint64, sanitize bool) (map[uint64]*pbbstream.Block, error) {
	reader, err := store.OpenObject(ctx, filename)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", filename, err)
	}
	defer reader.Close()

	blockReader, err := bstream.NewDBinBlockReader(reader)
	if err != nil {
		return nil, fmt.Errorf("creating block reader: %w", err)
	}

	out := make(map[uint64]*pbbstream.Block)
	for {
		blk, err := blockReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading block: %w", err)
		}
		if blk.Number < startBlock || blk.Number >= stopBlock {
			continue
		}
		if sanitize {
			if err := sanitizeBlockInPlace(blk); err != nil {
				return nil, fmt.Errorf("sanitizing block %d in %s: %w", blk.Number, filename, err)
			}
		}
		out[blk.Number] = blk
	}
	return out, nil
}

// sanitizeBlockInPlace runs fix.ConvertBrokenHash on the stellar
// payload's Hash and PreviousLedgerHash, then rewrites bstream Id /
// ParentId to match and re-marshals the payload.
func sanitizeBlockInPlace(blk *pbbstream.Block) error {
	var stellarBlk pbstellar.Block
	if err := blk.Payload.UnmarshalTo(&stellarBlk); err != nil {
		return fmt.Errorf("unmarshalling payload: %w", err)
	}

	recoveredHash, err := fix.ConvertBrokenHash(stellarBlk.Hash)
	if err != nil {
		return fmt.Errorf("recover Hash: %w", err)
	}
	recoveredPrev, err := fix.ConvertBrokenHash(stellarBlk.Header.PreviousLedgerHash)
	if err != nil {
		return fmt.Errorf("recover PreviousLedgerHash: %w", err)
	}

	stellarBlk.Hash = recoveredHash
	stellarBlk.Header.PreviousLedgerHash = recoveredPrev

	blk.Id = hex.EncodeToString(recoveredHash)
	blk.ParentId = hex.EncodeToString(recoveredPrev)

	newPayload, err := anypb.New(&stellarBlk)
	if err != nil {
		return fmt.Errorf("re-marshalling stellar payload: %w", err)
	}
	blk.Payload = newPayload
	return nil
}

// compareSingleBlock returns a list of differing fields between two
// bstream blocks plus the unmarshalled stellar payloads (for optional
// JSON diff printing). proto.Equal handles deep equality of the
// payload — we additionally surface a few top-level field names so
// the output is actionable.
func compareSingleBlock(ref, cur *pbbstream.Block) ([]string, *pbstellar.Block, *pbstellar.Block) {
	var diffs []string

	if ref.Number != cur.Number {
		diffs = append(diffs, fmt.Sprintf("bstream.Number: %d vs %d", ref.Number, cur.Number))
	}
	if ref.Id != cur.Id {
		diffs = append(diffs, fmt.Sprintf("bstream.Id: %s vs %s", ref.Id, cur.Id))
	}
	if ref.ParentId != cur.ParentId {
		diffs = append(diffs, fmt.Sprintf("bstream.ParentId: %s vs %s", ref.ParentId, cur.ParentId))
	}

	var refStellar, curStellar pbstellar.Block
	if err := ref.Payload.UnmarshalTo(&refStellar); err != nil {
		diffs = append(diffs, fmt.Sprintf("unmarshal reference payload: %s", err))
		return diffs, nil, nil
	}
	if err := cur.Payload.UnmarshalTo(&curStellar); err != nil {
		diffs = append(diffs, fmt.Sprintf("unmarshal current payload: %s", err))
		return diffs, &refStellar, nil
	}

	if !proto.Equal(&refStellar, &curStellar) {
		// Top-level field hints. proto.Equal already told us they
		// differ; these lines just say where.
		if !bytesEq(refStellar.Hash, curStellar.Hash) {
			diffs = append(diffs, fmt.Sprintf("pbstellar.Hash: %x vs %x", refStellar.Hash, curStellar.Hash))
		}
		if refStellar.Header != nil && curStellar.Header != nil {
			if !bytesEq(refStellar.Header.PreviousLedgerHash, curStellar.Header.PreviousLedgerHash) {
				diffs = append(diffs, fmt.Sprintf("pbstellar.Header.PreviousLedgerHash: %x vs %x", refStellar.Header.PreviousLedgerHash, curStellar.Header.PreviousLedgerHash))
			}
			if refStellar.Header.LedgerVersion != curStellar.Header.LedgerVersion {
				diffs = append(diffs, fmt.Sprintf("pbstellar.Header.LedgerVersion: %d vs %d", refStellar.Header.LedgerVersion, curStellar.Header.LedgerVersion))
			}
			if refStellar.Header.TotalCoins != curStellar.Header.TotalCoins {
				diffs = append(diffs, fmt.Sprintf("pbstellar.Header.TotalCoins: %d vs %d", refStellar.Header.TotalCoins, curStellar.Header.TotalCoins))
			}
			if refStellar.Header.BaseFee != curStellar.Header.BaseFee {
				diffs = append(diffs, fmt.Sprintf("pbstellar.Header.BaseFee: %d vs %d", refStellar.Header.BaseFee, curStellar.Header.BaseFee))
			}
			if refStellar.Header.BaseReserve != curStellar.Header.BaseReserve {
				diffs = append(diffs, fmt.Sprintf("pbstellar.Header.BaseReserve: %d vs %d", refStellar.Header.BaseReserve, curStellar.Header.BaseReserve))
			}
		}
		if len(refStellar.Transactions) != len(curStellar.Transactions) {
			diffs = append(diffs, fmt.Sprintf("pbstellar.Transactions count: %d vs %d", len(refStellar.Transactions), len(curStellar.Transactions)))
		} else {
			perTx := compareTransactionSlices(refStellar.Transactions, curStellar.Transactions)
			diffs = append(diffs, perTx...)
		}

		// Generic catch-all for anything we did not name above.
		if len(diffs) == 0 {
			diffs = append(diffs, "payloads differ (no top-level field surfaced; see --diff for full JSON)")
		}
	}

	return diffs, &refStellar, &curStellar
}

// compareTransactionSlices matches transactions by hash and reports
// per-transaction field-level differences. Returns one entry per
// differing field so the user can see exactly what changed.
func compareTransactionSlices(ref, cur []*pbstellar.Transaction) []string {
	var diffs []string
	refByHash := make(map[string]*pbstellar.Transaction, len(ref))
	refIdx := make(map[string]int, len(ref))
	for i, tx := range ref {
		h := fmt.Sprintf("%x", tx.Hash)
		refByHash[h] = tx
		refIdx[h] = i
	}
	curByHash := make(map[string]*pbstellar.Transaction, len(cur))
	for _, tx := range cur {
		curByHash[fmt.Sprintf("%x", tx.Hash)] = tx
	}

	for h, refTx := range refByHash {
		curTx, ok := curByHash[h]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("tx %s (index %d): missing in current", h, refIdx[h]))
			continue
		}
		if !proto.Equal(refTx, curTx) {
			if refTx.Status != curTx.Status {
				diffs = append(diffs, fmt.Sprintf("tx %s (index %d): Status %s vs %s", h, refIdx[h], refTx.Status, curTx.Status))
			}
			if refTx.ApplicationOrder != curTx.ApplicationOrder {
				diffs = append(diffs, fmt.Sprintf("tx %s (index %d): ApplicationOrder %d vs %d", h, refIdx[h], refTx.ApplicationOrder, curTx.ApplicationOrder))
			}
			if !bytesEq(refTx.EnvelopeXdr, curTx.EnvelopeXdr) {
				diffs = append(diffs, fmt.Sprintf("tx %s (index %d): EnvelopeXdr differs", h, refIdx[h]))
			}
			if !bytesEq(refTx.ResultXdr, curTx.ResultXdr) {
				diffs = append(diffs, fmt.Sprintf("tx %s (index %d): ResultXdr differs", h, refIdx[h]))
			}
			if !proto.Equal(refTx.Events, curTx.Events) {
				diffs = append(diffs, fmt.Sprintf("tx %s (index %d): Events differ", h, refIdx[h]))
			}
		}
	}
	for h, curTx := range curByHash {
		if _, ok := refByHash[h]; !ok {
			diffs = append(diffs, fmt.Sprintf("tx %x: missing in reference (present in current)", curTx.Hash))
		}
	}
	return diffs
}

func printJSONDiff(blockNum uint64, ref, cur *pbstellar.Block) {
	if ref == nil || cur == nil {
		return
	}
	marshaller := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: false}
	refJSON, err := marshaller.Marshal(ref)
	if err != nil {
		fmt.Printf("    ! marshalling reference: %s\n", err)
		return
	}
	curJSON, err := marshaller.Marshal(cur)
	if err != nil {
		fmt.Printf("    ! marshalling current: %s\n", err)
		return
	}
	fmt.Printf("    --- reference (block %d) ---\n%s\n", blockNum, string(refJSON))
	fmt.Printf("    --- current (block %d) ---\n%s\n", blockNum, string(curJSON))
}

func bytesEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
