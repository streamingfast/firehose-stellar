package rpc

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

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
	clientLimit              int

	logger *zap.Logger
}

func NewFetcher(fetchInterval, latestBlockRetryInterval time.Duration, clientLimit int, logger *zap.Logger) *Fetcher {
	return &Fetcher{
		fetchInterval:            fetchInterval,
		latestBlockRetryInterval: latestBlockRetryInterval,
		lastBlockInfo:            NewLastBlockInfo(),
		decoder:                  decoder.NewDecoder(logger),
		clientLimit:              clientLimit,
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

	numOfTransactions := len(ledgerMetadata.V1.TxProcessing)
	f.logger.Debug("fetching transactions", zap.Uint64("block_num", requestBlockNum), zap.Int("num_of_transactions", numOfTransactions))
	if numOfTransactions > f.clientLimit {
		// There is a hard limit on the number of transactions
		// to fetch. The RPC providers tipically set the maximum limit to 200.
		numOfTransactions = f.clientLimit
	}
	transactions, err := client.GetTransactions(requestBlockNum, numOfTransactions, f.lastBlockInfo.cursor)
	if err != nil {
		return nil, false, fmt.Errorf("fetching transactions: %w", err)
	}

	transactionMeta := make([]*types.TransactionMeta, 0)
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
		txResultMetaBytes, err := base64.StdEncoding.DecodeString(trx.ResultMetaXdr)
		if err != nil {
			return nil, false, fmt.Errorf("decoding transaction result meta: %w", err)
		}
		transactionMeta = append(transactionMeta, types.NewTransactionMeta(txHashBytes, trx.Status, txEnvelopeBytes, txResultBytes, txResultMetaBytes))
	}

	stellarTransactions := make([]*pbstellar.Transaction, 0)
	for i, trx := range transactionMeta {
		stellarTransactions = append(stellarTransactions, &pbstellar.Transaction{
			Hash:             trx.Hash,
			Status:           utils.ConvertTransactionStatus(trx.Status),
			CreatedAt:        timestamppb.New(time.Unix(ledgerTime, 0)),
			ApplicationOrder: uint64(i + 1),
			EnvelopeXdr:      trx.EnveloppeXdr,
			ResultXdr:        trx.ResultXdr,
			ResultMetaXdr:    trx.ResultMetaXdr,
		})
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
		Transactions: stellarTransactions,
		CreatedAt:    timestamppb.New(time.Unix(ledgerTime, 0)),
	}

	bstreamBlock, err := convertBlock(stellarBlk)
	if err != nil {
		return nil, false, fmt.Errorf("converting block: %w", err)
	}

	// reset the cursor
	f.lastBlockInfo.cursor = ""

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

	stellarBlockHash := base64.StdEncoding.EncodeToString(stellarBlk.Hash)
	previousStellarBlockHash := base64.StdEncoding.EncodeToString(stellarBlk.Header.PreviousLedgerHash)

	return &pbbstream.Block{
		Number:    stellarBlk.Number,
		Id:        stellarBlockHash,
		ParentId:  previousStellarBlockHash,
		Timestamp: timestamppb.New(stellarBlk.CreatedAt.AsTime()),
		LibNum:    stellarBlk.Number - 1, // every block in stellar is final
		ParentNum: stellarBlk.Number - 1, // every block in stellar is final
		Payload:   anyBlock,
	}, nil
}
