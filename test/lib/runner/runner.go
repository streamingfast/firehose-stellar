// Package runner submits a transaction, pulls the resulting block from
// every configured firehose Fetcher, asserts they agree, and compares the
// canonical view against a JSON snapshot.
package runner

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/txnbuild"

	"github.com/streamingfast/firehose-stellar/test/lib/firehose"
	"github.com/streamingfast/firehose-stellar/test/lib/snapshot"
	"github.com/streamingfast/firehose-stellar/test/lib/stellar"
	libxdr "github.com/streamingfast/firehose-stellar/test/lib/xdr"
)

type Config struct {
	// Fetchers is the list of firehose backends the runner reads blocks
	// from. Required; TestMain typically constructs poller +
	// captive-core in-process fetchers and assigns them here.
	Fetchers []firehose.Fetcher

	SnapshotRoot string        // base dir for snapshots/
	WaitTimeout  time.Duration // poll deadline per ledger
	WaitInterval time.Duration

	// AccountSeedScope, when non-empty, makes keypair generation
	// deterministic: NewFundedAccount("foo") derives from
	// sha256(scope + "/" + name). Tests set this to t.Name() so account
	// addresses are byte-stable across runs (snapshots commit literal
	// G... strings).
	AccountSeedScope string
}

// DefaultConfig fills snapshot + timeout fields from env vars. Fetchers
// is left to the caller — set it before calling New.
func DefaultConfig() Config {
	return Config{
		SnapshotRoot: envOr("BATTLEFIELD_SNAPSHOTS", "snapshots"),
		WaitTimeout:  envDuration("BATTLEFIELD_WAIT_TIMEOUT", 90*time.Second),
		WaitInterval: envDuration("BATTLEFIELD_WAIT_INTERVAL", 2*time.Second),
	}
}

type Runner struct {
	Config   Config
	Stellar  *stellar.Client
	Fetchers []firehose.Fetcher
}

func New(cfg Config) (*Runner, error) {
	if len(cfg.Fetchers) == 0 {
		return nil, fmt.Errorf("runner: Config.Fetchers is required")
	}
	sc, err := stellar.NewClient()
	if err != nil {
		return nil, err
	}
	return &Runner{
		Config:   cfg,
		Stellar:  sc,
		Fetchers: cfg.Fetchers,
	}, nil
}

// NewFundedAccount creates a friendbot-funded account.
//
// If Config.AccountSeedScope is set, the keypair is derived
// deterministically from sha256(scope + "/" + name) — see Config docs.
// Otherwise it's freshly random. The deterministic case is the supported
// one for snapshot tests: addresses appear as literal G... strings in
// committed snapshots and remain byte-stable across runs.
func (r *Runner) NewFundedAccount(name string) (*keypair.Full, error) {
	kp, err := r.deriveKeypair(name)
	if err != nil {
		return nil, err
	}
	if err := r.Stellar.FundAccount(kp.Address()); err != nil {
		return nil, fmt.Errorf("fund %s: %w", kp.Address(), err)
	}
	return kp, nil
}

func (r *Runner) MustNewFundedAccount(name string) *keypair.Full {
	kp, err := r.NewFundedAccount(name)
	if err != nil {
		panic(fmt.Errorf("fund %q: %w", name, err))
	}
	return kp
}

// NewUnfundedAccount returns a keypair derived from `name` without
// calling friendbot. Same determinism rules as NewFundedAccount.
//
// Use this for the target of a CreateAccount operation — friendbot-
// funding it first would make CreateAccount fail with op_already_exists.
func (r *Runner) NewUnfundedAccount(name string) (*keypair.Full, error) {
	kp, err := r.deriveKeypair(name)
	if err != nil {
		return nil, err
	}
	return kp, nil
}

func (r *Runner) MustNewUnfundedAccount(name string) *keypair.Full {
	kp, err := r.NewUnfundedAccount(name)
	if err != nil {
		panic(err)
	}
	return kp
}

// deriveKeypair produces a keypair for the logical role `name`. If a
// seed scope is configured, the seed is sha256(scope + "/" + name) so
// keypairs are identical across runs for the same (scope, name) — this
// makes SAC contract IDs (which are hashes of issuer pubkeys) byte-
// stable across the record-then-validate workflow.
func (r *Runner) deriveKeypair(name string) (*keypair.Full, error) {
	if r.Config.AccountSeedScope == "" {
		kp, err := keypair.Random()
		if err != nil {
			return nil, fmt.Errorf("generate keypair %q: %w", name, err)
		}
		return kp, nil
	}
	seed := sha256.Sum256([]byte(r.Config.AccountSeedScope + "/" + name))
	kp, err := keypair.FromRawSeed(seed)
	if err != nil {
		return nil, fmt.Errorf("derive keypair %q from scope %q: %w", name, r.Config.AccountSeedScope, err)
	}
	return kp, nil
}

