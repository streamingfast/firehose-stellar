package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/xdr"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli/sflags"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-core/blockpoller"
	"github.com/streamingfast/firehose-stellar/decoder"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"github.com/streamingfast/firehose-stellar/types"
	"github.com/streamingfast/firehose-stellar/utils"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewFetchCaptiveCoreCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "captive-core <first-streamable-block>",
		Short: "fetch blocks from stellar captive core",
		Args:  cobra.ExactArgs(1),
		RunE:  fetchCaptiveCoreRunE(logger, tracer),
	}

	cmd.Flags().String("state-dir", "/data/poller", "interval between fetch")
	cmd.Flags().String("stellar-core-bin", "/usr/bin/stellar-core", "path to stellar-core binary")
	cmd.Flags().String("stellar-core-conf", "", "path to stellar-core config file")
	cmd.Flags().String("stellar-core-network", "testnet", "stellar network (mainnet or testnet)")

	return cmd
}

func fetchCaptiveCoreRunE(logger *zap.Logger, tracer logging.Tracer) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) (err error) {
		//stateDir := sflags.MustGetString(cmd, "state-dir")

		startBlock, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse first streamable block %s: %w", args[0], err)
		}

		stellarCoreBin := sflags.MustGetString(cmd, "stellar-core-bin")
		stellarCoreNetwork := sflags.MustGetString(cmd, "stellar-core-network")

		var archiveURLs []string
		var networkPassphrase string
		isMainnet := false

		switch stellarCoreNetwork {
		case "mainnet":
			archiveURLs = network.PublicNetworkhistoryArchiveURLs
			networkPassphrase = network.PublicNetworkPassphrase
			isMainnet = true
		case "testnet":
			archiveURLs = network.TestNetworkhistoryArchiveURLs
			networkPassphrase = network.TestNetworkPassphrase
		default:
			return fmt.Errorf("unsupported stellar network: %s", stellarCoreNetwork)
		}

		var captiveCoreToml *ledgerbackend.CaptiveCoreToml

		params := ledgerbackend.CaptiveCoreTomlParams{
			NetworkPassphrase:  networkPassphrase,
			HistoryArchiveURLs: archiveURLs,
			CoreBinaryPath:     stellarCoreBin,
		}

		stellarCoreConf := sflags.MustGetString(cmd, "stellar-core-conf")
		if stellarCoreConf != "" {
			var err error
			captiveCoreToml, err = ledgerbackend.NewCaptiveCoreTomlFromFile(stellarCoreConf, params)
			if err != nil {
				return fmt.Errorf("setting up captive core toml from file %s: %w", stellarCoreConf, err)
			}
		} else {
			// If no config file is provided, use the default for the network
			var defaultData []byte
			if isMainnet {
				defaultData = ledgerbackend.PubnetDefaultConfig
			} else {
				defaultData = ledgerbackend.TestnetDefaultConfig
			}
			var err error
			captiveCoreToml, err = ledgerbackend.NewCaptiveCoreTomlFromData(defaultData, params)
			if err != nil {
				return fmt.Errorf("setting up captive core toml from default: %w", err)
			}
		}

		//log := logrus.New()
		//log.Level = logrus.WarnLevel
		//log.Formatter.(*logrus.TextFormatter).FullTimestamp = true
		//logger := logrus.NewEntry(log).WithField("pid", os.Getpid())
		//

		captiveCoreLogger := log.DefaultLogger
		captiveCoreLogger.SetLevel(log.DebugLevel)
		config := ledgerbackend.CaptiveCoreConfig{
			BinaryPath:         stellarCoreBin,
			NetworkPassphrase:  networkPassphrase,
			HistoryArchiveURLs: archiveURLs,
			Toml:               captiveCoreToml,
			Log:                captiveCoreLogger,
		}

		backend, err := ledgerbackend.NewCaptive(config)
		if err != nil {
			return fmt.Errorf("setting up captive core backend: %w", err)
		}

		fetcher := &CaptiveCoreFetcher{
			backend:                  backend,
			fetchInterval:            0,
			latestBlockRetryInterval: 0,
			isMainnet:                isMainnet,
			logger:                   logger,
			decoder:                  decoder.NewDecoder(logger),
		}

		handler := blockpoller.NewFireBlockHandler("type.googleapis.com/sf.stellar.type.v1.Block")
		handler.Init()

		ctx := context.Background()
		logger.Info("preparing range", zap.Uint32("start_block", uint32(startBlock)))
		if err := backend.PrepareRange(ctx, ledgerbackend.UnboundedRange(uint32(startBlock))); err != nil {
			return fmt.Errorf("prepare range from %d: %w", startBlock, err)
		}
		logger.Info("range prepared")

		seq := uint32(startBlock)
		for {
			logger.Info("fetching block", zap.Uint32("seq", seq))
			meta, err := backend.GetLedger(ctx, seq)
			if err != nil {
				return fmt.Errorf("get ledger %d: %w", seq, err)
			}

			logger.Info("block fetched", zap.Uint32("seq", seq))

			blk, err := fetcher.convertLedgerCloseMetaToBstreamBlock(&meta)
			if err != nil {
				return fmt.Errorf("convert ledger %d: %w", seq, err)
			}

			if err := handler.Handle(blk); err != nil {
				return fmt.Errorf("handling block %d: %w", blk.Number, err)
			}

			seq++
		}

		// unreachable
	}
}

