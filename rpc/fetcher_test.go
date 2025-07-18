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

	f := NewFetcher(time.Second, time.Second, 200, testLog)
	b, _, err := f.Fetch(context.Background(), c, uint64(ledger.Sequence))

	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, uint64(ledger.Sequence), b.Number)
}

func Test_FetchSpecificLedger(t *testing.T) {
	const BLOCK_TO_FETCH = uint64(58049417)

	c := NewClient(RPC_MAINNET_ENDPOINT, testLog, testTracer)
	f := NewFetcher(time.Second, time.Second, 200, testLog)
	b, _, err := f.Fetch(context.Background(), c, BLOCK_TO_FETCH)
	require.NoError(t, err)

	stellarBlock := &pbstellar.Block{}
	require.NoError(t, b.Payload.UnmarshalTo(stellarBlock))

	require.NotNil(t, b)
	require.Equal(t, BLOCK_TO_FETCH, b.Number)
	require.Equal(t, 665, len(stellarBlock.Transactions))
}

func Test_FetchSpecificLedger_ProtocolUpgrade23(t *testing.T) {
	const BLOCK_TO_FETCH = uint64(500201)

	c := NewClient(RPC_TESTNET_ENDPOINT, testLog, testTracer)
	f := NewFetcher(time.Second, time.Second, 200, testLog)
	b, _, err := f.Fetch(context.Background(), c, BLOCK_TO_FETCH)
	require.NoError(t, err)

	stellarBlock := &pbstellar.Block{}
	require.NoError(t, b.Payload.UnmarshalTo(stellarBlock))

	require.NotNil(t, b)
	require.Equal(t, BLOCK_TO_FETCH, b.Number)

	require.Equal(t, uint32(23), stellarBlock.Header.LedgerVersion)
	require.Equal(t, 8, len(stellarBlock.Transactions))
}
