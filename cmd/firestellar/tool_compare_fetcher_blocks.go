package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	jd "github.com/josephburnett/jd/lib"
	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/dstore"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"github.com/streamingfast/firehose-stellar/rpc"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func NewToolCompareFetcherBlocksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool-compare-fetcher-blocks <block_num> <gs_store_url> <rpc_endpoint>",
		Short: "Compare a block fetched via Fetcher with the same block from Google Storage",
		Long: `This tool fetches a block using the Fetcher.Fetch method and compares it with
the same block loaded from Google Storage. This is useful for validating that the
optimized Fetcher produces the same results as the stored blocks.

The tool automatically handles both one-block files and merged block files from Google Storage.
Differences are reported with both transaction hash and index for easy identification.

Arguments:
  block_num     The block number to compare
  gs_store_url  Google Storage URL for the block store (e.g., gs://dfuseio-global-blocks-uscentral/stellar-mainnet/v1)
  rpc_endpoint  RPC endpoint URL to use for fetching the block

Flags:
  --detailed-diff    Show detailed JSON diff for differing transactions (can be verbose)`,
		Args:    cobra.ExactArgs(3),
		RunE:    runCompareFetcherBlocksE,
		Example: `firestellar tool-compare-fetcher-blocks 60132634 gs://dfuseio-global-blocks-uscentral/stellar-mainnet/v1 https://soroban-mainnet.stellar.org`,
	}

	cmd.Flags().String("network", "mainnet", "Network to use (testnet or mainnet)")
	cmd.Flags().Bool("detailed-diff", false, "Show detailed JSON diff for differing transactions")

	return cmd
}

func runCompareFetcherBlocksE(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	logger := zap.NewNop() // Use a no-op logger for this tool

	blockNumStr := args[0]
	gsStoreURL := args[1]
	rpcEndpoint := args[2]

	blockNum, err := strconv.ParseUint(blockNumStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid block number %q: %w", blockNumStr, err)
	}

	network, _ := cmd.Flags().GetString("network")
	detailedDiff, _ := cmd.Flags().GetBool("detailed-diff")

	fmt.Printf("Comparing block %d\n", blockNum)
	fmt.Printf("RPC Endpoint: %s\n", rpcEndpoint)
	fmt.Printf("GS Store URL: %s\n", gsStoreURL)
	fmt.Printf("Network: %s\n", network)
	fmt.Printf("Detailed diff: %t\n", detailedDiff)

	// Step 1: Fetch block using Fetcher
	fmt.Println("\n=== Fetching block via Fetcher ===")
	fetcherBlock, err := fetchBlockViaFetcher(ctx, blockNum, rpcEndpoint, network, logger)
	if err != nil {
		return fmt.Errorf("failed to fetch block via Fetcher: %w", err)
	}

	// Unmarshal the payload to get the stellar block
	var fetcherStellarBlock pbstellar.Block
	if err := fetcherBlock.Payload.UnmarshalTo(&fetcherStellarBlock); err != nil {
		return fmt.Errorf("failed to unmarshal fetcher block payload: %w", err)
	}
	fmt.Printf("Fetched block %d with %d transactions\n", fetcherBlock.Number, len(fetcherStellarBlock.Transactions))

	// Step 2: Load block from Google Storage
	fmt.Println("\n=== Loading block from Google Storage ===")
	gsBlock, err := loadBlockFromGS(ctx, blockNum, gsStoreURL)
	if err != nil {
		return fmt.Errorf("failed to load block from GS: %w", err)
	}

	// Unmarshal the payload to get the stellar block
	var gsStellarBlock pbstellar.Block
	if err := gsBlock.Payload.UnmarshalTo(&gsStellarBlock); err != nil {
		return fmt.Errorf("failed to unmarshal GS block payload: %w", err)
	}
	fmt.Printf("Loaded block %d with %d transactions\n", gsBlock.Number, len(gsStellarBlock.Transactions))

	// Step 3: Compare the blocks
	fmt.Println("\n=== Comparing blocks ===")
	differences := compareBlocks(fetcherBlock, gsBlock, detailedDiff, rpcEndpoint)

	if len(differences) == 0 {
		fmt.Println("✅ Blocks are identical!")
		return nil
	}

	fmt.Printf("❌ Found %d differences:\n", len(differences))
	for _, diff := range differences {
		fmt.Printf("  - %s\n", diff)
	}

	return nil
}