// CaptiveCoreFetcher converts `LedgerCloseMeta` to `pbbstream.Block` and exposes helpers
// It is not plugged into blockpoller, we use it manually with Captive Core.
type CaptiveCoreFetcher struct {
	backend                  ledgerbackend.LedgerBackend
	fetchInterval            time.Duration
	latestBlockRetryInterval time.Duration
	isMainnet                bool
	logger                   *zap.Logger
	decoder                  *decoder.Decoder

	lastLatestBlockNum uint64
}

func (f *CaptiveCoreFetcher) Fetch(ctx context.Context, client ledgerbackend.LedgerBackend, requestBlockNum uint64) (b *pbbstream.Block, skipped bool, err error) {
	requestBlockNum32 := uint32(requestBlockNum)

	for {
		latest, err := client.GetLatestLedgerSequence(ctx)
		if err != nil {
			return nil, false, fmt.Errorf("getting latest ledger sequence: %w", err)
		}

		if latest >= requestBlockNum32 {
			break
		}

		f.logger.Debug("waiting for block", zap.Uint64("request_block_num", requestBlockNum), zap.Uint32("latest_block_num", latest))
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-time.After(f.latestBlockRetryInterval):
		}
	}

	ledgerMetadata, err := client.GetLedger(ctx, requestBlockNum32)
	if err != nil {
		return nil, false, fmt.Errorf("getting ledger %d: %w", requestBlockNum, err)
	}

	bstreamBlock, err := f.convertLedgerCloseMetaToBstreamBlock(&ledgerMetadata)
	if err != nil {
		return nil, false, fmt.Errorf("converting ledger %d: %w", requestBlockNum, err)
	}

	if f.fetchInterval > 0 {
		time.Sleep(f.fetchInterval)
	}

	return bstreamBlock, false, nil
}

func (f *CaptiveCoreFetcher) IsBlockAvailable(blockNum uint64) bool {
	// For captive core, we usually just try to fetch it.
	// But we can check against last seen latest.
	return blockNum <= f.lastLatestBlockNum
}

