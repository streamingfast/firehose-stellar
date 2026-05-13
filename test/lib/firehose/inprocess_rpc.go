// In-process RPC fetcher.
//
// Wraps github.com/streamingfast/firehose-stellar/rpc directly: no container,
// no firecore reader-node, no dbin files. Tests instantiate an InProcessRPCFetcher
// pointed at the host-side soroban-rpc (the quickstart container) and ask
// for blocks one at a time.
//
// This is the production rpc.Fetcher used by `firestellar fetch rpc`, just
// called from Go instead of from a long-running firecore subprocess.
package firehose

import (
	"context"
	"fmt"
	"sync"
	"time"

	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"github.com/streamingfast/firehose-stellar/rpc"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

// inProcessTracer is the package-level Tracer required by rpc.NewClient.
// The logging package only exposes Tracer construction via PackageLogger,
// so we register one for this package once and pass it through.
var _, inProcessTracer = logging.PackageLogger("test_inprocess", "github.com/streamingfast/firehose-stellar/test/lib/firehose")

// InProcessRPCFetcher implements Fetcher by calling rpc.Fetcher.Fetch in
// the caller's process. Useful for integration tests that want byte-
// equivalent output to the production poller without the docker overhead.
type InProcessRPCFetcher struct {
	name              string
	client            *rpc.Client
	fetcher           *rpc.Fetcher
	logger            *zap.Logger
	networkPassphrase string

	// rpc.Fetcher caches the last-seen ledger and won't drive its
	// internal "wait for chain to catch up" loop concurrently. Serialize
	// FetchBlock calls so concurrent tests are safe.
	mu sync.Mutex
}

// InProcessRPCConfig configures an InProcessRPCFetcher. Defaults are
// tuned for a local quickstart container (fast ticks, no retry pressure).
type InProcessRPCConfig struct {
	// Name is the fetcher identifier reported via Name(). Defaults to "rpc".
	Name string

	// RPCEndpoint is the soroban-rpc URL, e.g.
	// "http://localhost:8000/soroban/rpc". Required.
	RPCEndpoint string

	// NetworkPassphrase must match the chain — Stellar uses it to
	// recompute transaction hashes from ledger metadata. For a local
	// quickstart in --local mode this is the standalone passphrase
	// "Standalone Network ; February 2017".
	NetworkPassphrase string

	// FetchInterval controls the rpc.Fetcher's internal pacing between
	// successive ledger reads. Defaults to 1s.
	FetchInterval time.Duration

	// LatestBlockRetryInterval controls how often the fetcher polls for
	// the chain's latest ledger when waiting for a future block.
	// Defaults to 500ms (quickstart ticks every 5s).
	LatestBlockRetryInterval time.Duration

	// TransactionFetchLimit caps the number of transactions per
	// getTransactions paginated call. Defaults to 200 (rpc default).
	TransactionFetchLimit int

	// Logger receives fetcher events. Optional — defaults to a no-op
	// zap logger when nil.
	Logger *zap.Logger
}

// NewInProcessRPCFetcher constructs a fetcher ready to call FetchBlock.
// Validates required fields, fills defaults, and dials the rpc.Client.
func NewInProcessRPCFetcher(cfg InProcessRPCConfig) (*InProcessRPCFetcher, error) {
	if cfg.RPCEndpoint == "" {
		return nil, fmt.Errorf("inprocess rpc: RPCEndpoint is required")
	}
	if cfg.NetworkPassphrase == "" {
		return nil, fmt.Errorf("inprocess rpc: NetworkPassphrase is required")
	}
	name := cfg.Name
	if name == "" {
		name = "rpc"
	}
	if cfg.FetchInterval == 0 {
		cfg.FetchInterval = 1 * time.Second
	}
	if cfg.LatestBlockRetryInterval == 0 {
		cfg.LatestBlockRetryInterval = 500 * time.Millisecond
	}
	if cfg.TransactionFetchLimit == 0 {
		cfg.TransactionFetchLimit = 200
	}
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	client := rpc.NewClient(cfg.RPCEndpoint, logger, inProcessTracer)
	fetcher := rpc.NewFetcher(
		cfg.FetchInterval,
		cfg.LatestBlockRetryInterval,
		cfg.TransactionFetchLimit,
		cfg.NetworkPassphrase,
		logger,
	)
	return &InProcessRPCFetcher{
		name:              name,
		client:            client,
		fetcher:           fetcher,
		logger:            logger,
		networkPassphrase: cfg.NetworkPassphrase,
	}, nil
}

// Name returns the fetcher identifier.
func (f *InProcessRPCFetcher) Name() string { return f.name }

// FetchBlock pulls one ledger from soroban-rpc. If `skipped` is true (the
// rpc fetcher's signal that the requested ledger doesn't exist on chain)
// it's surfaced as an error so the caller's retry loop kicks in.
func (f *InProcessRPCFetcher) FetchBlock(ctx context.Context, ledger uint64) (*pbstellar.Block, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	bstreamBlock, skipped, err := f.fetcher.Fetch(ctx, f.client, ledger)
	if err != nil {
		return nil, fmt.Errorf("inprocess rpc fetch ledger %d: %w", ledger, err)
	}
	if skipped {
		return nil, fmt.Errorf("inprocess rpc: ledger %d skipped (gap or pre-genesis)", ledger)
	}
	return unmarshalStellarBlock(bstreamBlock)
}

// Close is a no-op for the rpc fetcher — there's no long-lived process
// or connection to release. Provided to satisfy the Fetcher interface.
func (f *InProcessRPCFetcher) Close() error { return nil }
