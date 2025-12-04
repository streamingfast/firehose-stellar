package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/streamingfast/dhttp"
	"github.com/streamingfast/firehose-stellar/types"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

type Client struct {
	rpcEndpoint string
	httpClient  *http.Client
	logger      *zap.Logger
}

func NewClient(rpcEndpoint string, logger *zap.Logger, tracer logging.Tracer) *Client {
	return &Client{
		rpcEndpoint: rpcEndpoint,
		httpClient: &http.Client{
			Transport: dhttp.NewLoggingRoundTripper(logger, tracer, http.DefaultTransport),
			Timeout:   60 * time.Second, // Set a reasonable timeout for HTTP requests
		},
		logger: logger,
	}
}

func (c *Client) GetLatestLedger(ctx context.Context) (*types.GetLatestLedgerResult, error) {
	payload := types.NewLatestLedgerRequest()

	rpcBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	body, err := c.makeRequest(ctx, rpcBody)
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

	if response.Error != nil {
		return nil, fmt.Errorf("rpc error: %w", response.Error)
	}

	return &response.Result, nil
}

// GetLedgers returns the ledgers for a given number
func (c *Client) GetLedgers(ctx context.Context, startLedgerNum uint64) ([]types.Ledger, error) {
	payload := types.NewLedgerRequest(startLedgerNum, &types.Pagination{Limit: 1})

	rpcBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	body, err := c.makeRequest(ctx, rpcBody)
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

	if response.Error != nil {
		return nil, fmt.Errorf("rpc error: %w", response.Error)
	}

	// TODO: The cursor in response.Result.Cursor is available for pagination but not currently used
	// since we only request 1 ledger at a time

	return response.Result.Ledgers, nil
}

// TODO: find out the limit from the RPC Provider and set it in the pagination or should we use the value
// 		from the header metadata of the ledger which gives out a number of transactions per ledger

// GetTransactions returns the transactions for a given ledger, it will return successful and failed transactions
func (c *Client) GetTransactions(ctx context.Context, ledgerNum uint64, limit int, lastCursor string) ([]types.Transaction, error) {
	transactions := make([]types.Transaction, 0)

	for {
		currentCursor, fetchedTransactions, err := c.getTransactions(ctx, ledgerNum, limit, lastCursor)
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

func (c *Client) getTransactions(ctx context.Context, ledgerNum uint64, limit int, cursor string) (string, []types.Transaction, error) {
	payload := types.NewGetTransactionsRquest(ledgerNum, types.NewPagination(limit, cursor))

	rpcBody, err := json.Marshal(payload)
	if err != nil {
		return cursor, nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	body, err := c.makeRequest(ctx, rpcBody)
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

func (c *Client) makeRequest(ctx context.Context, reqBody []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.rpcEndpoint, bytes.NewBuffer(reqBody))
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
