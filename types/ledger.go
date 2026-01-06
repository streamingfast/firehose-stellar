package types

var _ error = (*RPCError)(nil)

type RPCError struct {
	Code    int     `json:"code"`
	Message string  `json:"message"`
	Data    *string `json:"data,omitempty"`
}

func (r *RPCError) Error() string {
	return "JSON-RPC error: " + r.Message
}

type LatestLedgerRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
}

func NewLatestLedgerRequest() *LatestLedgerRequest {
	return &LatestLedgerRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getLatestLedger",
	}
}

type GetLatestLedgerResponse struct {
	JSONRPC string                `json:"jsonrpc"`
	ID      int                   `json:"id"`
	Error   *RPCError             `json:"error,omitempty"`
	Result  GetLatestLedgerResult `json:"result"`
}

type GetLatestLedgerResult struct {
	ID              string `json:"id"`
	ProtocolVersion int    `json:"protocolVersion"`
	Sequence        int    `json:"sequence"`
	CloseTime       string `json:"closeTime"`
	HeaderXdr       string `json:"headerXdr"`
	MetadataXdr     string `json:"metadataXdr"`
}

type LedgerRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  Params `json:"params"`
}

func NewLedgerRequest(startLedger uint64, pagination *Pagination) LedgerRequest {
	return LedgerRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getLedgers",
		Params:  NewParams(startLedger, pagination),
	}
}

type GetLedgersResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      int              `json:"id"`
	Error   *RPCError        `json:"error,omitempty"`
	Result  GetLedgersResult `json:"result"`
}

type Ledger struct {
	Hash            string `json:"hash"`
	Sequence        uint64 `json:"sequence"`
	LedgerCloseTime string `json:"ledgerCloseTime"`
	HeaderXdr       string `json:"headerXdr"`
	MetadataXdr     string `json:"metadataXdr"`
}

type GetLedgersResult struct {
	Ledgers               []Ledger `json:"ledgers"`
	LatestLedger          uint64   `json:"latestLedger"`
	LatestLedgerCloseTime uint64   `json:"latestLedgerCloseTime"`
	Oldestledger          uint64   `json:"oldestLedger"`
	OldestLedgerCloseTime uint64   `json:"oldestLedgerCloseTime"`
	Cursor                string   `json:"cursor"`
}
