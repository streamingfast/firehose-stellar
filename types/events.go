package types

type Events struct {
	DiagnosticEventsXdr  []string   `json:"diagnosticEventsXdr"`
	TransactionEventsXdr []string   `json:"transactionEventsXdr"`
	ContractEventsXdr    [][]string `json:"contractEventsXdr"`
}

type EventsParams struct {
	StartLedger uint64      `json:"startLedger"`
	EndLedger   uint64      `json:"endLedger"`
	Pagination  *Pagination `json:"pagination"`
}

type EventsRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  Params `json:"params"`
}

func NewEventsRequest(startLedger uint64, pagination *Pagination) EventsRequest {
	return EventsRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getEvents",
		Params:  NewParams(startLedger, pagination),
	}
}

type GetEventsResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  GetEventsResult `json:"result"`
}

type Event struct {
	Type                     string   `json:"type"`
	Ledger                   uint64   `json:"ledger"`
	LedgerClosedAt           string   `json:"ledgerClosedAt"`
	ContractID               string   `json:"contractId"`
	ID                       string   `json:"id"`
	PagingToken              string   `json:"pagingToken"`
	InSuccessfulContractCall bool     `json:"inSuccessfulContractCall"`
	Topic                    []string `json:"topic"`
	Value                    string   `json:"value"`
	TxHash                   string   `json:"txHash"`
}

type GetEventsResult struct {
	LatestLedger int     `json:"latestLedger"`
	Events       []Event `json:"events"`
	Cursor       string  `json:"cursor"`
}
