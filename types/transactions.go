package types

import (
	xdrTypes "github.com/stellar/go/xdr"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
)

type GetTransactionsRquest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  Params `json:"params"`
}

func NewGetTransactionsRquest(startLedger uint64, pagination *Pagination) GetTransactionsRquest {
	return GetTransactionsRquest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getTransactions",
		Params:  NewParams(startLedger, pagination),
	}
}

type GetTransactionResponse struct {
	JSONRPC string                `json:"jsonrpc"`
	ID      int                   `json:"id"`
	Result  GetTransactionsResult `json:"result"`
}

type Transaction struct {
	Status           string `json:"status"`
	TxHash           string `json:"txHash"`
	ApplicationOrder int    `json:"applicationOrder"`
	FeeBump          bool   `json:"feeBump"`
	EnvelopeXdr      string `json:"envelopeXdr"`
	ResultXdr        string `json:"resultXdr"`
	ResultMetaXdr    string `json:"resultMetaXdr"`
	// deprecated, check the Events field instead
	DiagnosticEventsXdr []string   `json:"diagnosticEventsXdr"`
	Events              *RPCEvents `json:"events"`
	Ledger              uint64     `json:"ledger"`
	CreatedAt           uint64     `json:"createdAt"`
}

type GetTransactionsResult struct {
	Transactions               []Transaction `json:"transactions"`
	LatestLedger               uint64        `json:"latestLedger"`
	LatestLedgerCloseTimestamp uint64        `json:"latestLedgerCloseTimestamp"`
	OldestLedger               uint64        `json:"oldestLedger"`
	OldestLedgerCloseTimestamp uint64        `json:"oldestLedgerCloseTimestamp"`
	Cursor                     string        `json:"cursor"`
}

type TransactionMeta struct {
	Hash          []byte
	Status        string
	EnveloppeXdr  []byte
	ResultXdr     []byte
	ResultMetaXdr []byte
	Meta          *xdrTypes.TransactionMeta
	Events        *pbstellar.Events
}

func NewTransactionMeta(
	hash []byte,
	status string,
	envelopeXdr []byte,
	resultXdr []byte,
	resultMetaXdr []byte,
	events *pbstellar.Events,
) *TransactionMeta {
	return &TransactionMeta{
		Hash:          hash,
		Status:        status,
		EnveloppeXdr:  envelopeXdr,
		ResultXdr:     resultXdr,
		ResultMetaXdr: resultMetaXdr,
		Events:        events,
	}
}
