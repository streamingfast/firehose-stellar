package rpc

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/firehose-stellar/decoder"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"github.com/streamingfast/firehose-stellar/types"
	"github.com/streamingfast/firehose-stellar/utils"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type LastBlockInfo struct {
	blockNum uint64
	cursor   string
}

func NewLastBlockInfo() *LastBlockInfo {
	return &LastBlockInfo{}
}

type Fetcher struct {
	fetchInterval            time.Duration
	latestBlockRetryInterval time.Duration
	lastBlockInfo            *LastBlockInfo
	decoder                  *decoder.Decoder
	transactionFetchLimit    int

	logger    *zap.Logger
	isMainnet bool

	// Statistics
	acquisitionTimes      []time.Duration
	conversionTimes       []time.Duration
	totalTimes            []time.Duration
	interCallDelays       []time.Duration
	lastFetchStart        time.Time
	blocksFetchedInPeriod int
	statsTicker           *time.Ticker
}

func NewFetcher(fetchInterval, latestBlockRetryInterval time.Duration, transactionFetchLimit int, isMainnet bool, logger *zap.Logger) *Fetcher {
	f := &Fetcher{
		fetchInterval:            fetchInterval,
		latestBlockRetryInterval: latestBlockRetryInterval,
		lastBlockInfo:            NewLastBlockInfo(),
		decoder:                  decoder.NewDecoder(logger),
		transactionFetchLimit:    transactionFetchLimit,
		logger:                   logger,
		isMainnet:                isMainnet,
		acquisitionTimes:         make([]time.Duration, 0, 50),
		conversionTimes:          make([]time.Duration, 0, 50),
		totalTimes:               make([]time.Duration, 0, 50),
		blocksFetchedInPeriod:    0,
		statsTicker:              time.NewTicker(10 * time.Second),
	}

	// Start statistics logging goroutine
	go f.logStatistics()

	return f
}

