package fix

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

// A real-looking 32-byte Stellar ledger hash (lowercase hex, 64 chars).
const knownHashHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
const knownPrevHex = "ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100"

// brokenBytes reproduces what the v1 RPC fetcher stored in
// pbstellar.Block.Hash and pbstellar.Block.Header.PreviousLedgerHash:
// it decoded the hex hash string as base64 (which silently succeeds
// because hex chars are a subset of the base64 alphabet) yielding 48
// bytes of garbage.
func brokenBytes(t *testing.T, hexStr string) []byte {
	t.Helper()
	out, err := base64.StdEncoding.DecodeString(hexStr)
	require.NoError(t, err)
	require.Len(t, out, 48, "v1 garbage bytes are 48 bytes long")
	return out
}

func TestConvertBrokenHash_RoundTrip(t *testing.T) {
	broken := brokenBytes(t, knownHashHex)

	recovered, err := ConvertBrokenHash(broken)
	require.NoError(t, err)
	require.Len(t, recovered, stellarHashLen)

	want, err := hex.DecodeString(knownHashHex)
	require.NoError(t, err)
	require.Equal(t, want, recovered, "recovered bytes must equal the original 32-byte hash")
}

func TestConvertBrokenHash_RejectsWrongLength(t *testing.T) {
	// 32 bytes input (looks like a valid hash) would base64-encode to 44
	// chars, which is not a valid hex length — hex.DecodeString returns
	// an error, so ConvertBrokenHash surfaces that.
	_, err := ConvertBrokenHash(make([]byte, 32))
	require.Error(t, err)
}

func TestFixBlock_RecoversBothHashes(t *testing.T) {
	src := buildBrokenBlock(t, 60_000_001, knownHashHex, knownPrevHex)

	fixed, err := FixBlock(src)
	require.NoError(t, err)

	var stellar pbstellar.Block
	require.NoError(t, fixed.Payload.UnmarshalTo(&stellar))

	wantHash, _ := hex.DecodeString(knownHashHex)
	wantPrev, _ := hex.DecodeString(knownPrevHex)
	require.Equal(t, wantHash, stellar.Hash)
	require.Equal(t, wantPrev, stellar.Header.PreviousLedgerHash)

	// bstream.Id/ParentId on the v1 block are already the correct hex
	// strings (base64.Encode(base64.Decode(hex)) is identity over the
	// 64-char hex alphabet), so they pass through untouched.
	require.Equal(t, knownHashHex, fixed.Id)
	require.Equal(t, knownPrevHex, fixed.ParentId)
}

func TestFixBlock_PreservesNonHashFields(t *testing.T) {
	src := buildBrokenBlock(t, 60_000_002, knownHashHex, knownPrevHex)

	var origStellar pbstellar.Block
	require.NoError(t, src.Payload.UnmarshalTo(&origStellar))

	fixed, err := FixBlock(src)
	require.NoError(t, err)

	require.Equal(t, src.Number, fixed.Number)
	require.Equal(t, src.Timestamp, fixed.Timestamp)
	require.Equal(t, src.LibNum, fixed.LibNum)
	require.Equal(t, src.ParentNum, fixed.ParentNum)

	var fixedStellar pbstellar.Block
	require.NoError(t, fixed.Payload.UnmarshalTo(&fixedStellar))

	require.Equal(t, origStellar.Version, fixedStellar.Version)
	require.Equal(t, origStellar.Header.LedgerVersion, fixedStellar.Header.LedgerVersion)
	require.Equal(t, origStellar.Header.TotalCoins, fixedStellar.Header.TotalCoins)
	require.Equal(t, origStellar.Header.BaseFee, fixedStellar.Header.BaseFee)
	require.Equal(t, origStellar.Header.BaseReserve, fixedStellar.Header.BaseReserve)
	require.Len(t, fixedStellar.Transactions, len(origStellar.Transactions))
}

func TestFixBlock_FailsWhenIdMismatch(t *testing.T) {
	src := buildBrokenBlock(t, 60_000_003, knownHashHex, knownPrevHex)
	src.Id = "deadbeef" // intentionally wrong

	_, err := FixBlock(src)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not match bstream Id")
}

// buildBrokenBlock constructs a pbbstream.Block in exactly the shape
// the v1 RPC fetcher produced: pbstellar.Block.Hash and
// Header.PreviousLedgerHash hold base64.Decode(hex) garbage, while the
// bstream Id/ParentId hold the original hex string (because
// base64.Encode(base64.Decode(hex)) is identity for 64-char hex
// alphabet inputs).
func buildBrokenBlock(t *testing.T, num uint64, hashHex, prevHex string) *pbbstream.Block {
	t.Helper()

	stellar := &pbstellar.Block{
		Number:  num,
		Version: 1,
		Hash:    brokenBytes(t, hashHex),
		Header: &pbstellar.Header{
			LedgerVersion:      23,
			PreviousLedgerHash: brokenBytes(t, prevHex),
			TotalCoins:         105_443_902_087_300_000,
			BaseFee:            100,
			BaseReserve:        5_000_000,
		},
		Transactions: []*pbstellar.Transaction{
			{ApplicationOrder: 1},
		},
	}

	payload, err := anypb.New(stellar)
	require.NoError(t, err)

	return &pbbstream.Block{
		Number:    num,
		Id:        hashHex,
		ParentId:  prevHex,
		LibNum:    num - 1,
		ParentNum: num - 1,
		Payload:   payload,
	}
}