func fetchBlockViaFetcher(ctx context.Context, blockNum uint64, rpcEndpoint, network string, logger *zap.Logger) (*pbbstream.Block, error) {
	// Create a client
	client := rpc.NewClient(rpcEndpoint, logger, nil)

	// Create a Fetcher instance
	fetcher := rpc.NewFetcher(0, time.Second, 200, logger)

	// Fetch the block
	block, skipped, err := fetcher.Fetch(ctx, client, blockNum)
	if err != nil {
		return nil, fmt.Errorf("fetching block: %w", err)
	}
	if skipped {
		return nil, fmt.Errorf("block %d was skipped", blockNum)
	}

	return block, nil
}

func loadBlockFromGS(ctx context.Context, blockNum uint64, storeURL string) (*pbbstream.Block, error) {
	// Create dstore from URL
	store, err := dstore.NewDBinStore(storeURL)
	if err != nil {
		return nil, fmt.Errorf("creating store: %w", err)
	}

	// First try to find one-block files
	files, err := findOneBlockFiles(ctx, store, blockNum)
	if err != nil {
		return nil, fmt.Errorf("finding one-block files: %w", err)
	}

	if len(files) > 0 {
		// Use one-block file approach
		filepath := files[0]
		reader, err := store.OpenObject(ctx, filepath)
		if err != nil {
			return nil, fmt.Errorf("opening object %s: %w", filepath, err)
		}
		defer reader.Close()

		blockReader, err := bstream.NewDBinBlockReader(reader)
		if err != nil {
			return nil, fmt.Errorf("creating block reader: %w", err)
		}

		block, err := blockReader.Read()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("no block found in file %s", filepath)
			}
			return nil, fmt.Errorf("reading block: %w", err)
		}

		return block, nil
	}

	// If no one-block files found, try merged blocks files
	return findBlockInMergedFiles(ctx, store, blockNum)
}

func findOneBlockFiles(ctx context.Context, store dstore.Store, blockNum uint64) ([]string, error) {
	var files []string
	filePrefix := fmt.Sprintf("%010d", blockNum)
	err := store.Walk(ctx, filePrefix, func(filename string) (err error) {
		files = append(files, filename)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking files: %w", err)
	}

	return files, nil
}

func findBlockInMergedFiles(ctx context.Context, store dstore.Store, blockNum uint64) (*pbbstream.Block, error) {
	// Calculate which merged file should contain this block
	// Merged files are typically named like 0000000000, 0000000100, etc. (100 blocks per file)
	segmentSize := uint64(100)
	startBlock := (blockNum / segmentSize) * segmentSize
	fileName := fmt.Sprintf("%010d", startBlock)

	// Try to open the merged file
	reader, err := store.OpenObject(ctx, fileName)
	if err != nil {
		return nil, fmt.Errorf("opening merged file %s: %w", fileName, err)
	}
	defer reader.Close()

	// Create a block reader for the merged file
	blockReader, err := bstream.NewDBinBlockReader(reader)
	if err != nil {
		return nil, fmt.Errorf("creating block reader for merged file: %w", err)
	}

	// Iterate through blocks in the file to find the one we want
	for {
		block, err := blockReader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("reading block from merged file: %w", err)
		}

		if block.Number == blockNum {
			return block, nil
		}

		// If we've passed the block number, it's not in this file
		if block.Number > blockNum {
			break
		}
	}

	return nil, fmt.Errorf("block %d not found in merged file %s", blockNum, fileName)
}

