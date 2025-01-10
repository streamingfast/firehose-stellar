package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/streamingfast/firehose-stellar/types"
	"go.uber.org/zap"
)

type Client struct {
	rpcEndpoint string
	httpClient  *http.Client
	logger      *zap.Logger
}

func NewClient(rpcEndpoint string, logger *zap.Logger) *Client {
	return &Client{
		rpcEndpoint: rpcEndpoint,
		httpClient:  http.DefaultClient,
		logger:      logger,
	}
}

func (c *Client) GetLatestLedger() (*types.GetLatestLedgerResult, error) {
	payload := types.NewLatestLedgerRequest()

	rpcBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	body, err := c.makeRquest(rpcBody)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest ledger: %w", err)
	}

	var response types.GetLatestLedgerResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &response.Result, nil
}

// GetLedgers returns the ledgers for a given number
func (c *Client) GetLedgers(startLedgerNum uint64) ([]types.Ledger, error) {
	payload := types.NewLedgerRequest(startLedgerNum, &types.Pagination{Limit: 1})

	rpcBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	body, err := c.makeRquest(rpcBody)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledgers: %w", err)
	}

	var response types.GetLedgersResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return response.Result.Ledgers, nil
}

// GetTransactions returns the transactions for a given ledger
func (c *Client) GetTransactions(ledgerNum uint64) ([]types.Transaction, error) {
	payload := types.NewGetTransactionsRquest(ledgerNum, nil)

	rpcBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	body, err := c.makeRquest(rpcBody)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	var transactions types.GetTransactionResponse
	err = json.Unmarshal(body, &transactions)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return transactions.Result.Transactions, nil
}

// GetEvents returns the events for a given ledger
func (c *Client) GetEvents(ledgerNum uint64) ([]*types.Event, error) {
	payload := types.NewEventsRequest(ledgerNum, nil)

	rpcBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	body, err := c.makeRquest(rpcBody)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	var response types.GetEventsResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil, nil
}

func (c *Client) makeRquest(reqBody []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", c.rpcEndpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}
