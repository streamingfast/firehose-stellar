package rpc

import (
	"context"
	"testing"
	"time"

	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"github.com/stretchr/testify/require"
)

func Test_Fetch(t *testing.T) {
	c := NewClient(RPC_MAINNET_ENDPOINT, testLog, testTracer)

	ledger, err := c.GetLatestLedger(context.Background())
	require.NoError(t, err)

	f := NewFetcher(time.Second, time.Second, 200, true, testLog)
	b, _, err := f.Fetch(context.Background(), c, uint64(ledger.Sequence))

	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, uint64(ledger.Sequence), b.Number)
}

func Test_FetchSpecificLedger(t *testing.T) {
	t.Skip("This test leads to fetching ledger: rpc error: JSON-RPC error: [-32003] request failed to process due to internal issue on our provider, check back in some time to re-activate")

	const BLOCK_TO_FETCH = uint64(60132634)

	c := NewClient(RPC_MAINNET_ENDPOINT, testLog, testTracer)
	f := NewFetcher(time.Second, time.Second, 200, true, testLog)
	b, _, err := f.Fetch(context.Background(), c, BLOCK_TO_FETCH)
	require.NoError(t, err)

	stellarBlock := &pbstellar.Block{}
	require.NoError(t, b.Payload.UnmarshalTo(stellarBlock))

	require.NotNil(t, b)
	require.Equal(t, BLOCK_TO_FETCH, b.Number)
	require.Equal(t, 252, len(stellarBlock.Transactions))
}

func Test_FetchSpecificLedger_Testnet(t *testing.T) {
	//t.Skip("Testnet endpoint resets from time to time, so this test cannot last in time, adjust the block number to test it again correctly")

	const BLOCK_TO_FETCH = uint64(342805)
	const EXPECTED_TRANSACTION_COUNT = 3

	c := NewClient(RPC_TESTNET_ENDPOINT, testLog, testTracer)
	f := NewFetcher(time.Second, time.Second, 200, false, testLog)
	b, _, err := f.Fetch(context.Background(), c, BLOCK_TO_FETCH)
	require.NoError(t, err)

	stellarBlock := &pbstellar.Block{}
	require.NoError(t, b.Payload.UnmarshalTo(stellarBlock))

	require.NotNil(t, b)
	require.Equal(t, BLOCK_TO_FETCH, b.Number)

	require.Equal(t, EXPECTED_TRANSACTION_COUNT, len(stellarBlock.Transactions))
}