func compareBlocks(block1, block2 *pbbstream.Block, detailedDiff bool, rpcEndpoint string) []string {
	var differences []string

	// Compare basic block properties
	if block1.Number != block2.Number {
		differences = append(differences, fmt.Sprintf("Block numbers differ: %d vs %d", block1.Number, block2.Number))
	}

	if block1.Id != block2.Id {
		differences = append(differences, fmt.Sprintf("Block IDs differ: %s vs %s", block1.Id, block2.Id))
	}

	if block1.ParentId != block2.ParentId {
		differences = append(differences, fmt.Sprintf("Parent IDs differ: %s vs %s", block1.ParentId, block2.ParentId))
	}

	// Unmarshal payloads to compare the actual block data
	var stellarBlock1, stellarBlock2 pbstellar.Block
	if err := block1.Payload.UnmarshalTo(&stellarBlock1); err != nil {
		differences = append(differences, fmt.Sprintf("Failed to unmarshal block1 payload: %v", err))
		return differences
	}
	if err := block2.Payload.UnmarshalTo(&stellarBlock2); err != nil {
		differences = append(differences, fmt.Sprintf("Failed to unmarshal block2 payload: %v", err))
		return differences
	}

	// Compare transaction counts
	if len(stellarBlock1.Transactions) != len(stellarBlock2.Transactions) {
		differences = append(differences, fmt.Sprintf("Transaction counts differ: %d vs %d", len(stellarBlock1.Transactions), len(stellarBlock2.Transactions)))
	}

	// Compare transactions by matching them by hash
	tx1Map := make(map[string]*pbstellar.Transaction)
	for _, tx := range stellarBlock1.Transactions {
		tx1Map[fmt.Sprintf("%x", tx.Hash)] = tx
	}

	tx2Map := make(map[string]*pbstellar.Transaction)
	for _, tx := range stellarBlock2.Transactions {
		tx2Map[fmt.Sprintf("%x", tx.Hash)] = tx
	}

	// Find transactions that are in both blocks
	for hash, tx1 := range tx1Map {
		if tx2, exists := tx2Map[hash]; exists {
			if !proto.Equal(tx1, tx2) {
				txHash := fmt.Sprintf("%x", tx1.Hash)
				// Find the index for reporting
				var index int
				for idx, t := range stellarBlock1.Transactions {
					if fmt.Sprintf("%x", t.Hash) == hash {
						index = idx
						break
					}
				}
				txDiffs := compareTransactions(tx1, tx2, index, txHash, detailedDiff, rpcEndpoint)
				differences = append(differences, txDiffs...)
			}
		} else {
			differences = append(differences, fmt.Sprintf("Transaction %x exists in block1 but not in block2", tx1.Hash))
		}
	}

	// Find transactions that are only in block2
	for hash, tx2 := range tx2Map {
		if _, exists := tx1Map[hash]; !exists {
			differences = append(differences, fmt.Sprintf("Transaction %x exists in block2 but not in block1", tx2.Hash))
		}
	}

	return differences
}