func (f *Fetcher) Fetch(ctx context.Context, client *Client, requestBlockNum uint64) (b *pbbstream.Block, skipped bool, err error) {
	fetchStart := time.Now()
	var interCallDelay time.Duration
	if !f.lastFetchStart.IsZero() {
		interCallDelay = fetchStart.Sub(f.lastFetchStart)
	}
	f.lastFetchStart = fetchStart
	sleepDuration := time.Duration(0)
	for f.lastBlockInfo.blockNum < requestBlockNum {
		time.Sleep(sleepDuration)

		latestLedger, err := client.GetLatestLedger(ctx)
		if err != nil {
			return nil, false, fmt.Errorf("fetching latest block num: %w", err)
		}

		f.lastBlockInfo.blockNum = uint64(latestLedger.Sequence)
		f.logger.Info("got latest block num", zap.Uint64("latest_block_num", f.lastBlockInfo.blockNum), zap.Uint64("requested_block_num", requestBlockNum), zap.Bool("keep", false))

		if f.lastBlockInfo.blockNum >= requestBlockNum {
			break
		}
		sleepDuration = f.latestBlockRetryInterval
	}

	ledgerStart := time.Now()
	ledger, err := client.GetLedgers(ctx, requestBlockNum)
	if err != nil {
		return nil, false, fmt.Errorf("fetching ledger: %w", err)
	}
	acquisitionEnd := time.Now()
	acquisitionTime := acquisitionEnd.Sub(ledgerStart)

	if len(ledger) == 0 {
		return nil, false, fmt.Errorf("ledger not found %d", requestBlockNum)
	}

	if len(ledger) > 1 {
		return nil, false, fmt.Errorf("multiple ledgers found for block %d", requestBlockNum)
	}

	ledgerTime, err := strconv.ParseInt(ledger[0].LedgerCloseTime, 10, 64)
	if err != nil {
		return nil, false, fmt.Errorf("parsing ledger time: %w", err)
	}

	ledgerMetadata, err := f.decoder.DecodeLedgerMetadata(ledger[0].MetadataXdr)
	if err != nil {
		return nil, false, fmt.Errorf("decoding ledger metadata: %w", err)
	}

	var ledgerHeader xdr.LedgerHeaderHistoryEntry
	switch {
	case ledgerMetadata.V0 != nil:
		ledgerHeader = ledgerMetadata.V0.LedgerHeader
	case ledgerMetadata.V2 != nil:
		ledgerHeader = ledgerMetadata.V2.LedgerHeader
	case ledgerMetadata.V1 != nil:
		ledgerHeader = ledgerMetadata.V1.LedgerHeader
	default:
		return nil, false, fmt.Errorf("ledger metadata does not contain V1 or V2 data: %v", ledgerMetadata)
	}

	// Extract transactions directly from ledger metadata (no fallback)
	transactions, err := f.extractTransactionsFromLedgerMetadata(ledgerMetadata)
	if err != nil {
		return nil, false, fmt.Errorf("extracting transactions from ledger metadata: %w", err)
	}

	transactionMetas := make([]*types.TransactionMeta, 0)
	for _, trx := range transactions {
		txHashBytes, err := hex.DecodeString(trx.TxHash)
		if err != nil {
			return nil, false, fmt.Errorf("decoding transaction hash: %w", err)
		}
		txEnvelopeBytes, err := base64.StdEncoding.DecodeString(trx.EnvelopeXdr)
		if err != nil {
			return nil, false, fmt.Errorf("decoding transaction envelope: %w", err)
		}
		txResultBytes, err := base64.StdEncoding.DecodeString(trx.ResultXdr)
		if err != nil {
			return nil, false, fmt.Errorf("decoding transaction result: %w", err)
		}

		events := &pbstellar.Events{}
		if trx.Events != nil {
			diagnosticEvents := make([][]byte, 0)
			for _, event := range trx.Events.DiagnosticEventsXdr {
				decodedEvent, err := base64.StdEncoding.DecodeString(event)
				if err != nil {
					return nil, false, fmt.Errorf("decoding diagnostic event: %w", err)
				}

				diagnosticEvents = append(diagnosticEvents, decodedEvent)
			}

			transactionsEvents := make([][]byte, 0)
			for _, event := range trx.Events.TransactionEventsXdr {
				decodedEvent, err := base64.StdEncoding.DecodeString(event)
				if err != nil {
					return nil, false, fmt.Errorf("decoding transaction event: %w", err)
				}

				transactionsEvents = append(transactionsEvents, decodedEvent)
			}

			contractEvents := make([]*pbstellar.ContractEvent, 0)
			for _, events := range trx.Events.ContractEventsXdr {
				innerContractEvents := make([][]byte, 0)
				for _, event := range events {
					decodedEvent, err := base64.StdEncoding.DecodeString(event)
					if err != nil {
						return nil, false, fmt.Errorf("decoding contract event: %w", err)
					}

					innerContractEvents = append(innerContractEvents, decodedEvent)
				}
				contractEvents = append(contractEvents, &pbstellar.ContractEvent{
					Events: innerContractEvents,
				})
			}

			events.DiagnosticEventsXdr = diagnosticEvents
			events.TransactionEventsXdr = transactionsEvents
			events.ContractEventsXdr = contractEvents
		}

		transactionMetas = append(transactionMetas,
			types.NewTransactionMeta(txHashBytes, trx.Status, txEnvelopeBytes, txResultBytes, events),
		)
	}

	stellarTransactions := make([]*pbstellar.Transaction, 0)
	for i, trx := range transactionMetas {
		stellarTransactions = append(stellarTransactions, &pbstellar.Transaction{
			Hash:             trx.Hash,
			Status:           utils.ConvertTransactionStatus(trx.Status),
			CreatedAt:        timestamppb.New(time.Unix(ledgerTime, 0)),
			ApplicationOrder: uint64(i + 1),
			EnvelopeXdr:      trx.EnveloppeXdr,
			ResultXdr:        trx.ResultXdr,
			Events:           trx.Events,
		})
		if trx.Events != nil {
			if len(trx.Events.DiagnosticEventsXdr) > 0 || len(trx.Events.TransactionEventsXdr) > 0 || len(trx.Events.ContractEventsXdr) > 0 {
				f.logger.Debug("transaction has events", zap.String("tx_hash", base64.StdEncoding.EncodeToString(trx.Hash)))
			}
		}
	}

	ledgerHashBytes, err := base64.StdEncoding.DecodeString(ledger[0].Hash)
	if err != nil {
		return nil, false, fmt.Errorf("decoding ledger hash: %w", err)
	}

	previousLedgerHashBytes, err := base64.StdEncoding.DecodeString(ledgerHeader.Header.PreviousLedgerHash.HexString())
	if err != nil {
		return nil, false, fmt.Errorf("decoding previous ledger hash: %w", err)
	}

	stellarBlk := &pbstellar.Block{
		Number: ledger[0].Sequence,
		Hash:   ledgerHashBytes,
		Header: &pbstellar.Header{
			LedgerVersion:      uint32(ledgerHeader.Header.LedgerVersion),
			PreviousLedgerHash: previousLedgerHashBytes,
			TotalCoins:         int64(ledgerHeader.Header.TotalCoins),
			BaseFee:            uint32(ledgerHeader.Header.BaseFee),
			BaseReserve:        uint32(ledgerHeader.Header.BaseReserve),
		},
		Version:      1,
		Transactions: stellarTransactions,
		CreatedAt:    timestamppb.New(time.Unix(ledgerTime, 0)),
	}

	bstreamBlock, err := convertBlock(stellarBlk)
	if err != nil {
		return nil, false, fmt.Errorf("converting block: %w", err)
	}

	// reset the cursor
	f.lastBlockInfo.cursor = ""

	// Update statistics
	conversionTime := time.Since(acquisitionEnd)
	totalTime := time.Since(fetchStart)
	f.updateStatistics(acquisitionTime, conversionTime, totalTime, interCallDelay)

	return bstreamBlock, false, nil
}

