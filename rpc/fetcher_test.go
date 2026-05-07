package rpc

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"github.com/stretchr/testify/require"
)

func Test_convertBlock_HexEncodesIDs(t *testing.T) {
	// Bytes chosen so standard base64 of these 32-byte hashes contains '/' —
	// guards against any regression back to base64, which would corrupt
	// firecore one-block filenames (path-separator collision).
	hash := []byte{
		0xff, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00, 0x00,
		0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
		0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00,
		0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10,
	}
	prev := []byte{
		0x00, 0xff, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff,
		0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90,
		0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10,
		0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
	}
	require.Contains(t, base64.StdEncoding.EncodeToString(hash), "/", "test fixture invariant: base64(hash) should contain '/' for this regression test to be meaningful")

	stellarBlk := &pbstellar.Block{
		Number: 12345,
		Hash:   hash,
		Header: &pbstellar.Header{PreviousLedgerHash: prev},
	}

	b, err := convertBlock(stellarBlk)
	require.NoError(t, err)

	require.Equal(t, hex.EncodeToString(hash), b.Id)
	require.Equal(t, hex.EncodeToString(prev), b.ParentId)
	for _, id := range []string{b.Id, b.ParentId} {
		require.False(t, strings.ContainsAny(id, "/+="), "block id must not contain base64-only chars: %q", id)
	}
}

func Test_Fetch(t *testing.T) {
	c := NewClient(RPC_MAINNET_ENDPOINT, testLog, testTracer)

	ledger, err := c.GetLatestLedger(context.Background())
	require.NoError(t, err)

	f := NewFetcher(time.Second, time.Second, 200, passphraseFor(c.rpcEndpoint), testLog)
	b, _, err := f.Fetch(context.Background(), c, uint64(ledger.Sequence))

	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, uint64(ledger.Sequence), b.Number)
}

func Test_FetchSpecificLedger(t *testing.T) {
	t.Skip("Some RPC are not archive so they won't see this ledger at some point")

	const BLOCK_TO_FETCH = uint64(61322487)

	c := NewClient(RPC_MAINNET_ENDPOINT, testLog, testTracer)
	f := NewFetcher(time.Second, time.Second, 200, passphraseFor(c.rpcEndpoint), testLog)
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
	f := NewFetcher(time.Second, time.Second, 200, passphraseFor(c.rpcEndpoint), testLog)
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
	f := NewFetcher(time.Second, time.Second, 200, passphraseFor(c.rpcEndpoint), testLog)
	b, _, err := f.Fetch(context.Background(), c, BLOCK_TO_FETCH)
	require.NoError(t, err)

	stellarBlock := &pbstellar.Block{}
	require.NoError(t, b.Payload.UnmarshalTo(stellarBlock))

	require.NotNil(t, b)
	require.Equal(t, BLOCK_TO_FETCH, b.Number)

	require.Equal(t, uint32(23), stellarBlock.Header.LedgerVersion)
	require.Equal(t, 3, len(stellarBlock.Transactions))
}