func compareTransactions(tx1, tx2 *pbstellar.Transaction, index int, txHash string, detailedDiff bool, rpcEndpoint string) []string {
	var differences []string

	// Compare basic transaction fields
	if !bytes.Equal(tx1.Hash, tx2.Hash) {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): Hash differs - RPC: %x vs GS: %x", txHash, index, tx1.Hash, tx2.Hash))
	}

	if tx1.Status != tx2.Status {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): Status differs - RPC: %s vs GS: %s", txHash, index, tx1.Status, tx2.Status))
	}

	if tx1.ApplicationOrder != tx2.ApplicationOrder {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): ApplicationOrder differs - RPC: %d vs GS: %d", txHash, index, tx1.ApplicationOrder, tx2.ApplicationOrder))
	}

	// Compare envelope XDR
	if !bytes.Equal(tx1.EnvelopeXdr, tx2.EnvelopeXdr) {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): EnvelopeXdr differs - RPC and GS have different envelope data", txHash, index))
	}

	// Compare result XDR
	if !bytes.Equal(tx1.ResultXdr, tx2.ResultXdr) {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): ResultXdr differs - RPC and GS have different result data", txHash, index))
	}

	// Compare result meta XDR
	if !bytes.Equal(tx1.ResultMetaXdr, tx2.ResultMetaXdr) {
		// Try to decode core_metrics from both versions
		rpcCpu, rpcMem, rpcInvoke, rpcMaxKey, rpcMaxData, rpcMaxCode, rpcMaxEvent, rpcErr := decodeCoreMetrics(tx1.ResultMetaXdr)
		gsCpu, gsMem, gsInvoke, gsMaxKey, gsMaxData, gsMaxCode, gsMaxEvent, gsErr := decodeCoreMetrics(tx2.ResultMetaXdr)

		if rpcErr == nil && gsErr == nil {
			// Check if any metrics differ
			hasDifferences := rpcCpu != gsCpu || rpcMem != gsMem || rpcInvoke != gsInvoke ||
				rpcMaxKey != gsMaxKey || rpcMaxData != gsMaxData || rpcMaxCode != gsMaxCode || rpcMaxEvent != gsMaxEvent

			if hasDifferences {
				// Try to get the transaction from RPC to compare
				trxCpu, trxMem, trxInvoke, trxMaxKey, trxMaxData, trxMaxCode, trxMaxEvent, trxErr := getTrxCoreMetrics(rpcEndpoint, txHash)

				// Create table-like output for core_metrics differences
				tableLines := []string{
					fmt.Sprintf("Transaction %s (index %d): ResultMetaXdr core_metrics differ", txHash, index),
					"┌─────────────────────┬───────────────────────┬───────────────────────┬───────────────────────┐",
					"│ Value Name          │ RPC Get Ledger        │ GS Value              │ RPC getTransaction    │",
					"├─────────────────────┼───────────────────────┼───────────────────────┼───────────────────────┤",
				}

				// Add rows for each metric that differs
				if rpcCpu != gsCpu {
					trxValue := "N/A                      "
					if trxErr == nil {
						trxValue = fmt.Sprintf("%21d", trxCpu)
					}
					tableLines = append(tableLines, fmt.Sprintf("│ cpu_insns           │ %21d │ %21d │ %s │", rpcCpu, gsCpu, trxValue))
				}
				if rpcMem != gsMem {
					trxValue := "N/A                      "
					if trxErr == nil {
						trxValue = fmt.Sprintf("%21d", trxMem)
					}
					tableLines = append(tableLines, fmt.Sprintf("│ mem_bytes           │ %21d │ %21d │ %s │", rpcMem, gsMem, trxValue))
				}
				if rpcInvoke != gsInvoke {
					trxValue := "N/A                      "
					if trxErr == nil {
						trxValue = fmt.Sprintf("%21d", trxInvoke)
					}
					tableLines = append(tableLines, fmt.Sprintf("│ invoke_time_nsec    │ %21d │ %21d │ %s │", rpcInvoke, gsInvoke, trxValue))
				}
				if rpcMaxKey != gsMaxKey {
					trxValue := "N/A                      "
					if trxErr == nil {
						trxValue = fmt.Sprintf("%21d", trxMaxKey)
					}
					tableLines = append(tableLines, fmt.Sprintf("│ max_rw_key_byte     │ %21d │ %21d │ %s │", rpcMaxKey, gsMaxKey, trxValue))
				}
				if rpcMaxData != gsMaxData {
					trxValue := "N/A                      "
					if trxErr == nil {
						trxValue = fmt.Sprintf("%21d", trxMaxData)
					}
					tableLines = append(tableLines, fmt.Sprintf("│ max_rw_data_byte    │ %21d │ %21d │ %s │", rpcMaxData, gsMaxData, trxValue))
				}
				if rpcMaxCode != gsMaxCode {
					trxValue := "N/A                      "
					if trxErr == nil {
						trxValue = fmt.Sprintf("%21d", trxMaxCode)
					}
					tableLines = append(tableLines, fmt.Sprintf("│ max_rw_code_byte    │ %21d │ %21d │ %s │", rpcMaxCode, gsMaxCode, trxValue))
				}
				if rpcMaxEvent != gsMaxEvent {
					trxValue := "N/A                      "
					if trxErr == nil {
						trxValue = fmt.Sprintf("%21d", trxMaxEvent)
					}
					tableLines = append(tableLines, fmt.Sprintf("│ max_emit_event_byte │ %21d │ %21d │ %s │", rpcMaxEvent, gsMaxEvent, trxValue))
				}

				tableLines = append(tableLines, "└─────────────────────┴───────────────────────┴───────────────────────┴───────────────────────┘")

				// Join all lines into a single message
				msg := strings.Join(tableLines, "\n")
				differences = append(differences, msg)
			}
		} else {
			// Fall back to raw diff if decoding fails
			rpcValue := base64.StdEncoding.EncodeToString(tx1.ResultMetaXdr)
			gsValue := base64.StdEncoding.EncodeToString(tx2.ResultMetaXdr)

			// Find the first difference point
			minLen := len(rpcValue)
			if len(gsValue) < minLen {
				minLen = len(gsValue)
			}

			diffPos := -1
			for i := 0; i < minLen; i++ {
				if rpcValue[i] != gsValue[i] {
					diffPos = i
					break
				}
			}

			if diffPos >= 0 {
				// Show context around the difference
				start := diffPos - 20
				if start < 0 {
					start = 0
				}
				end := diffPos + 60
				if end > len(rpcValue) {
					end = len(rpcValue)
				}
				if end > len(gsValue) {
					end = len(gsValue)
				}

				rpcSnippet := rpcValue[start:end]
				gsSnippet := gsValue[start:end]

				if start > 0 {
					rpcSnippet = "..." + rpcSnippet
					gsSnippet = "..." + gsSnippet
				}
				if end < len(rpcValue) || end < len(gsValue) {
					rpcSnippet += "..."
					gsSnippet += "..."
				}

				differences = append(differences, fmt.Sprintf("Transaction %s (index %d): ResultMetaXdr differs at position %d - RPC: %s vs GS: %s", txHash, index, diffPos, rpcSnippet, gsSnippet))
			} else {
				// Different lengths
				differences = append(differences, fmt.Sprintf("Transaction %s (index %d): ResultMetaXdr differs in length - RPC: %d chars vs GS: %d chars", txHash, index, len(rpcValue), len(gsValue)))
			}
		}
	}

	// Compare events if they exist
	if tx1.Events == nil && tx2.Events != nil {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): Events differs - RPC: nil vs GS: present", txHash, index))
	} else if tx1.Events != nil && tx2.Events == nil {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): Events differs - RPC: present vs GS: nil", txHash, index))
	} else if tx1.Events != nil && tx2.Events != nil {
		// Compare diagnostic events
		if len(tx1.Events.DiagnosticEventsXdr) != len(tx2.Events.DiagnosticEventsXdr) {
			differences = append(differences, fmt.Sprintf("Transaction %s (index %d): DiagnosticEvents count differs - RPC: %d vs GS: %d", txHash, index, len(tx1.Events.DiagnosticEventsXdr), len(tx2.Events.DiagnosticEventsXdr)))
		} else {
			for i := 0; i < len(tx1.Events.DiagnosticEventsXdr); i++ {
				if !bytes.Equal(tx1.Events.DiagnosticEventsXdr[i], tx2.Events.DiagnosticEventsXdr[i]) {
					differences = append(differences, fmt.Sprintf("Transaction %s (index %d): DiagnosticEvent %d differs - RPC and GS have different event data", txHash, index, i))
				}
			}
		}

		// Compare transaction events
		if len(tx1.Events.TransactionEventsXdr) != len(tx2.Events.TransactionEventsXdr) {
			differences = append(differences, fmt.Sprintf("Transaction %s (index %d): TransactionEvents count differs - RPC: %d vs GS: %d", txHash, index, len(tx1.Events.TransactionEventsXdr), len(tx2.Events.TransactionEventsXdr)))
		} else {
			for i := 0; i < len(tx1.Events.TransactionEventsXdr); i++ {
				if !bytes.Equal(tx1.Events.TransactionEventsXdr[i], tx2.Events.TransactionEventsXdr[i]) {
					differences = append(differences, fmt.Sprintf("Transaction %s (index %d): TransactionEvent %d differs - RPC and GS have different event data", txHash, index, i))
				}
			}
		}

		// Compare contract events
		if len(tx1.Events.ContractEventsXdr) != len(tx2.Events.ContractEventsXdr) {
			differences = append(differences, fmt.Sprintf("Transaction %s (index %d): ContractEvents count differs - RPC: %d vs GS: %d", txHash, index, len(tx1.Events.ContractEventsXdr), len(tx2.Events.ContractEventsXdr)))
		} else {
			for i := 0; i < len(tx1.Events.ContractEventsXdr); i++ {
				ce1 := tx1.Events.ContractEventsXdr[i]
				ce2 := tx2.Events.ContractEventsXdr[i]
				if len(ce1.Events) != len(ce2.Events) {
					differences = append(differences, fmt.Sprintf("Transaction %s (index %d): ContractEvent %d events count differs - RPC: %d vs GS: %d", txHash, index, i, len(ce1.Events), len(ce2.Events)))
				} else {
					for j := 0; j < len(ce1.Events); j++ {
						if !bytes.Equal(ce1.Events[j], ce2.Events[j]) {
							differences = append(differences, fmt.Sprintf("Transaction %s (index %d): ContractEvent %d event %d differs - RPC and GS have different event data", txHash, index, i, j))
						}
					}
				}
			}
		}
	}

	// If no specific differences found but transactions are not equal, show JSON diff if requested
	if len(differences) == 0 && !proto.Equal(tx1, tx2) {
		if detailedDiff {
			jsonDiff := getTransactionJSONDiff(tx1, tx2)
			if jsonDiff != "" {
				differences = append(differences, fmt.Sprintf("Transaction %s (index %d): JSON diff between RPC and GS:\n%s", txHash, index, jsonDiff))
			} else {
				differences = append(differences, fmt.Sprintf("Transaction %s (index %d): Transactions differ between RPC and GS but no specific differences identified", txHash, index))
			}
		} else {
			differences = append(differences, fmt.Sprintf("Transaction %s (index %d): Transactions differ between RPC and GS (use --detailed-diff for more info)", txHash, index))
		}
	}

	return differences
}

