// Package stellar wraps the stellar/go-stellar-sdk to provide the high-level
// operations battlefield scenarios need: account creation/funding,
// transaction building/signing/submission, and minting custom assets.
//
// The dev stack runs a stellar/quickstart docker in --local mode, so the
// client is hardcoded to its standalone-network endpoints (horizon at
// :8000, friendbot at :8000/friendbot).
package stellar

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/txnbuild"
)

// StandaloneNetworkPassphrase is the canonical passphrase the stellar/quickstart
// docker image uses in --local mode. There is no constant for it in the
// stellar SDK, so we hardcode the official string.
const StandaloneNetworkPassphrase = "Standalone Network ; February 2017"

type Client struct {
	Horizon       *horizonclient.Client
	NetworkPhrase string
	FriendbotURL  string
	httpClient    *http.Client
}

// NewClient returns a Client wired to the local stellar/quickstart standalone
// network: horizon at http://localhost:8000, friendbot at the same host.
func NewClient() (*Client, error) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return &Client{
		Horizon: &horizonclient.Client{
			HorizonURL: "http://localhost:8000/",
			HTTP:       httpClient,
		},
		NetworkPhrase: StandaloneNetworkPassphrase,
		FriendbotURL:  "http://localhost:8000/friendbot",
		httpClient:    httpClient,
	}, nil
}

// FundAccount asks friendbot to fund the given address.
//
// Retries transient 5xx responses with capped exponential backoff. The
// quickstart friendbot is prone to 502 Bad Gateway in the seconds
// immediately after a chain reset — supervisord brings up the
// horizon/friendbot/soroban-rpc stack with some startup race that takes
// 10–30s to settle even after horizon's `/` endpoint returns 200. So we
// retry generously (up to ~90s of retries) before giving up.
//
// 4xx errors are treated as permanent (e.g. address already funded).
func (c *Client) FundAccount(address string) error {
	const maxAttempts = 15
	const maxDelay = 8 * time.Second
	delay := 500 * time.Millisecond

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := c.httpClient.Get(c.FriendbotURL + "/?addr=" + address)
		if err != nil {
			lastErr = fmt.Errorf("friendbot request: %w", err)
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode < 400 {
				return nil
			}
			lastErr = fmt.Errorf("friendbot returned %d: %s", resp.StatusCode, truncate(body, 200))
			// 4xx (bad request, address already funded) is permanent — bail.
			if resp.StatusCode < 500 {
				return lastErr
			}
		}

		if attempt < maxAttempts {
			time.Sleep(delay)
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}
	return fmt.Errorf("friendbot %s after %d attempts: %w", address, maxAttempts, lastErr)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}

// LoadAccount fetches the on-chain account state for the given address.
func (c *Client) LoadAccount(address string) (hProtocol.Account, error) {
	return c.Horizon.AccountDetail(horizonclient.AccountRequest{AccountID: address})
}

// SubmitOps builds, signs and submits a transaction with the given operations,
// signed by the given source keypair (and any extra cosigners). Returns the
// horizon submit response, which includes the canonical transaction hash.
func (c *Client) SubmitOps(source *keypair.Full, ops []txnbuild.Operation, extraSigners ...*keypair.Full) (hProtocol.Transaction, error) {
	srcAccount, err := c.LoadAccount(source.Address())
	if err != nil {
		return hProtocol.Transaction{}, fmt.Errorf("load source account: %w", err)
	}

	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &srcAccount,
		IncrementSequenceNum: true,
		BaseFee:              txnbuild.MinBaseFee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
		Operations:           ops,
	})
	if err != nil {
		return hProtocol.Transaction{}, fmt.Errorf("build tx: %w", err)
	}

	signers := append([]*keypair.Full{source}, extraSigners...)
	for _, s := range signers {
		tx, err = tx.Sign(c.NetworkPhrase, s)
		if err != nil {
			return hProtocol.Transaction{}, fmt.Errorf("sign tx with %s: %w", s.Address(), err)
		}
	}

	resp, err := c.Horizon.SubmitTransaction(tx)
	if err != nil {
		return hProtocol.Transaction{}, fmt.Errorf("submit tx: %w", err)
	}
	return resp, nil
}

// SubmitOpsExpectFail builds and submits a transaction expected to fail at
// horizon submission. It returns nil only when horizon itself rejects the
// transaction (the expected case). Non-submission errors (build/sign, horizon
// unreachable) are propagated so callers don't get false positives. If the
// transaction unexpectedly succeeds, an error is returned.
func (c *Client) SubmitOpsExpectFail(source *keypair.Full, ops []txnbuild.Operation, extraSigners ...*keypair.Full) error {
	resp, err := c.SubmitOps(source, ops, extraSigners...)
	if err != nil {
		if _, ok := err.(*horizonclient.Error); ok {
			return nil
		}
		if wrapped := unwrapHorizonError(err); wrapped != nil {
			return nil
		}
		return fmt.Errorf("submit tx (expected horizon failure): %w", err)
	}
	return fmt.Errorf("expected submission to fail, got hash=%s ledger=%d", resp.Hash, resp.Ledger)
}

func unwrapHorizonError(err error) *horizonclient.Error {
	for err != nil {
		if he, ok := err.(*horizonclient.Error); ok {
			return he
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return nil
		}
		err = u.Unwrap()
	}
	return nil
}
