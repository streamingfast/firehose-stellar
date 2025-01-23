package main

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/bobg/go-generics/v3/slices"
	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/spf13/cobra"
	xdrTypes "github.com/stellar/go/xdr"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/firehose-core/types"
	"github.com/streamingfast/firehose-stellar/decoder"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewToolDecodeBlockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool-decode-block <store> <block-range>",
		Short: "Tool to decode a firehose block",
		Args:  cobra.ExactArgs(2),
		RunE:  runDecodeBlockE,
	}

	cmd.Flags().String("trx-hash", "", "Transaction hash to filter block on")

	return cmd
}

func runDecodeBlockE(cmd *cobra.Command, args []string) error {
	store, err := dstore.NewDBinStore(args[0])
	cli.NoError(err, "Unable to create store %q", args[0])

	blockRange := types.NewOpenRange(int64(bstream.GetProtocolFirstStreamableBlock))
	if len(args) > 1 {
		blockRange, err = types.GetBlockRangeFromArg(args[1])
		cli.NoError(err, "Unable to parse block range %q", args[1])

		// If the range is open, we assume the user wants to print a single block
		if blockRange.IsOpen() {
			blockRange = types.NewClosedRange(blockRange.GetStartBlock(), uint64(blockRange.GetStartBlock())+1)
		}

		cli.Ensure(blockRange.IsResolved(), "Invalid block range %q: print only accepts fully resolved range", blockRange)
	}

	options := []bstream.FileSourceOption{
		bstream.FileSourceErrorOnMissingMergedBlocksFile(),
	}
	if !blockRange.IsOpen() {
		options = append(options, bstream.FileSourceWithStopBlock(blockRange.MustGetStopBlock()))
	}

	trxHash := cmd.Flag("trx-hash").Value.String()

	source := bstream.NewFileSource(store, uint64(blockRange.GetStartBlock()), bstream.HandlerFunc(func(blk *pbbstream.Block, obj any) error {
		if !blockRange.Contains(blk.Number, types.RangeBoundaryExclusive) {
			return nil
		}

		msg, err := blk.Payload.UnmarshalNew()
		if err != nil {
			return fmt.Errorf("unmarshalling block payload: %w", err)
		}
		stellarBlock, ok := msg.(*pbstellar.Block)
		if !ok {
			return fmt.Errorf("unexpected type %T", msg)
		}

		if trxHash != "" {
			stellarBlock.Transactions = slices.Filter(stellarBlock.Transactions, func(tx *pbstellar.Transaction) bool {
				return hex.EncodeToString(tx.Hash) == trxHash
			})
		}

		out, err := json.Marshal(stellarBlock,
			json.WithMarshalers(json.JoinMarshalers(
				json.MarshalToFunc(marshalBytes),
				json.MarshalToFunc(marshalAccountId),
				json.MarshalToFunc(marshalTransaction),
				json.MarshalToFunc(marshalMuxedAccount),
			)),
			jsontext.WithIndent("  "),
		)
		if err != nil {
			return fmt.Errorf("unable to marshal: %w", err)
		}

		fmt.Println(string(out))

		return nil
	}), logger, options...)

	// Blocking call, perform the work we asked for
	source.Run()

	if source.Err() != nil && !errors.Is(source.Err(), bstream.ErrStopBlockReached) {
		return source.Err()
	}

	return nil
}

func marshalBytes(e *jsontext.Encoder, value []byte, options json.Options) error {
	return e.WriteToken(jsontext.String(hex.EncodeToString(value[:])))
}

func marshalAccountId(e *jsontext.Encoder, value xdrTypes.AccountId, options json.Options) error {
	return e.WriteToken(jsontext.String(value.Address()))
}

func marshalMuxedAccount(e *jsontext.Encoder, value xdrTypes.MuxedAccount, options json.Options) error {
	return e.WriteToken(jsontext.String(value.Address()))
}

func marshalTransaction(e *jsontext.Encoder, value *pbstellar.Transaction, options json.Options) error {
	decoder := decoder.NewDecoder(zap.NewNop())
	transactionMetadata, err := decoder.DecodeTransactionResultMetaFromBytes(value.ResultMetaXdr)
	if err != nil {
		return fmt.Errorf("unable to decode transaction meta: %w", err)
	}

	transactionEnvelope, err := decoder.DecodeTransactionEnvelopeFromBytes(value.EnvelopeXdr)
	if err != nil {
		return fmt.Errorf("unable to decode transaction meta: %w", err)
	}

	transactionResult, err := decoder.DecodeTransactionResultFromBytes(value.ResultXdr)
	if err != nil {
		return fmt.Errorf("unable to decode transaction meta: %w", err)
	}

	type DecodedTransaction struct {
		Hash             []byte
		Status           pbstellar.TransactionStatus
		CreatedAt        *timestamppb.Timestamp
		ApplicationOrder uint64
		EnvelopeXdr      *xdrTypes.TransactionEnvelope
		ResultMetaXdr    *xdrTypes.TransactionMeta
		ResultXdr        *xdrTypes.TransactionResult
	}

	// // FIXME: once the transaction hash is fixed, we can remove this
	// fixHash := base64.StdEncoding.EncodeToString(value.Hash)
	// fixHashB, err := hex.DecodeString(fixHash)
	// if err != nil {
	// 	return fmt.Errorf("unable to decode hash: %w", err)
	// }
	trx := &DecodedTransaction{
		Hash:             value.Hash,
		Status:           value.Status,
		CreatedAt:        value.CreatedAt,
		ApplicationOrder: value.ApplicationOrder,
		EnvelopeXdr:      transactionEnvelope,
		ResultMetaXdr:    transactionMetadata,
		ResultXdr:        transactionResult,
	}

	out, err := json.Marshal(trx,
		json.WithMarshalers(json.JoinMarshalers(
			json.MarshalToFunc(marshalBytes),
			json.MarshalToFunc(marshalAccountId),
			json.MarshalToFunc(marshalMuxedAccount),
		)),
	)
	if err != nil {
		return fmt.Errorf("unable to marshal: %w", err)
	}

	return e.WriteValue(out)
}

func printLedgerEntryChanges(changes []xdrTypes.LedgerEntryChange) {
	for _, change := range changes {
		fmt.Printf("LedgerEntryChange: %s\n", change.Type)

		if change.Created != nil {
			fmt.Printf("\tCreated %s\n", printLedgerEntry(change.Created))
		}

		if change.Updated != nil {
			fmt.Printf("\tUpdated %s\n", printLedgerEntry(change.Updated))
		}

		if change.Removed != nil {
			fmt.Printf("\tRemoved %s\n", printLedgerKey(change.Removed))
		}

		if change.State != nil {
			fmt.Printf("\tState %s\n", printLedgerEntry(change.State))
		}
	}
}

func printLedgerEntry(ledgerEntry *xdrTypes.LedgerEntry) string {
	return fmt.Sprintf("Type: %v Account: %v Balance: %v\n", ledgerEntry.Data.Type, ledgerEntry.Data.Account.AccountId.Address(), ledgerEntry.Data.Account.Balance)
}

func printLedgerKey(ledgerKey *xdrTypes.LedgerKey) string {
	return fmt.Sprintf("Type: %v Account: %v\n", ledgerKey.Type, ledgerKey.Account.AccountId.Address())
}