func getTrxCoreMetrics(rpcEndpoint, txHash string) (cpu, mem, invoke, maxKey, maxData, maxCode, maxEvent uint64, err error) {
	// Create RPC client
	client := rpc.NewClient(rpcEndpoint, zap.NewNop(), nil)

	// Call getTransaction - try with 0x prefix first, then without
	ctx := context.Background()
	hashWithPrefix := "0x" + txHash
	trxResult, err := client.GetTransaction(ctx, hashWithPrefix)
	if err != nil {
		// Try without prefix
		trxResult, err = client.GetTransaction(ctx, txHash)
		if err != nil {
			// Print the error for debugging
			fmt.Printf("Error getting trx core metrics for %s: %v\n", txHash, err)
			// Return zeros to indicate failure
			return 0, 0, 0, 0, 0, 0, 0, nil
		}
	}

	// Decode ResultMetaXdr
	raw, err := base64.StdEncoding.DecodeString(trxResult.ResultMetaXdr)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("failed to decode ResultMetaXdr: %w", err)
	}

	return decodeCoreMetrics(raw)
}

func decodeCoreMetrics(data []byte) (cpu, mem, invoke, maxKey, maxData, maxCode, maxEvent uint64, err error) {
	if len(data) < 250 {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("data too short for core_metrics")
	}

	// Fast-forward to the last ~250 bytes where core_metrics live
	offset := len(data) - 250

	// Simple U64 decoding (big-endian)
	decodeU64 := func(data []byte, offset int) uint64 {
		if offset+8 > len(data) {
			return 0
		}
		return uint64(data[offset])<<56 |
			uint64(data[offset+1])<<48 |
			uint64(data[offset+2])<<40 |
			uint64(data[offset+3])<<32 |
			uint64(data[offset+4])<<24 |
			uint64(data[offset+5])<<16 |
			uint64(data[offset+6])<<8 |
			uint64(data[offset+7])
	}

	// Decode the 7 U64 values
	cpu = decodeU64(data, offset)
	mem = decodeU64(data, offset+8)
	invoke = decodeU64(data, offset+16)
	maxKey = decodeU64(data, offset+24)
	maxData = decodeU64(data, offset+32)
	maxCode = decodeU64(data, offset+40)
	maxEvent = decodeU64(data, offset+48)

	return cpu, mem, invoke, maxKey, maxData, maxCode, maxEvent, nil
}

func getTransactionJSONDiff(tx1, tx2 *pbstellar.Transaction) string {
	marshaler := &protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}

	json1, err := marshaler.Marshal(tx1)
	if err != nil {
		return fmt.Sprintf("Error marshaling tx1: %v", err)
	}

	json2, err := marshaler.Marshal(tx2)
	if err != nil {
		return fmt.Sprintf("Error marshaling tx2: %v", err)
	}

	r, err := jd.ReadJsonString(string(json1))
	if err != nil {
		return fmt.Sprintf("Error reading JSON1: %v", err)
	}

	c, err := jd.ReadJsonString(string(json2))
	if err != nil {
		return fmt.Sprintf("Error reading JSON2: %v", err)
	}

	diff := r.Diff(c)
	return diff.Render()
}