func (f *Fetcher) logStatistics() {
	for range f.statsTicker.C {
		f.logger.Info("block fetch statistics",
			zap.Int("blocks_fetched_in_period", f.blocksFetchedInPeriod),
			zap.Duration("avg_acquisition_time", f.averageDuration(f.acquisitionTimes)),
			zap.Duration("avg_conversion_time", f.averageDuration(f.conversionTimes)),
			zap.Duration("avg_total_time", f.averageDuration(f.totalTimes)),
			zap.Duration("avg_inter_call_delay", f.averageDuration(f.interCallDelays)),
		)
		f.blocksFetchedInPeriod = 0
	}
}

func (f *Fetcher) updateStatistics(acquisitionTime, conversionTime, totalTime, interCallDelay time.Duration) {
	f.acquisitionTimes = append(f.acquisitionTimes, acquisitionTime)
	if len(f.acquisitionTimes) > 50 {
		f.acquisitionTimes = f.acquisitionTimes[1:]
	}
	f.conversionTimes = append(f.conversionTimes, conversionTime)
	if len(f.conversionTimes) > 50 {
		f.conversionTimes = f.conversionTimes[1:]
	}
	f.totalTimes = append(f.totalTimes, totalTime)
	if len(f.totalTimes) > 50 {
		f.totalTimes = f.totalTimes[1:]
	}
	if interCallDelay > 0 {
		f.interCallDelays = append(f.interCallDelays, interCallDelay)
		if len(f.interCallDelays) > 50 {
			f.interCallDelays = f.interCallDelays[1:]
		}
	}
	f.blocksFetchedInPeriod++
}

func (f *Fetcher) averageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}

func (f *Fetcher) extractTransactionsFromLedgerMetadata(ledgerMetadata *xdr.LedgerCloseMeta) ([]types.Transaction, error) {
	// Use the Stellar SDK's LedgerTransactionReader to extract transactions from ledger metadata
	// This is the proper way to access transaction data from LedgerCloseMeta

	passphrase := network.PublicNetworkPassphrase
	if !f.isMainnet {
		passphrase = network.TestNetworkPassphrase
	}
	reader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(passphrase, *ledgerMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create ledger transaction reader: %w", err)
	}
	defer reader.Close()

	transactions := make([]types.Transaction, 0)
	for {
		tx, err := reader.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("failed to read transaction: %w", err)
		}

		// Convert the ingest.LedgerTransaction to our types.Transaction format
		transaction, err := f.convertLedgerTransactionToTypes(tx)
		if err != nil {
			return nil, fmt.Errorf("failed to convert transaction: %w", err)
		}

		transactions = append(transactions, *transaction)
	}

	return transactions, nil
}

