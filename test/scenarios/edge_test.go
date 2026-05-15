// Edge-case scenarios that exercise transaction shapes most likely to
// surface differences between the poller (RPC-based) and captive-core
// (stellar-core-direct) firehose backends:
//
//   - V1 envelope with multi-sig (multiple decorated signatures)
//   - Fee-bump envelope (FeeBumpTransactionEnvelope wraps an inner tx)
//   - SetOptions adding a signer (changes account thresholds)
//   - ManageSellOffer (DEX op, exercises offer events)
//   - BumpSequence (boring op, useful as a sanity benchmark)
//
// Soroban-specific scenarios live in soroban_test.go (TODO).
package scenarios

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/streamingfast/firehose-stellar/test/lib/stellar"
)

func TestFeeBumpPayment(t *testing.T) {
	r := newRunner(t)
	payer := r.MustNewFundedAccount("payer")
	innerSrc := r.MustNewFundedAccount("inner_source")
	dest := r.MustNewFundedAccount("dest")

	resp, err := r.Stellar.SubmitFeeBump(payer, innerSrc, []txnbuild.Operation{
		stellar.Payment(dest.Address(), "3", txnbuild.NativeAsset{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.AssertScenario("fee_bump/payment", uint64(resp.Ledger), resp.Hash); err != nil {
		t.Fatal(err)
	}
}

func TestMultiSigPayment(t *testing.T) {
	r := newRunner(t)
	src := r.MustNewFundedAccount("source")
	cosigner := r.MustNewFundedAccount("cosigner")
	dest := r.MustNewFundedAccount("dest")

	// Add cosigner as a 1-weight signer; lower master to 1 and bump
	// thresholds to 2 so that BOTH master and cosigner are required.
	masterWeight := txnbuild.Threshold(1)
	med := txnbuild.Threshold(2)
	high := txnbuild.Threshold(2)
	if err := r.RunScenario("multisig/setup", []txnbuild.Operation{
		&txnbuild.SetOptions{
			MasterWeight:    &masterWeight,
			MediumThreshold: &med,
			HighThreshold:   &high,
			Signer: &txnbuild.Signer{
				Address: cosigner.Address(),
				Weight:  txnbuild.Threshold(1),
			},
		},
	}, src); err != nil {
		t.Fatal(err)
	}

	// Now require both signatures to push a payment through.
	if err := r.RunScenario("multisig/payment", []txnbuild.Operation{
		stellar.Payment(dest.Address(), "1", txnbuild.NativeAsset{}),
	}, src, cosigner); err != nil {
		t.Fatal(err)
	}
}

func TestManageSellOffer(t *testing.T) {
	r := newRunner(t)
	issuer := r.MustNewFundedAccount("issuer")
	seller := r.MustNewFundedAccount("seller")
	asset := stellar.CreditAsset("OFFR", issuer)

	if err := r.RunScenario("offer/setup_trustline", []txnbuild.Operation{
		stellar.ChangeTrust(asset, "1000"),
	}, seller); err != nil {
		t.Fatal(err)
	}
	if err := r.RunScenario("offer/issue_to_seller", []txnbuild.Operation{
		stellar.Payment(seller.Address(), "100", asset),
	}, issuer); err != nil {
		t.Fatal(err)
	}

	// Place a passive offer: 100 OFFR for native, 1 OFFR = 0.5 XLM.
	if err := r.RunScenario("offer/manage_sell", []txnbuild.Operation{
		&txnbuild.ManageSellOffer{
			Selling: asset,
			Buying:  txnbuild.NativeAsset{},
			Amount:  "10",
			Price:   xdr.Price{N: 1, D: 2},
		},
	}, seller); err != nil {
		t.Fatal(err)
	}
}

func TestBumpSequence(t *testing.T) {
	r := newRunner(t)
	src := r.MustNewFundedAccount("source")

	current, err := r.Stellar.LoadAccount(src.Address())
	if err != nil {
		t.Fatal(err)
	}
	seq, err := current.GetSequenceNumber()
	if err != nil {
		t.Fatal(err)
	}

	if err := r.RunScenario("seq/bump", []txnbuild.Operation{
		&txnbuild.BumpSequence{BumpTo: seq + 100},
	}, src); err != nil {
		t.Fatal(err)
	}
}