func (f *CaptiveCoreFetcher) convertLedgerCloseMetaToBstreamBlock(ledgerMetadata *xdr.LedgerCloseMeta) (*pbbstream.Block, error) {
	var ledgerHeader xdr.LedgerHeaderHistoryEntry
	var ledgerSeq uint32
	var ledgerHash xdr.Hash

	switch {
	case ledgerMetadata.V0 != nil:
		ledgerHeader = ledgerMetadata.V0.LedgerHeader
		ledgerSeq = uint32(ledgerHeader.Header.LedgerSeq)
		// V0 doesn't have the hash directly in LedgerCloseMeta, but we can compute it if needed or get it from Header
		// Actually LedgerHeaderHistoryEntry HAS the hash in a way? No, LedgerHeaderHistoryEntry is:
		// struct LedgerHeaderHistoryEntry
		// {
		//     Hash hash;
		//     LedgerHeader header;
		//     // reserved for future use
		//     union switch (int v)
		//     {
		//     case 0:
		//         void;
		//     }
		//     ext;
		// };
		ledgerHash = ledgerMetadata.V0.LedgerHeader.Hash
	case ledgerMetadata.V1 != nil:
		ledgerHeader = ledgerMetadata.V1.LedgerHeader
		ledgerSeq = uint32(ledgerHeader.Header.LedgerSeq)
		ledgerHash = ledgerMetadata.V1.LedgerHeader.Hash
	case ledgerMetadata.V2 != nil:
		ledgerHeader = ledgerMetadata.V2.LedgerHeader
		ledgerSeq = uint32(ledgerHeader.Header.LedgerSeq)
		ledgerHash = ledgerMetadata.V2.LedgerHeader.Hash
	default:
		return nil, fmt.Errorf("unsupported LedgerCloseMeta version")
	}

	ledgerCloseTime := int64(ledgerHeader.Header.ScpValue.CloseTime)

	transactions, err := f.extractTransactionsFromLedgerMetadata(ledgerMetadata)
	if err != nil {
		return nil, fmt.Errorf("extracting transactions: %w", err)
	}

	stellarTransactions := make([]*pbstellar.Transaction, 0)
	for i, trx := range transactions {
		txHashBytes, err := hex.DecodeString(trx.TxHash)
		if err != nil {
			return nil, fmt.Errorf("decoding tx hash %s: %w", trx.TxHash, err)
		}
		envelopeXdr, err := base64.StdEncoding.DecodeString(trx.EnvelopeXdr)
		if err != nil {
			return nil, fmt.Errorf("decoding envelope XDR: %w", err)
		}
		resultXdr, err := base64.StdEncoding.DecodeString(trx.ResultXdr)
		if err != nil {
			return nil, fmt.Errorf("decoding result XDR: %w", err)
		}

		events := &pbstellar.Events{}
		if trx.Events != nil {
			diagnosticEvents := make([][]byte, 0)
			for _, event := range trx.Events.DiagnosticEventsXdr {
				decodedEvent, err := base64.StdEncoding.DecodeString(event)
				if err != nil {
					return nil, fmt.Errorf("decoding diagnostic event: %w", err)
				}
				diagnosticEvents = append(diagnosticEvents, decodedEvent)
			}

			transactionsEvents := make([][]byte, 0)
			for _, event := range trx.Events.TransactionEventsXdr {
				decodedEvent, err := base64.StdEncoding.DecodeString(event)
				if err != nil {
					return nil, fmt.Errorf("decoding transaction event: %w", err)
				}
				transactionsEvents = append(transactionsEvents, decodedEvent)
			}

			contractEvents := make([]*pbstellar.ContractEvent, 0)
			for _, eventsGroup := range trx.Events.ContractEventsXdr {
				innerContractEvents := make([][]byte, 0)
				for _, event := range eventsGroup {
					decodedEvent, err := base64.StdEncoding.DecodeString(event)
					if err != nil {
						return nil, fmt.Errorf("decoding contract event: %w", err)
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

		stellarTransactions = append(stellarTransactions, &pbstellar.Transaction{
			Hash:             txHashBytes,
			Status:           utils.ConvertTransactionStatus(trx.Status),
			CreatedAt:        timestamppb.New(time.Unix(ledgerCloseTime, 0)),
			ApplicationOrder: uint64(i + 1),
			EnvelopeXdr:      envelopeXdr,
			ResultXdr:        resultXdr,
			Events:           events,
		})
	}

	previousLedgerHash := ledgerHeader.Header.PreviousLedgerHash[:]

	stellarBlk := &pbstellar.Block{
		Number: uint64(ledgerSeq),
		Hash:   ledgerHash[:],
		Header: &pbstellar.Header{
			LedgerVersion:      uint32(ledgerHeader.Header.LedgerVersion),
			PreviousLedgerHash: previousLedgerHash,
			TotalCoins:         int64(ledgerHeader.Header.TotalCoins),
			BaseFee:            uint32(ledgerHeader.Header.BaseFee),
			BaseReserve:        uint32(ledgerHeader.Header.BaseReserve),
		},
		Version:      1,
		Transactions: stellarTransactions,
		CreatedAt:    timestamppb.New(time.Unix(ledgerCloseTime, 0)),
	}

	return f.convertStellarBlockToBstreamBlock(stellarBlk)
}

func (f *CaptiveCoreFetcher) extractTransactionsFromLedgerMetadata(ledgerMetadata *xdr.LedgerCloseMeta) ([]types.Transaction, error) {
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

		transaction, err := f.convertLedgerTransactionToTypes(tx)
		if err != nil {
			return nil, fmt.Errorf("failed to convert transaction: %w", err)
		}

		transactions = append(transactions, *transaction)
	}

	return transactions, nil
}

func (f *CaptiveCoreFetcher) convertLedgerTransactionToTypes(tx ingest.LedgerTransaction) (*types.Transaction, error) {
	envelopeXdr, err := tx.Envelope.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal envelope: %w", err)
	}
	envelopeXdrStr := base64.StdEncoding.EncodeToString(envelopeXdr)

	resultXdr, err := tx.Result.Result.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	resultXdrStr := base64.StdEncoding.EncodeToString(resultXdr)

	txHash := tx.Result.TransactionHash.HexString()

	status := "UNKNOWN"
	if tx.Successful() {
		status = "SUCCESS"
	} else {
		status = "FAILED"
	}

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

func (f *CaptiveCoreFetcher) convertLedgerTransactionEventsToRPCEvents(tx ingest.LedgerTransaction) (*types.RPCEvents, error) {
	rpcEvents := &types.RPCEvents{
		DiagnosticEventsXdr:  make([]string, 0),
		TransactionEventsXdr: make([]string, 0),
		ContractEventsXdr:    make([][]string, 0),
	}

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
		rpcEvents.ContractEventsXdr = append(rpcEvents.ContractEventsXdr, operationEventStrings)
	}

	return rpcEvents, nil
}

func (f *CaptiveCoreFetcher) convertStellarBlockToBstreamBlock(stellarBlk *pbstellar.Block) (*pbbstream.Block, error) {
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
		LibNum:    stellarBlk.Number - 1,
		ParentNum: stellarBlk.Number - 1,
		Payload:   anyBlock,
	}, nil
}
