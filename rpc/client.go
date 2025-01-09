package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	hClient "github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/protocols/horizon"
	"go.uber.org/zap"
)

type Client struct {
	horizonClient *hClient.Client

	logger *zap.Logger
}

func NewClient(horizonUrl string, logger *zap.Logger) *Client {
	c := &hClient.Client{
		HorizonURL: horizonUrl,
		HTTP:       http.DefaultClient,
	}

	return &Client{
		horizonClient: c,
		logger:        logger,
	}
}

func (c *Client) GetLatestLedger() (uint64, error) {
	type latestLedgerRequest struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
	}

	payload := latestLedgerRequest{
		JSONRPC: "2.0",
		ID:      8675309,
		Method:  "getLatestLedger",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequest("POST", c.horizonClient.HorizonURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.horizonClient.HTTP.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %w", err)
	}

	type latestLedgerResponse struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ID              string `json:"id"`
			ProtocolVersion int    `json:"protocolVersion"`
			Sequence        int    `json:"sequence"`
		} `json:"result"`
	}

	var response latestLedgerResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return uint64(response.Result.Sequence), nil
}

// TODO: check the method: StreamLedgers
func (c *Client) GetLedgers(blockNum uint64) ([]horizon.Ledger, error) {
	ledgerRequest := hClient.LedgerRequest{
		Limit: 100,
	}

	ledger, err := c.horizonClient.Ledgers(ledgerRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger: %w", err)
	}

	return ledger.Embedded.Records, nil
}

func (c *Client) GetTransactions(ledgerNum uint64) ([]horizon.Transaction, error) {
	transactionsRequest := hClient.TransactionRequest{
		Limit: 100,
	}
	transactions, err := c.horizonClient.Transactions(transactionsRequest)
	if err != nil {
		return nil, err
	}
	return transactions.Embedded.Records, nil
}

func (c *Client) GetEvents(ledgerNum uint64) ([]horizon.Transaction, error) {
	return nil, nil
}

/*
1. getLedgers to fetch all the blocks
2. getTransactions (call within a ledger)
3. getEvents (call within a ledger) -> this is the equivalent of logs + traces
*/