// RunScenario submits the operations, waits for the resulting ledger to land
// in every configured fetcher, decodes the transaction from each, asserts
// they all agree, and compares the canonical view against the snapshot at
// <SnapshotRoot>/<id>.expected.json.
func (r *Runner) RunScenario(id string, ops []txnbuild.Operation, source *keypair.Full, extraSigners ...*keypair.Full) error {
	resp, err := r.Stellar.SubmitOps(source, ops, extraSigners...)
	if err != nil {
		return fmt.Errorf("submit %s: %w", id, err)
	}
	return r.AssertScenario(id, uint64(resp.Ledger), resp.Hash)
}

// AssertScenario runs the post-submission half of RunScenario against a tx
// hash that was produced by something other than `SubmitOps` — e.g. a fee
// bump submission, a multi-sig flow, or a horizon-bypassing direct submit.
func (r *Runner) AssertScenario(id string, ledger uint64, txHash string) error {
	views, canonical, err := r.fetchAndDecodeFromAll(ledger, txHash)
	if err != nil {
		return err
	}
	if err := r.assertFetchersAgree(id, views); err != nil {
		return err
	}
	return r.compareAgainstSnapshot(id, canonical, ledger)
}

// fetchAndDecodeFromAll fetches the ledger from every fetcher, finds the
// transaction by hash in each block, decodes it structurally, and returns
// (per-fetcher view, canonical view, error). The canonical view is taken
// from the first fetcher in the configured order.
func (r *Runner) fetchAndDecodeFromAll(ledger uint64, txHash string) (map[string]*libxdr.TxView, *libxdr.TxView, error) {
	views := map[string]*libxdr.TxView{}
	var canonical *libxdr.TxView

	for _, f := range r.Fetchers {
		ctx, cancel := context.WithTimeout(context.Background(), r.Config.WaitTimeout)
		block, err := firehose.WaitForBlock(ctx, f, ledger, r.Config.WaitInterval)
		cancel()
		if err != nil {
			return nil, nil, fmt.Errorf("fetcher %s wait for ledger %d: %w", f.Name(), ledger, err)
		}

		tx, _, err := firehose.FindTxByHash(block, txHash)
		if err != nil {
			return nil, nil, fmt.Errorf("fetcher %s: %w", f.Name(), err)
		}

		view, err := libxdr.FromTransaction(tx)
		if err != nil {
			return nil, nil, fmt.Errorf("fetcher %s decode: %w", f.Name(), err)
		}
		views[f.Name()] = view
		if canonical == nil {
			canonical = view
		}
	}
	return views, canonical, nil
}

// assertFetchersAgree compares the structural view emitted by every fetcher
// against the canonical (first) view. Any difference is a fetcher bug and
// fails the scenario regardless of snapshot status.
func (r *Runner) assertFetchersAgree(id string, views map[string]*libxdr.TxView) error {
	if len(views) <= 1 {
		return nil
	}

	canonicalName := r.Fetchers[0].Name()
	canonical := views[canonicalName]
	for _, f := range r.Fetchers[1:] {
		other := views[f.Name()]
		if err := snapshot.DiffViews(canonical, other); err != nil {
			return fmt.Errorf("fetcher disagreement for %s: %s vs %s:\n%w",
				id, canonicalName, f.Name(), err)
		}
	}
	return nil
}

func (r *Runner) compareAgainstSnapshot(id string, view *libxdr.TxView, ledger uint64) error {
	snap, err := snapshot.Load(filepath.Join(r.Config.SnapshotRoot, id+".expected.json"))
	if err != nil {
		return err
	}
	// Account addresses are deterministic across runs (derived from
	// sha256(t.Name()+"/"+name) when AccountSeedScope is set), so we let
	// them appear as literal G... strings in snapshots instead of
	// templating them. This makes the snapshot diff stronger: any drift
	// in an emitted address fails the byte-equality check directly.
	//
	// Renaming a Test… function changes the scope and therefore every
	// derived address, so the affected snapshot must be regenerated:
	//   SNAPSHOTS_UPDATE=^<scenario/id>$ scripts/run_tests.sh
	snap.Bind("ledger", strconv.FormatUint(ledger, 10))
	snap.Bind("hash", view.Hash)
	snap.Bind("createdAt", view.CreatedAt)
	return snap.Compare(view)
}

// Close releases all fetcher resources.
func (r *Runner) Close() {
	for _, f := range r.Fetchers {
		_ = f.Close()
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
