// Package captivecore exposes the captive-core block fetcher as a library.
//
// Two layers:
//
//  1. Fetcher — converter from xdr.LedgerCloseMeta to pbbstream.Block.
//     Stateless apart from network passphrase + logger.
//
//  2. Backend — wraps *ledgerbackend.CaptiveStellarCore (the stellar-core
//     subprocess) and offers PrepareRange / GetBlock / Close.
package captivecore

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/xdr"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"github.com/streamingfast/firehose-stellar/types"
	"github.com/streamingfast/firehose-stellar/utils"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Config is the parameter set for a captive-core backend. Use
// ResolveNetwork to fill defaults for mainnet/testnet.
type Config struct {
	// BinaryPath to the stellar-core executable. Required.
	BinaryPath string

	// NetworkPassphrase the chain uses (e.g. "Public Global Stellar
	// Network ; September 2015"). Required.
	NetworkPassphrase string

	// HistoryArchiveURLs stellar-core pulls from to catch up. At least
	// one required.
	HistoryArchiveURLs []string

	// StellarCoreConfPath to a stellar-core.cfg on disk. If empty,
	// DefaultTomlData must be set.
	StellarCoreConfPath string

	// DefaultTomlData is the bundled config bytes for the chosen
	// network (PubnetDefaultConfig / TestnetDefaultConfig from
	// stellar/go SDK). Only used when StellarCoreConfPath is empty.
	DefaultTomlData []byte

	// StoragePath is the working directory stellar-core uses for its
	// sqlite db, buckets, and tmp files. Defaults to a fresh temp dir
	// created at New() time when empty.
	StoragePath string

	// LogLevel for the stellar-core subprocess log stream. Defaults to
	// logrus.InfoLevel.
	LogLevel logrus.Level

	// LogOutput is where stellar-core's logrus stream writes. When nil,
	// the SDK default (stderr) is used. Set this to a file handle to
	// redirect the spammy stellar-core output away from the terminal.
	LogOutput io.Writer

	// Logger receives high-level fetcher events. Required.
	Logger *zap.Logger
}

// ResolveNetwork fills NetworkPassphrase, HistoryArchiveURLs, and
// DefaultTomlData based on the logical network name (mainnet|testnet|
// custom). Values already set on Config are preserved.
func (c *Config) ResolveNetwork(name string) error {
	switch name {
	case "mainnet":
		if c.NetworkPassphrase == "" {
			c.NetworkPassphrase = network.PublicNetworkPassphrase
		}
		if len(c.HistoryArchiveURLs) == 0 {
			c.HistoryArchiveURLs = network.PublicNetworkhistoryArchiveURLs
		}
		if c.DefaultTomlData == nil {
			c.DefaultTomlData = ledgerbackend.PubnetDefaultConfig
		}
	case "testnet":
		if c.NetworkPassphrase == "" {
			c.NetworkPassphrase = network.TestNetworkPassphrase
		}
		if len(c.HistoryArchiveURLs) == 0 {
			c.HistoryArchiveURLs = network.TestNetworkhistoryArchiveURLs
		}
		if c.DefaultTomlData == nil {
			c.DefaultTomlData = ledgerbackend.TestnetDefaultConfig
		}
	case "custom":
		// Passphrase + archive URLs must come from caller. No defaults.
	default:
		return fmt.Errorf("unsupported stellar network: %s (want mainnet|testnet|custom)", name)
	}
	return nil
}

// validate checks required fields. Called from New().
func (c *Config) validate() error {
	if c.BinaryPath == "" {
		return errors.New("captivecore: BinaryPath is required")
	}
	if c.NetworkPassphrase == "" {
		return errors.New("captivecore: NetworkPassphrase is required")
	}
	if len(c.HistoryArchiveURLs) == 0 {
		return errors.New("captivecore: HistoryArchiveURLs is required (at least one)")
	}
	if c.StellarCoreConfPath == "" && c.DefaultTomlData == nil {
		return errors.New("captivecore: either StellarCoreConfPath or DefaultTomlData must be set")
	}
	if c.Logger == nil {
		return errors.New("captivecore: Logger is required")
	}
	return nil
}

// Backend drives a stellar-core subprocess and converts each fetched
// ledger to pbbstream.Block via the embedded Fetcher.
type Backend struct {
	core    *ledgerbackend.CaptiveStellarCore
	fetcher *Fetcher
	logger  *zap.Logger
}

