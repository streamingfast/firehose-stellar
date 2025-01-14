package types

import (
	xdrTypes "github.com/stellar/go/xdr"
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
	Status              string   `json:"status"`
	TxHash              string   `json:"txHash"`
	ApplicationOrder    int      `json:"applicationOrder"`
	FeeBump             bool     `json:"feeBump"`
	EnvelopeXdr         string   `json:"envelopeXdr"`
	ResultXdr           string   `json:"resultXdr"`
	ResultMetaXdr       string   `json:"resultMetaXdr"`
	DiagnosticEventsXdr []string `json:"diagnosticEventsXdr"`
	Ledger              uint64   `json:"ledger"`
	CreatedAt           uint64   `json:"createdAt"`
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
	Hash string
	Meta *xdrTypes.TransactionMeta
}

func NewTransactionMeta(hash string, meta *xdrTypes.TransactionMeta) *TransactionMeta {
	return &TransactionMeta{
		Hash: hash,
		Meta: meta,
	}
}
