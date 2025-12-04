package types

import (
	xdrTypes "github.com/stellar/go-stellar-sdk/xdr"
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

type GetTransactionRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  struct {
		Hash string `json:"hash"`
	} `json:"params"`
}

func NewGetTransactionRequest(hash string) GetTransactionRequest {
	return GetTransactionRequest{
		JSONRPC: "2.0",
		ID:      8675309,
		Method:  "getTransaction",
		Params: struct {
			Hash string `json:"hash"`
		}{Hash: hash},
	}
}

type GetTransactionsResponse struct {
	JSONRPC string                `json:"jsonrpc"`
	ID      int                   `json:"id"`
	Error   *RPCError             `json:"error,omitempty"`
	Result  GetTransactionsResult `json:"result"`
}

type GetTransactionResponse struct {
	JSONRPC string               `json:"jsonrpc"`
	ID      int                  `json:"id"`
	Error   *RPCError            `json:"error,omitempty"`
	Result  GetTransactionResult `json:"result"`
}

type GetTransactionResult struct {
	LatestLedger          uint64     `json:"latestLedger"`
	LatestLedgerCloseTime string     `json:"latestLedgerCloseTime"`
	OldestLedger          uint64     `json:"oldestLedger"`
	OldestLedgerCloseTime string     `json:"oldestLedgerCloseTime"`
	Status                string     `json:"status"`
	TxHash                string     `json:"txHash"`
	ApplicationOrder      int        `json:"applicationOrder"`
	FeeBump               bool       `json:"feeBump"`
	EnvelopeXdr           string     `json:"envelopeXdr"`
	ResultXdr             string     `json:"resultXdr"`
	ResultMetaXdr         string     `json:"resultMetaXdr"`
	Events                *RPCEvents `json:"events"`
	Ledger                uint64     `json:"ledger"`
	CreatedAt             string     `json:"createdAt"`
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
