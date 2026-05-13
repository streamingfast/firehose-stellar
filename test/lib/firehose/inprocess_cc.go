// In-process captive-core fetcher.
//
// Wraps github.com/streamingfast/firehose-stellar/captivecore directly:
// supervises a stellar-core subprocess from the test runner, no docker
// container. Requires the stellar-core binary on the host (homebrew on
// macOS, apt on Linux).
//
// Same library used by `firestellar fetch captive-core`. Calling it
// directly skips the firecore reader-node + merger plumbing the
// production CLI needs.
package firehose

import (
	"context"
	"fmt"
	"sync"

	"github.com/streamingfast/firehose-stellar/captivecore"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"go.uber.org/zap"
)

// InProcessCaptiveCoreFetcher implements Fetcher by driving a captivecore.Backend
// in-process. The first FetchBlock call lazily calls PrepareRange with that
// ledger as the start; subsequent calls fetch via the same prepared range.
//
// stellar-core runs as a subprocess of the test runner — defer Close()
// to terminate it cleanly.
type InProcessCaptiveCoreFetcher struct {
	name    string
	backend *captivecore.Backend
	logger  *zap.Logger

	mu             sync.Mutex
	rangePrepared  bool
	preparedLedger uint64
}

// NewInProcessCaptiveCoreFetcher constructs a fetcher. The Config mirrors
// captivecore.Config; this is just a thin wrapper that adds a Name and
// the lazy PrepareRange behavior battlefield needs.
func NewInProcessCaptiveCoreFetcher(name string, cfg captivecore.Config) (*InProcessCaptiveCoreFetcher, error) {
	if name == "" {
		name = "captive-core"
	}
	backend, err := captivecore.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("inprocess captive-core: %w", err)
	}
	return &InProcessCaptiveCoreFetcher{
		name:    name,
		backend: backend,
		logger:  cfg.Logger,
	}, nil
}

// Name returns the fetcher identifier.
func (f *InProcessCaptiveCoreFetcher) Name() string { return f.name }

// FetchBlock returns the stellar block at ledger. On first call,
// PrepareRange is invoked with this ledger as the start — meaning the
// stellar-core subprocess catches up from scratch. Subsequent calls
// hit the prepared range directly.
//
// If the caller asks for a ledger BEFORE the prepared range, returns an
// error — captive-core can't rewind without restarting stellar-core.
// Most battlefield scenarios submit a tx and immediately fetch its
// ledger, so the natural call pattern is monotonically increasing
// ledger numbers, which works fine.
func (f *InProcessCaptiveCoreFetcher) FetchBlock(ctx context.Context, ledger uint64) (*pbstellar.Block, error) {
	f.mu.Lock()
	if !f.rangePrepared {
		// First call dictates the range start. PrepareRange spawns
		// stellar-core and waits for it to catch up to `ledger` — can
		// take 5-30s depending on archive replay distance.
		if err := f.backend.PrepareRange(ctx, ledger); err != nil {
			f.mu.Unlock()
			return nil, fmt.Errorf("inprocess captive-core prepare range from %d: %w", ledger, err)
		}
		f.rangePrepared = true
		f.preparedLedger = ledger
	} else if ledger < f.preparedLedger {
		f.mu.Unlock()
		return nil, fmt.Errorf("inprocess captive-core: ledger %d is below prepared start %d (captive-core can't rewind)",
			ledger, f.preparedLedger)
	}
	f.mu.Unlock()

	bstreamBlock, err := f.backend.GetBlock(ctx, ledger)
	if err != nil {
		return nil, fmt.Errorf("inprocess captive-core fetch ledger %d: %w", ledger, err)
	}
	return unmarshalStellarBlock(bstreamBlock)
}

// Close terminates the stellar-core subprocess. Idempotent.
func (f *InProcessCaptiveCoreFetcher) Close() error {
	if f.backend == nil {
		return nil
	}
	err := f.backend.Close()
	f.backend = nil
	return err
}
