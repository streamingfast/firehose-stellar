package rpc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/firehose-stellar/decoder"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"github.com/streamingfast/firehose-stellar/types"
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

	logger *zap.Logger
}

func NewFetcher(fetchInterval, latestBlockRetryInterval time.Duration, logger *zap.Logger) *Fetcher {
	return &Fetcher{
		fetchInterval:            fetchInterval,
		latestBlockRetryInterval: latestBlockRetryInterval,
		lastBlockInfo:            NewLastBlockInfo(),
		decoder:                  decoder.NewDecoder(logger),
		logger:                   logger,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, client *Client, requestBlockNum uint64) (b *pbbstream.Block, skipped bool, err error) {
	sleepDuration := time.Duration(0)
	for f.lastBlockInfo.blockNum < requestBlockNum {
		time.Sleep(sleepDuration)

		latestLedger, err := client.GetLatestLedger()
		if err != nil {
			return nil, false, fmt.Errorf("fetching latest block num: %w", err)
		}

		f.lastBlockInfo.blockNum = uint64(latestLedger.Sequence)
		f.logger.Info("got latest block num", zap.Uint64("latest_block_num", f.lastBlockInfo.blockNum), zap.Uint64("requested_block_num", requestBlockNum))

		if f.lastBlockInfo.blockNum >= requestBlockNum {
			break
		}
		sleepDuration = f.latestBlockRetryInterval
	}

	ledger, err := client.GetLedgers(requestBlockNum)
	if err != nil {
		return nil, false, fmt.Errorf("fetching ledger: %w", err)
	}

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

	ledgerHeader := ledgerMetadata.V1.LedgerHeader

	// The number of transactions to fetch when calling the 'getTransactions' RPC method
	numOfTransactions := len(ledgerMetadata.V1.TxProcessing)
	lastCursor, transactions, err := client.GetTransactions(requestBlockNum, numOfTransactions, f.lastBlockInfo.cursor)
	if err != nil {
		return nil, false, fmt.Errorf("fetching transactions: %w", err)
	}

	if lastCursor != "" {
		f.lastBlockInfo.cursor = lastCursor
	}

	decodedTransactionMeta := make([]*types.TransactionMeta, 0)
	for _, trx := range transactions {
		trxMeta, err := f.decoder.DecodeTransactionResultMeta(trx.ResultMetaXdr)
		if err != nil {
			return nil, false, fmt.Errorf("decoding transaction result meta for trx %s: %w", trx.TxHash, err)
		}

		decodedTransactionMeta = append(decodedTransactionMeta, types.NewTransactionMeta(trx.TxHash, trxMeta))
	}

	// contractEvents := make([]*pbstellar.ContractEvent, 0)
	sorobanEvents := make([]*pbstellar.ContractEvent, 0)
	stellarTransactions := make([]*pbstellar.Transaction, 0)

	for _, trx := range decodedTransactionMeta {
		trxMeta, ok := trx.Meta.GetV3()
		if !ok {
			f.logger.Debug("transaction meta not v3", zap.String("tx_hash", trx.Hash))
			continue
		}

		if trxMeta.SorobanMeta != nil && trxMeta.SorobanMeta.Events != nil {
			for _, event := range trxMeta.SorobanMeta.Events {
				contractEvent := &pbstellar.ContractEvent{
					ContractId: event.ContractId.HexString(),
					Type:       pbstellar.FromXdrContactEventType(event.Type),
					// TODO: check how we want to add the eventBody? (raw bytes or all the data decoded)
				}
				sorobanEvents = append(sorobanEvents, contractEvent)
			}
		}

		stellarTransactions = append(stellarTransactions, &pbstellar.Transaction{
			Hash: trx.Hash,
		})

	}

	// TODO: once the transaction is decoded, find the number of events and call the
	// 	'getEvents' RPC method to fetch the events and then decode it

	// events, err := client.GetEvents(requestBlockNum)
	// if err != nil {
	// 	return nil, false, fmt.Errorf("fetching events: %w", err)
	// }

	stellarBlk := &pbstellar.Block{
		Number: ledger[0].Sequence,
		Hash:   ledger[0].Hash,
		Header: &pbstellar.Header{
			LedgerVersion:      uint32(ledgerHeader.Header.LedgerVersion),
			PreviousLedgerHash: ledgerHeader.Header.PreviousLedgerHash.HexString(),
			TotalCoins:         int64(ledgerHeader.Header.TotalCoins),
			BaseFee:            uint32(ledgerHeader.Header.BaseFee),
			BaseReserve:        uint32(ledgerHeader.Header.BaseReserve),
		},
		Transactions: stellarTransactions,
		Timestamp:    timestamppb.New(time.Unix(ledgerTime, 0)),
	}

	bstreamBlock, err := convertBlock(stellarBlk)
	if err != nil {
		return nil, false, fmt.Errorf("converting block: %w", err)
	}

	return bstreamBlock, false, nil
}

func (f *Fetcher) IsBlockAvailable(blockNum uint64) bool {
	return blockNum <= f.lastBlockInfo.blockNum
}

func convertBlock(stellarBlk *pbstellar.Block) (*pbbstream.Block, error) {
	anyBlock, err := anypb.New(stellarBlk)
	if err != nil {
		return nil, fmt.Errorf("unable to create anypb: %w", err)
	}
	fmt.Println("stellar block hash", stellarBlk.Hash)
	blk := &pbbstream.Block{
		Number:    stellarBlk.Number,
		Id:        stellarBlk.Hash,
		ParentId:  stellarBlk.Header.PreviousLedgerHash,
		Timestamp: timestamppb.New(stellarBlk.Timestamp.AsTime()),
		LibNum:    stellarBlk.Number - 1, // every block in stellar is final
		ParentNum: stellarBlk.Number - 1, // every block in stellar is final
		Payload:   anyBlock,
	}

	return blk, nil
}
