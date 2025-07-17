package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Fetch(t *testing.T) {
	c := NewClient(RPC_ENDPOINT, testLog, testTracer)

	ledger, err := c.GetLatestLedger(context.Background())
	require.NoError(t, err)

	f := NewFetcher(time.Second, time.Second, 200, testLog)
	b, _, err := f.Fetch(context.Background(), c, uint64(ledger.Sequence))

	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, uint64(ledger.Sequence), b.Number)
}

func Test_FetchSpecificLedger(t *testing.T) {
	const BLOCK_TO_FETCH = uint64(58049417)

	c := NewClient(RPC_ENDPOINT, testLog, testTracer)
	f := NewFetcher(time.Second, time.Second, 200, testLog)
	b, _, err := f.Fetch(context.Background(), c, BLOCK_TO_FETCH)
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, BLOCK_TO_FETCH, b.Number)
}
