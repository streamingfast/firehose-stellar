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

	f := NewFetcher(time.Second, time.Second, 200, c.rpcEndpoint == RPC_MAINNET_ENDPOINT, testLog)
	b, _, err := f.Fetch(context.Background(), c, uint64(ledger.Sequence))

	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, uint64(ledger.Sequence), b.Number)
}

func Test_FetchSpecificLedger(t *testing.T) {
	t.Skip("Some RPC are not archive so they won't see this ledger at some point")

	const BLOCK_TO_FETCH = uint64(61322487)

	c := NewClient(RPC_MAINNET_ENDPOINT, testLog, testTracer)
	f := NewFetcher(time.Second, time.Second, 200, c.rpcEndpoint == RPC_MAINNET_ENDPOINT, testLog)
	b, _, err := f.Fetch(context.Background(), c, BLOCK_TO_FETCH)
	require.NoError(t, err)

	stellarBlock := &pbstellar.Block{}
	require.NoError(t, b.Payload.UnmarshalTo(stellarBlock))

	require.NotNil(t, b)
	require.Equal(t, BLOCK_TO_FETCH, b.Number)
	require.Equal(t, 252, len(stellarBlock.Transactions))
}

func Test_FetchSpecificLedger_Testnet(t *testing.T) {
	t.Skip("Testnet endpoint resets from time to time, so this test cannot last in time, adjust the block number to test it again correctly")

	const BLOCK_TO_FETCH = uint64(342805)
	const EXPECTED_TRANSACTION_COUNT = 3

	c := NewClient(RPC_TESTNET_ENDPOINT, testLog, testTracer)
	f := NewFetcher(time.Second, time.Second, 200, c.rpcEndpoint == RPC_MAINNET_ENDPOINT, testLog)
	b, _, err := f.Fetch(context.Background(), c, BLOCK_TO_FETCH)
	require.NoError(t, err)

	stellarBlock := &pbstellar.Block{}
	require.NoError(t, b.Payload.UnmarshalTo(stellarBlock))

	require.NotNil(t, b)
	require.Equal(t, BLOCK_TO_FETCH, b.Number)

	require.Equal(t, EXPECTED_TRANSACTION_COUNT, len(stellarBlock.Transactions))
}

func Test_FetchSpecificLedger_ProtocolUpgrade23_MetadataV2(t *testing.T) {
	t.Skip("Testnet endpoint resets from time to time, so this test cannot last in time, adjust the block number to test it again correctly")

	const BLOCK_TO_FETCH = uint64(2063)

	c := NewClient(RPC_TESTNET_ENDPOINT, testLog, testTracer)
	f := NewFetcher(time.Second, time.Second, 200, c.rpcEndpoint == RPC_MAINNET_ENDPOINT, testLog)
	b, _, err := f.Fetch(context.Background(), c, BLOCK_TO_FETCH)
	require.NoError(t, err)

	stellarBlock := &pbstellar.Block{}
	require.NoError(t, b.Payload.UnmarshalTo(stellarBlock))

	require.NotNil(t, b)
	require.Equal(t, BLOCK_TO_FETCH, b.Number)

	require.Equal(t, uint32(23), stellarBlock.Header.LedgerVersion)
	require.Equal(t, 3, len(stellarBlock.Transactions))
}
