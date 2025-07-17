package types

// These are the events that are returned by the RPC server
type RPCEvents struct {
	DiagnosticEventsXdr  []string   `json:"diagnosticEventsXdr"`
	TransactionEventsXdr []string   `json:"transactionEventsXdr"`
	ContractEventsXdr    [][]string `json:"contractEventsXdr"`
}
