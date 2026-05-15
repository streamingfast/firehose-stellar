// Package firehose abstracts a source of decoded pbstellar.Block values.
// Two implementations:
//
//   - InProcessRPCFetcher: wraps the rpc.Fetcher library
//   - InProcessCaptiveCoreFetcher: wraps the captivecore.Backend library
//
// The runner pulls each ledger from every configured fetcher and asserts
// they agree (cross-backend diff) before snapshot comparison.
package firehose

import (
	"context"
	"fmt"
	"strings"
	"time"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
)

// unmarshalStellarBlock unwraps the pbstellar.Block payload from a
// pbbstream.Block. Used by the in-process fetcher impls to convert the
// firestellar library's return type into the shape the runner expects.
func unmarshalStellarBlock(b *pbbstream.Block) (*pbstellar.Block, error) {
	var out pbstellar.Block
	if err := b.Payload.UnmarshalTo(&out); err != nil {
		return nil, fmt.Errorf("unmarshal stellar payload: %w", err)
	}
	return &out, nil
}

// Fetcher is a source of decoded stellar blocks.
type Fetcher interface {
	// Name identifies the fetcher ("poller", "captive-core").
	Name() string
	// FetchBlock returns the block at ledger or an error if not yet available.
	FetchBlock(ctx context.Context, ledger uint64) (*pbstellar.Block, error)
	// Close releases the fetcher's resources.
	Close() error
}

// WaitForBlock polls a fetcher with backoff until either the block becomes
// available or the context is cancelled. Useful right after submitting a tx,
// when the firehose tail is a few seconds behind horizon.
func WaitForBlock(ctx context.Context, f Fetcher, ledger uint64, every time.Duration) (*pbstellar.Block, error) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		blk, err := f.FetchBlock(ctx, ledger)
		if err == nil {
			return blk, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("[%s] waiting for ledger %d: %w", f.Name(), ledger, ctx.Err())
		case <-t.C:
		}
	}
}

// FindTxByHash scans a block for a transaction with the matching hex hash.
func FindTxByHash(block *pbstellar.Block, hexHash string) (*pbstellar.Transaction, int, error) {
	for i, tx := range block.Transactions {
		if fmt.Sprintf("%x", tx.Hash) == strings.ToLower(hexHash) {
			return tx, i, nil
		}
	}
	return nil, -1, fmt.Errorf("transaction %s not found in ledger %d", hexHash, block.Number)
}
