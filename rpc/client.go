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
	decoder := json.NewDecoder(bytes.NewBuffer(body))
	decoder.DisallowUnknownFields() // Fail on unknown fields
	err = decoder.Decode(&response)
	if err != nil {
		return nil, fmt.Errorf("original body: %s failed to unmarshal JSON: %w", string(body), err)
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
	decoder := json.NewDecoder(bytes.NewBuffer(body))
	decoder.DisallowUnknownFields() // Fail on unknown fields
	err = decoder.Decode(&response)
	if err != nil {
		return nil, fmt.Errorf("original body: %s failed to unmarshal JSON: %w", string(body), err)
	}

	return response.Result.Ledgers, nil
}

// TODO: find out the limit from the RPC Provider and set it in the pagination or should we use the value
// 		from the header metadata of the ledger which gives out a number of transactions per ledger

// GetTransactions returns the transactions for a given ledger, it will return successful and failed transactions
func (c *Client) GetTransactions(ledgerNum uint64, limit int, lastCursor string) ([]types.Transaction, error) {
	transactions := make([]types.Transaction, 0)

	for {
		currentCursor, fetchedTransactions, err := c.getTransactions(ledgerNum, limit, lastCursor)
		if err != nil {
			return nil, fmt.Errorf("failed to get transactions: %w", err)
		}

		allTransactionsFetched := len(fetchedTransactions) == 0 || currentCursor == ""

		for _, f := range fetchedTransactions {
			if f.Ledger != ledgerNum {
				allTransactionsFetched = true
				break
			}
			transactions = append(transactions, f)
		}

		if allTransactionsFetched {
			break
		}
		lastCursor = currentCursor
	}

	return transactions, nil
}

func (c *Client) getTransactions(ledgerNum uint64, limit int, cursor string) (string, []types.Transaction, error) {
	payload := types.NewGetTransactionsRquest(ledgerNum, types.NewPagination(limit, cursor))

	rpcBody, err := json.Marshal(payload)
	if err != nil {
		return cursor, nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	body, err := c.makeRquest(rpcBody)
	if err != nil {
		return cursor, nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	var transactions types.GetTransactionResponse
	decoder := json.NewDecoder(bytes.NewBuffer(body))
	decoder.DisallowUnknownFields() // Fail on unknown fields
	err = decoder.Decode(&transactions)
	if err != nil {
		return cursor, nil, fmt.Errorf("original body: %s failed to unmarshal JSON: %w", string(body), err)
	}

	cursor = transactions.Result.Cursor
	return cursor, transactions.Result.Transactions, nil
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
