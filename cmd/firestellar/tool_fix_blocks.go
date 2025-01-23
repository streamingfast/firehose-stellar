package main

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dstore"
	firecore "github.com/streamingfast/firehose-core"
	pbstellarv1 "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	pbstellarv1_old "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1_old"
	"github.com/streamingfast/firehose-stellar/utils"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewToolFixBlocksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool-fix-blocks <first-streamable-block> <stop-block> <src-store-url> <dst-store-url>",
		Short: "Tool to fix blocks",
		Args:  cobra.ExactArgs(4),
		RunE:  toolFixBlockRunE,
	}
	return cmd
}

func toolFixBlockRunE(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	firstStreamableBlock, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("converting first streamable block to uint64: %w", err)
	}

	stopBlock, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return fmt.Errorf("converting stop block to uint64: %w", err)
	}

	srcStore, err := dstore.NewDBinStore(args[2])
	if err != nil {
		return fmt.Errorf("unable to create source store: %w", err)
	}

	destStore, err := dstore.NewDBinStore(args[3])
	if err != nil {
		return fmt.Errorf("unable to create destination store: %w", err)
	}

	logger.Debug(
		"starting fix unknown type blocks",
		zap.String("source_blocks_store", srcStore.BaseURL().String()),
		zap.String("destination_blocks_store", destStore.BaseURL().String()),
		zap.Uint64("first_streamable_block", firstStreamableBlock),
		zap.Uint64("stop_block", stopBlock),
	)

	mergeWriter := &firecore.MergedBlocksWriter{
		Store:        destStore,
		LowBlockNum:  firstStreamableBlock,
		StopBlockNum: stopBlock,
		Logger:       logger,
		Cmd:          cmd,
	}
	var lastFilename string
	var blockCount = 0
	err = srcStore.WalkFrom(ctx, "", fmt.Sprintf("%010d", firstStreamableBlock), func(filename string) error {
		var fileReader io.Reader
		fileReader, err = srcStore.OpenObject(ctx, filename)
		if err != nil {
			return fmt.Errorf("creating reader: %w", err)
		}

		var blockReader *bstream.DBinBlockReader
		blockReader, err = bstream.NewDBinBlockReader(fileReader)
		if err != nil {
			return fmt.Errorf("creating block reader: %w", err)
		}

		// the source store is a merged file store
		for {
			currentBlock, err := blockReader.Read()
			if err != nil {
				if err == io.EOF {
					fmt.Fprintf(os.Stderr, "Total blocks: %d\n", blockCount)
					break
				}
				return fmt.Errorf("error receiving blocks: %w", err)
			}

			blockCount++

			stellarBlock := &pbstellarv1_old.Block{}
			err = proto.Unmarshal(currentBlock.Payload.Value, stellarBlock)
			if err != nil {
				return fmt.Errorf("unmarshaling block: %w", err)
			}

			convertedTransactions, err := convertOldTransactions(stellarBlock.Transactions)
			if err != nil {
				return fmt.Errorf("converting old transactions at %d: %w", currentBlock.Number, err)
			}

			fixedStellarBlock := &pbstellarv1.Block{
				Number:       stellarBlock.Number,
				Hash:         stellarBlock.Hash,
				Header:       convertOldHeader(stellarBlock.Header),
				Transactions: convertedTransactions,
				CreatedAt:    stellarBlock.CreatedAt,
			}

			payload, err := anypb.New(fixedStellarBlock)
			if err != nil {
				return fmt.Errorf("creating payload: %w", err)
			}

			currentBlock.Payload = payload
			if err = mergeWriter.ProcessBlock(currentBlock, nil); err != nil {
				return fmt.Errorf("processing block: %w", err)
			}
		}

		lastFilename = filename
		return nil
	})

	mergeWriter.Logger = mergeWriter.Logger.With(zap.String("last_filename", lastFilename), zap.Int("block_count", blockCount))
	if err != nil {
		if errors.Is(err, dstore.StopIteration) {
			err = mergeWriter.WriteBundle()
			if err != nil {
				return fmt.Errorf("writing bundle: %w", err)
			}
			fmt.Println("done")
			return nil
		}
		if errors.Is(err, io.EOF) {
			fmt.Println("done")
			return nil
		}
		return fmt.Errorf("walking source store: %w", err)
	}

	fmt.Println("Done fixing blocks")
	return nil
}

func convertOldHeader(oldHeader *pbstellarv1_old.Header) *pbstellarv1.Header {
	return &pbstellarv1.Header{
		LedgerVersion:      oldHeader.LedgerVersion,
		PreviousLedgerHash: oldHeader.PreviousLedgerHash,
		TotalCoins:         oldHeader.TotalCoins,
		BaseFee:            oldHeader.BaseFee,
		BaseReserve:        oldHeader.BaseReserve,
	}
}

func convertOldTransactions(oldTransactions []*pbstellarv1_old.Transaction) ([]*pbstellarv1.Transaction, error) {
	var transactions []*pbstellarv1.Transaction
	for _, oldTransaction := range oldTransactions {
		convertedTransaction, err := convertOldTransaction(oldTransaction)
		if err != nil {
			return nil, fmt.Errorf("converting transaction: %w", err)
		}
		transactions = append(transactions, convertedTransaction)
	}
	return transactions, nil
}

func convertOldTransaction(oldTransaction *pbstellarv1_old.Transaction) (*pbstellarv1.Transaction, error) {
	fixHash := base64.StdEncoding.EncodeToString(oldTransaction.Hash)
	txHashbytes, err := hex.DecodeString(fixHash)
	if err != nil {
		return nil, fmt.Errorf("decoding transaction hash: %w", err)
	}
	return &pbstellarv1.Transaction{
		Hash:             txHashbytes,
		Status:           utils.ConvertTransactionStatus(oldTransaction.Status),
		CreatedAt:        oldTransaction.CreatedAt,
		ApplicationOrder: oldTransaction.ApplicationOrder,
		EnvelopeXdr:      oldTransaction.EnvelopeXdr,
		ResultMetaXdr:    oldTransaction.ResultMetaXdr,
		ResultXdr:        oldTransaction.ResultXdr,
	}, nil
}
