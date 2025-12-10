package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const RPC_MAINNET_ENDPOINT = "https://mainnet.sorobanrpc.com"

// const RPC_TESTNET_ENDPOINT = "https://soroban-rpc.testnet.stellar.gateway.fm"
const RPC_TESTNET_ENDPOINT = "https://blue-stylish-spree.stellar-testnet.quiknode.pro/a631f833abb51e32c79012ac81783d1faf18734a/"

func Test_GetLatestLedger(t *testing.T) {
	c := NewClient(RPC_MAINNET_ENDPOINT, zap.NewNop(), nil)
	ledger, err := c.GetLatestLedger(context.Background())
	require.NoError(t, err)
	require.NotZero(t, ledger)
}

func Test_GetLedgers(t *testing.T) {
	c := NewClient(RPC_MAINNET_ENDPOINT, zap.NewNop(), nil)
	ledger, err := c.GetLatestLedger(context.Background())
	require.NoError(t, err)
	ledgers, err := c.GetLedgers(context.Background(), uint64(ledger.Sequence))
	require.NoError(t, err)
	require.NotEmpty(t, ledgers)
	require.Equal(t, 1, len(ledgers))
}

func Test_GetTransactions(t *testing.T) {
	c := NewClient(RPC_MAINNET_ENDPOINT, zap.NewNop(), nil)
	ledger, err := c.GetLatestLedger(context.Background())
	require.NoError(t, err)
	transactions, err := c.GetTransactions(context.Background(), uint64(ledger.Sequence), 100, "")
	require.NoError(t, err)
	require.NotNil(t, transactions)
}

func Test_GetTransactionsWithEvents(t *testing.T) {
	c := NewClient(RPC_MAINNET_ENDPOINT, zap.NewNop(), nil)
	ledger, err := c.GetLatestLedger(context.Background())
	require.NoError(t, err)
	transactions, err := c.GetTransactions(context.Background(), uint64(ledger.Sequence), 100, "")
	require.NoError(t, err)
	require.NotNil(t, transactions)
}

func Test_GetTransactionsWithLimitTooHigh(t *testing.T) {
	c := NewClient("https://mainnet.sorobanrpc.com", zap.NewNop(), nil)
	ledger, err := c.GetLatestLedger(context.Background())
	require.NoError(t, err)
	_, err = c.GetTransactions(context.Background(), uint64(ledger.Sequence), 2000, "")
	require.Error(t, err)
}