func (f *Fetcher) convertLedgerTransactionToTypes(tx ingest.LedgerTransaction) (*types.Transaction, error) {
	// Convert from ingest.LedgerTransaction to types.Transaction

	// Get transaction envelope
	envelopeXdr, err := tx.Envelope.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal envelope: %w", err)
	}
	envelopeXdrStr := base64.StdEncoding.EncodeToString(envelopeXdr)

	// Get transaction result (just the Result part, not the pair)
	resultXdr, err := tx.Result.Result.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	resultXdrStr := base64.StdEncoding.EncodeToString(resultXdr)

	txHash := tx.Result.TransactionHash.HexString()

	// Determine status
	status := "UNKNOWN"
	if tx.Successful() {
		status = "SUCCESS"
	} else {
		status = "FAILED"
	}

	// Convert events
	events, err := f.convertLedgerTransactionEventsToRPCEvents(tx)
	if err != nil {
		f.logger.Warn("failed to convert events", zap.Error(err))
		events = nil
	}

	return &types.Transaction{
		TxHash:      txHash,
		EnvelopeXdr: envelopeXdrStr,
		ResultXdr:   resultXdrStr,
		Status:      status,
		Events:      events,
	}, nil
}

func (f *Fetcher) convertLedgerTransactionEventsToRPCEvents(tx ingest.LedgerTransaction) (*types.RPCEvents, error) {
	rpcEvents := &types.RPCEvents{
		DiagnosticEventsXdr:  make([]string, 0),
		TransactionEventsXdr: make([]string, 0),
		ContractEventsXdr:    make([][]string, 0),
	}

	// Get diagnostic events using tx.GetDiagnosticEvents()
	diagnosticEvents, err := tx.GetDiagnosticEvents()
	if err != nil {
		return nil, fmt.Errorf("failed to get diagnostic events: %w", err)
	}
	for _, event := range diagnosticEvents {
		eventXdr, err := event.MarshalBinary()
		if err != nil {
			continue
		}
		rpcEvents.DiagnosticEventsXdr = append(rpcEvents.DiagnosticEventsXdr, base64.StdEncoding.EncodeToString(eventXdr))
	}

	// Get transaction events using tx.GetTransactionEvents()
	transactionEvents, err := tx.GetTransactionEvents()
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction events: %w", err)
	}
	for _, event := range transactionEvents.TransactionEvents {
		eventXdr, err := event.MarshalBinary()
		if err != nil {
			continue
		}
		rpcEvents.TransactionEventsXdr = append(rpcEvents.TransactionEventsXdr, base64.StdEncoding.EncodeToString(eventXdr))
	}

	// Get contract events grouped by operation using transactionEvents.OperationEvents
	for _, operationEvents := range transactionEvents.OperationEvents {
		operationEventStrings := make([]string, 0, len(operationEvents))
		if operationEvents == nil {
			continue
		}
		for _, event := range operationEvents {
			eventXdr, err := event.MarshalBinary()
			if err != nil {
				continue
			}
			operationEventStrings = append(operationEventStrings, base64.StdEncoding.EncodeToString(eventXdr))
		}

		//if len(operationEventStrings) > 0 {
		rpcEvents.ContractEventsXdr = append(rpcEvents.ContractEventsXdr, operationEventStrings)
		//}
	}

	return rpcEvents, nil
}

func (f *Fetcher) IsBlockAvailable(blockNum uint64) bool {
	return blockNum <= f.lastBlockInfo.blockNum
}

func convertBlock(stellarBlk *pbstellar.Block) (*pbbstream.Block, error) {
	anyBlock, err := anypb.New(stellarBlk)
	if err != nil {
		return nil, fmt.Errorf("unable to create anypb: %w", err)
	}

	stellarBlockHash := base64.StdEncoding.EncodeToString(stellarBlk.Hash)
	previousStellarBlockHash := base64.StdEncoding.EncodeToString(stellarBlk.Header.PreviousLedgerHash)

	return &pbbstream.Block{
		Number:    stellarBlk.Number,
		Id:        stellarBlockHash,
		ParentId:  previousStellarBlockHash,
		Timestamp: stellarBlk.CreatedAt,
		LibNum:    stellarBlk.Number - 1, // every block in stellar is final
		ParentNum: stellarBlk.Number - 1, // every block in stellar is final
		Payload:   anyBlock,
	}, nil
}
