package captivecore

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/streamingfast/firehose-stellar/decoder"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// A Protocol 30 ledger header carries a CAP-0088 millisecond close-time
// StellarValue arm. The block timestamp comes from closeTime, which stays in
// seconds, not from closeTimeMs.
func TestConvertLedgerCloseMeta_Protocol30MillisecondCloseTime(t *testing.T) {
	sig := xdr.LedgerCloseValueSignature{
		NodeId:    xdr.NodeId{Type: xdr.PublicKeyTypePublicKeyTypeEd25519, Ed25519: &xdr.Uint256{1}},
		Signature: xdr.Signature{2},
	}

	const closeTimeMs = 1_794_000_000_437
	tests := map[string]xdr.StellarValueExt{
		"SIGNED_MS": {
			V:             xdr.StellarValueTypeStellarValueSignedMs,
			SignedMsValue: &xdr.StellarValueSignedMsValue{CloseTimeMs: closeTimeMs, LcValueSignature: sig},
		},
		"EMPTY_TX_SET_MS": {
			V:               xdr.StellarValueTypeStellarValueEmptyTxSetMs,
			ProposedMsValue: &xdr.StellarValueProposedMsValue{CloseTimeMs: closeTimeMs, PreviousLedgerVersion: 30, LcValueSignature: sig},
		},
	}

	for name, ext := range tests {
		t.Run(name, func(t *testing.T) {
			meta := xdr.LedgerCloseMeta{
				V: 2,
				V2: &xdr.LedgerCloseMetaV2{
					LedgerHeader: xdr.LedgerHeaderHistoryEntry{
						Hash: xdr.Hash{0xaa},
						Header: xdr.LedgerHeader{
							LedgerVersion:      30,
							PreviousLedgerHash: xdr.Hash{0xbb},
							ScpValue:           xdr.StellarValue{CloseTime: closeTimeMs / 1000, Ext: ext},
							LedgerSeq:          1000,
						},
					},
					TxSet: xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{}},
				},
			}

			encoded, err := xdr.MarshalBase64(meta)
			require.NoError(t, err)

			decoded, err := decoder.NewDecoder(zap.NewNop()).DecodeLedgerMetadata(encoded)
			require.NoError(t, err)
			require.Equal(t, ext.V, decoded.V2.LedgerHeader.Header.ScpValue.Ext.V)

			fetcher := &Fetcher{NetworkPassphrase: network.TestNetworkPassphrase, Logger: zap.NewNop()}
			blk, err := fetcher.ConvertLedgerCloseMetaToBstreamBlock(decoded)
			require.NoError(t, err)
			require.Equal(t, uint64(1000), blk.Number)
			require.Equal(t, int64(closeTimeMs/1000), blk.Timestamp.Seconds)
			require.Zero(t, blk.Timestamp.Nanos)

			stellarBlk := &pbstellar.Block{}
			require.NoError(t, blk.Payload.UnmarshalTo(stellarBlk))
			require.Equal(t, uint32(30), stellarBlk.Header.LedgerVersion)
			require.Empty(t, stellarBlk.Transactions)
		})
	}
}