// New constructs a Backend. The stellar-core subprocess starts on the
// first PrepareRange call. Always defer Close.
func New(cfg Config) (*Backend, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Match what soroban-rpc emits so blocks are byte-equivalent across
	// both fetchers. All three flags require stellar-core protocol >= 23:
	//   - EmitUnifiedEvents: TransactionMetaV4 (CAP-67) — populates
	//     classic-tx transaction-events + per-op contract-events.
	//   - EnforceSorobanDiagnosticEvents: forces ENABLE_SOROBAN_DIAGNOSTIC_EVENTS.
	//   - EnforceSorobanTransactionMetaExtV1: extra Soroban meta ext.
	params := ledgerbackend.CaptiveCoreTomlParams{
		NetworkPassphrase:                  cfg.NetworkPassphrase,
		HistoryArchiveURLs:                 cfg.HistoryArchiveURLs,
		CoreBinaryPath:                     cfg.BinaryPath,
		EmitUnifiedEvents:                  true,
		EnforceSorobanDiagnosticEvents:     true,
		EnforceSorobanTransactionMetaExtV1: true,
	}

	var toml *ledgerbackend.CaptiveCoreToml
	if cfg.StellarCoreConfPath != "" {
		t, err := ledgerbackend.NewCaptiveCoreTomlFromFile(cfg.StellarCoreConfPath, params)
		if err != nil {
			return nil, fmt.Errorf("captivecore: setting up toml from file %s: %w", cfg.StellarCoreConfPath, err)
		}
		toml = t
	} else {
		t, err := ledgerbackend.NewCaptiveCoreTomlFromData(cfg.DefaultTomlData, params)
		if err != nil {
			return nil, fmt.Errorf("captivecore: setting up toml from default: %w", err)
		}
		toml = t
	}

	coreLogger := log.New()
	level := cfg.LogLevel
	if level == 0 {
		level = logrus.InfoLevel
	}
	coreLogger.SetLevel(level)
	if cfg.LogOutput != nil {
		coreLogger.SetOutput(cfg.LogOutput)
	}

	core, err := ledgerbackend.NewCaptive(ledgerbackend.CaptiveCoreConfig{
		BinaryPath:         cfg.BinaryPath,
		NetworkPassphrase:  cfg.NetworkPassphrase,
		HistoryArchiveURLs: cfg.HistoryArchiveURLs,
		StoragePath:        cfg.StoragePath,
		Toml:               toml,
		Log:                coreLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("captivecore: setting up captive-core backend: %w", err)
	}

	return &Backend{
		core:    core,
		fetcher: &Fetcher{NetworkPassphrase: cfg.NetworkPassphrase, Logger: cfg.Logger},
		logger:  cfg.Logger,
	}, nil
}

// PrepareRange launches stellar-core and catches up to startLedger.
// Subsequent GetBlock calls expect ledgers in [startLedger, ∞). Stays
// open until Close.
func (b *Backend) PrepareRange(ctx context.Context, startLedger uint64) error {
	if startLedger < 1 {
		return fmt.Errorf("captivecore: start ledger must be >= 1 (stellar ledger sequences start at 1)")
	}
	if startLedger > math.MaxUint32 {
		return fmt.Errorf("captivecore: start ledger %d exceeds stellar ledger sequence range (uint32)", startLedger)
	}
	b.logger.Info("captivecore preparing range", zap.Uint64("start_block", startLedger))
	if err := b.core.PrepareRange(ctx, ledgerbackend.UnboundedRange(uint32(startLedger))); err != nil {
		return fmt.Errorf("captivecore: prepare range from %d: %w", startLedger, err)
	}
	b.logger.Info("captivecore range prepared")
	return nil
}

// GetBlock returns one ledger as pbbstream.Block. Blocks until the
// ledger is available or ctx fires.
func (b *Backend) GetBlock(ctx context.Context, ledgerSeq uint64) (*pbbstream.Block, error) {
	if ledgerSeq > math.MaxUint32 {
		return nil, fmt.Errorf("captivecore: ledger %d exceeds uint32", ledgerSeq)
	}
	meta, err := b.core.GetLedger(ctx, uint32(ledgerSeq))
	if err != nil {
		return nil, fmt.Errorf("captivecore: get ledger %d: %w", ledgerSeq, err)
	}
	blk, err := b.fetcher.ConvertLedgerCloseMetaToBstreamBlock(&meta)
	if err != nil {
		return nil, fmt.Errorf("captivecore: convert ledger %d: %w", ledgerSeq, err)
	}
	return blk, nil
}

// Close terminates the stellar-core subprocess. Idempotent.
func (b *Backend) Close() error {
	if b.core == nil {
		return nil
	}
	err := b.core.Close()
	b.core = nil
	return err
}

// Fetcher converts xdr.LedgerCloseMeta to the pbbstream.Block shape the
// RPC fetcher emits. NetworkPassphrase is used to recompute tx hashes.
type Fetcher struct {
	NetworkPassphrase string
	Logger            *zap.Logger
}

// ConvertLedgerCloseMetaToBstreamBlock converts one ledger to a
// pbbstream.Block.
func (f *Fetcher) ConvertLedgerCloseMetaToBstreamBlock(ledgerMetadata *xdr.LedgerCloseMeta) (*pbbstream.Block, error) {
	var ledgerHeader xdr.LedgerHeaderHistoryEntry
	var ledgerSeq uint32
	var ledgerHash xdr.Hash

	switch {
	case ledgerMetadata.V0 != nil:
		ledgerHeader = ledgerMetadata.V0.LedgerHeader
		ledgerSeq = uint32(ledgerHeader.Header.LedgerSeq)
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

func (f *Fetcher) extractTransactionsFromLedgerMetadata(ledgerMetadata *xdr.LedgerCloseMeta) ([]types.Transaction, error) {
	reader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(f.NetworkPassphrase, *ledgerMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create ledger transaction reader: %w", err)
	}
	defer reader.Close()

	transactions := make([]types.Transaction, 0)
	for {
		tx, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
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

func (f *Fetcher) convertLedgerTransactionToTypes(tx ingest.LedgerTransaction) (*types.Transaction, error) {
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
		f.Logger.Warn("failed to convert events", zap.Error(err))
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

func (f *Fetcher) convertStellarBlockToBstreamBlock(stellarBlk *pbstellar.Block) (*pbbstream.Block, error) {
	anyBlock, err := anypb.New(stellarBlk)
	if err != nil {
		return nil, fmt.Errorf("unable to create anypb: %w", err)
	}

	// Hex-encode Id / ParentId so they're safe for one-block-file names
	// (firecore mindreader splits on '/'). pbstellar.Block.Hash stays raw.
	stellarBlockHash := hex.EncodeToString(stellarBlk.Hash)
	previousStellarBlockHash := hex.EncodeToString(stellarBlk.Header.PreviousLedgerHash)

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
