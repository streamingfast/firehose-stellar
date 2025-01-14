package rpc

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_GetLatestLedger(t *testing.T) {
	c := NewClient(os.Getenv("HORIZON_URL"), nil)
	ledger, err := c.GetLatestLedger()
	require.NoError(t, err)
	require.NotZero(t, ledger)
}

func Test_GetLedgers(t *testing.T) {
	c := NewClient(os.Getenv("HORIZON_URL"), nil)
	ledger, err := c.GetLatestLedger()
	require.NoError(t, err)
	ledgers, err := c.GetLedgers(uint64(ledger.Sequence))
	require.NoError(t, err)
	require.NotEmpty(t, ledgers)
	require.Equal(t, 1, len(ledgers))
}

func Test_GetTransactions(t *testing.T) {
	c := NewClient(os.Getenv("HORIZON_URL"), nil)
	cursor, transactions, err := c.GetTransactions(55253344, 100, "")
	require.NoError(t, err)
	require.NotNil(t, transactions)
	require.Equal(t, 140, len(transactions))
	require.NotEmpty(t, cursor)
}

func Test_GetEvents(t *testing.T) {
	c := NewClient(os.Getenv("HORIZON_URL"), nil)
	cursor, events, err := c.GetEvents(55253344, 100, "")
	require.NoError(t, err)
	require.NotEmpty(t, events)
	require.NotEmpty(t, cursor)
}
