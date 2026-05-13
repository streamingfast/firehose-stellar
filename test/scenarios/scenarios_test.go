// Package scenarios is the battlefield-stellar test suite. Each test submits
// one or more transactions to a Stellar network, fetches the resulting block
// via firehose, and asserts the structural transaction view against a
// recorded snapshot.
//
// Run from the repo root with:
//
//	go test ./test/scenarios/... -v
//
// Or to regenerate all snapshots:
//
//	SNAPSHOTS_UPDATE=. go test ./test/scenarios/... -v
package scenarios

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stellar/go-stellar-sdk/txnbuild"

	"github.com/streamingfast/firehose-stellar/test/lib/runner"
	"github.com/streamingfast/firehose-stellar/test/lib/stellar"
)

// snapshotRoot points the runner at the repo-root snapshots/ directory rather
// than scenarios/snapshots, matching the layout in the design doc.
func snapshotRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(filepath.Dir(cwd), "snapshots")
}

func newRunner(t *testing.T) *runner.Runner {
	t.Helper()
	cfg := runner.DefaultConfig()
	cfg.SnapshotRoot = snapshotRoot(t)
	// Seed keypair derivation off the test name so accounts are
	// byte-identical across two runs of the same test (record from
	// poller → validate from captive-core). Without this, every run
	// gets fresh random keypairs and SAC contract IDs vary, defeating
	// snapshot byte-stability.
	cfg.AccountSeedScope = t.Name()

	// Use the in-process fetchers TestMain built. Shared across the
	// suite; TestMain closes them after m.Run(). Per-test Close would
	// tear down captive-core's stellar-core subprocess.
	cfg.Fetchers = sharedFetchers

	r, err := runner.New(cfg)
	if err != nil {
		t.Fatalf("runner setup: %v", err)
	}
	// Do NOT call r.Close in cleanup — fetchers are shared across tests
	// and owned by TestMain. r.Close would propagate to the shared
	// captive-core fetcher and kill its stellar-core subprocess.
	return r
}

func TestNativePayment(t *testing.T) {
	r := newRunner(t)
	src := r.MustNewFundedAccount("source")
	dst := r.MustNewFundedAccount("dest")

	if err := r.RunScenario("payment/native", []txnbuild.Operation{
		stellar.Payment(dst.Address(), "10", txnbuild.NativeAsset{}),
	}, src); err != nil {
		t.Fatal(err)
	}
}

func TestDoubleNativePayment(t *testing.T) {
	r := newRunner(t)
	src := r.MustNewFundedAccount("source")
	dst := r.MustNewFundedAccount("dest")

	if err := r.RunScenario("payment/double", []txnbuild.Operation{
		stellar.Payment(dst.Address(), "5", txnbuild.NativeAsset{}),
		stellar.Payment(dst.Address(), "7", txnbuild.NativeAsset{}),
	}, src); err != nil {
		t.Fatal(err)
	}
}

func TestCreateAccount(t *testing.T) {
	r := newRunner(t)
	funder := r.MustNewFundedAccount("funder")
	// `target` must NOT be friendbot-funded — the CreateAccount operation
	// is what brings it into existence. Friendbot-funding first would
	// make the op fail with op_already_exists.
	target := r.MustNewUnfundedAccount("target")

	if err := r.RunScenario("account/create", []txnbuild.Operation{
		stellar.CreateAccount(target.Address(), "5"),
	}, funder); err != nil {
		t.Fatal(err)
	}
}

func TestManageData(t *testing.T) {
	r := newRunner(t)
	src := r.MustNewFundedAccount("source")

	if err := r.RunScenario("data/manage", []txnbuild.Operation{
		stellar.ManageData("battlefield", "hello"),
	}, src); err != nil {
		t.Fatal(err)
	}
}

func TestIssueAndSendAsset(t *testing.T) {
	r := newRunner(t)
	issuer := r.MustNewFundedAccount("issuer")
	distributor := r.MustNewFundedAccount("distributor")
	asset := stellar.CreditAsset("USDB", issuer)

	// Distributor accepts the trustline first.
	if err := r.RunScenario("asset/trustline", []txnbuild.Operation{
		stellar.ChangeTrust(asset, "5000"),
	}, distributor); err != nil {
		t.Fatal(err)
	}

	// Then the issuer mints into the distributor account.
	if err := r.RunScenario("asset/issue", []txnbuild.Operation{
		stellar.Payment(distributor.Address(), "100", asset),
	}, issuer); err != nil {
		t.Fatal(err)
	}
}

func TestMultiOp(t *testing.T) {
	r := newRunner(t)
	src := r.MustNewFundedAccount("source")
	a := r.MustNewFundedAccount("recipientA")
	b := r.MustNewFundedAccount("recipientB")

	if err := r.RunScenario("multi_op/payment_pair", []txnbuild.Operation{
		stellar.Payment(a.Address(), "1", txnbuild.NativeAsset{}),
		stellar.Payment(b.Address(), "2", txnbuild.NativeAsset{}),
		stellar.ManageData("note", "multi-op-test"),
	}, src); err != nil {
		t.Fatal(err)
	}
}

// TestFailedTransaction issues a payment from an account that does not exist
// on chain — the submission should be rejected. We assert the rejection
// rather than snapshotting a (non-existent) firehose block.
func TestFailedTransaction(t *testing.T) {
	r := newRunner(t)
	src := r.MustNewFundedAccount("source")
	// Generate a destination keypair but skip funding so the payment fails
	// at create-account-required (the destination has no trustline / no XLM
	// reserve). For native payments this still succeeds and creates the
	// account, so we instead try a custom asset payment with no trustline.
	stranger := r.MustNewFundedAccount("stranger")
	asset := stellar.CreditAsset("NOPE", src)

	err := r.Stellar.SubmitOpsExpectFail(src, []txnbuild.Operation{
		stellar.Payment(stranger.Address(), "1", asset),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAccountMerge(t *testing.T) {
	r := newRunner(t)
	dying := r.MustNewFundedAccount("dying")
	heir := r.MustNewFundedAccount("heir")

	if err := r.RunScenario("account/merge", []txnbuild.Operation{
		stellar.AccountMerge(heir.Address()),
	}, dying); err != nil {
		t.Fatal(err)
	}
}
