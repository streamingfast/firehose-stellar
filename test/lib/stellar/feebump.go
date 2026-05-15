package stellar

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/keypair"
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/txnbuild"
)

// SubmitFeeBump wraps an inner-signed transaction in a fee-bump envelope
// signed by `payer`, then submits it. Useful for surfacing fee-bump-specific
// envelope encoding (the firehose poller and captive-core paths historically
// disagreed on FeeBumpTransactionEnvelope decoding).
func (c *Client) SubmitFeeBump(payer *keypair.Full, innerSource *keypair.Full, ops []txnbuild.Operation) (hProtocol.Transaction, error) {
	innerAccount, err := c.LoadAccount(innerSource.Address())
	if err != nil {
		return hProtocol.Transaction{}, fmt.Errorf("load inner account: %w", err)
	}
	inner, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &innerAccount,
		IncrementSequenceNum: true,
		BaseFee:              txnbuild.MinBaseFee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
		Operations:           ops,
	})
	if err != nil {
		return hProtocol.Transaction{}, fmt.Errorf("build inner: %w", err)
	}
	inner, err = inner.Sign(c.NetworkPhrase, innerSource)
	if err != nil {
		return hProtocol.Transaction{}, fmt.Errorf("sign inner: %w", err)
	}

	bump, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      inner,
		FeeAccount: payer.Address(),
		BaseFee:    txnbuild.MinBaseFee * 10,
	})
	if err != nil {
		return hProtocol.Transaction{}, fmt.Errorf("build fee bump: %w", err)
	}
	bump, err = bump.Sign(c.NetworkPhrase, payer)
	if err != nil {
		return hProtocol.Transaction{}, fmt.Errorf("sign fee bump: %w", err)
	}

	resp, err := c.Horizon.SubmitFeeBumpTransaction(bump)
	if err != nil {
		return hProtocol.Transaction{}, fmt.Errorf("submit fee bump: %w", err)
	}
	return resp, nil
}
