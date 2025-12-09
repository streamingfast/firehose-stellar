package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/dstore"
	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
	"github.com/streamingfast/firehose-stellar/rpc"
	"go.uber.org/zap"
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

func compareBlocks(rpcBlock, gsBlock *pbbstream.Block, detailedDiff bool, rpcEndpoint string) []string {
	var differences []string

	// Compare basic block properties
	if rpcBlock.Number != gsBlock.Number {
		differences = append(differences, fmt.Sprintf("Block numbers differ: %d vs %d", rpcBlock.Number, gsBlock.Number))
	}

	if rpcBlock.Id != gsBlock.Id {
		differences = append(differences, fmt.Sprintf("Block IDs differ: %s vs %s", rpcBlock.Id, gsBlock.Id))
	}

	if rpcBlock.ParentId != gsBlock.ParentId {
		differences = append(differences, fmt.Sprintf("Parent IDs differ: %s vs %s", rpcBlock.ParentId, gsBlock.ParentId))
	}

	// Unmarshal payloads to compare the actual block data
	var rcpStellarBlock, gsStellarBlock pbstellar.Block
	if err := rpcBlock.Payload.UnmarshalTo(&rcpStellarBlock); err != nil {
		differences = append(differences, fmt.Sprintf("Failed to unmarshal RPC payload: %v", err))
		return differences
	}
	if err := gsBlock.Payload.UnmarshalTo(&gsStellarBlock); err != nil {
		differences = append(differences, fmt.Sprintf("Failed to unmarshal GS payload: %v", err))
		return differences
	}

	// Compare transaction counts
	if len(rcpStellarBlock.Transactions) != len(gsStellarBlock.Transactions) {
		differences = append(differences, fmt.Sprintf("Transaction counts differ: %d vs %d", len(rcpStellarBlock.Transactions), len(gsStellarBlock.Transactions)))
	}

	// Compare transactions by matching them by hash
	rpcTrxMap := make(map[string]*pbstellar.Transaction)
	for _, tx := range rcpStellarBlock.Transactions {
		rpcTrxMap[fmt.Sprintf("%x", tx.Hash)] = tx
	}

	gsTrxMap := make(map[string]*pbstellar.Transaction)
	for _, tx := range gsStellarBlock.Transactions {
		gsTrxMap[fmt.Sprintf("%x", tx.Hash)] = tx
	}

	// Find transactions that are in both blocks
	for hash, rpcTx := range rpcTrxMap {
		if gsTx, exists := gsTrxMap[hash]; exists {
			if !proto.Equal(rpcTx, gsTx) {
				txHash := fmt.Sprintf("%x", rpcTx.Hash)
				// Find the index for reporting
				var index int
				for idx, t := range rcpStellarBlock.Transactions {
					if fmt.Sprintf("%x", t.Hash) == hash {
						index = idx
						break
					}
				}
				txDiffs := compareTransactions(rpcTx, gsTx, index, txHash, detailedDiff, rpcEndpoint)
				differences = append(differences, txDiffs...)
			}
		} else {
			differences = append(differences, fmt.Sprintf("Transaction %x exists in RPC but not in GS", rpcTx.Hash))
		}
	}

	// Find transactions that are only in GS
	for hash, tx2 := range gsTrxMap {
		if _, exists := rpcTrxMap[hash]; !exists {
			differences = append(differences, fmt.Sprintf("Transaction %x exists in GS but not in RPC", tx2.Hash))
		}
	}

	return differences
}

func compareTransactions(rpcTx, gsTx *pbstellar.Transaction, index int, txHash string, detailedDiff bool, rpcEndpoint string) []string {
	var differences []string

	// Compare basic transaction fields
	tx1HashHex := fmt.Sprintf("%x", rpcTx.Hash)
	tx2HashHex := fmt.Sprintf("%x", gsTx.Hash)
	if tx1HashHex != tx2HashHex {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): Hash differs - RPC: %s vs GS: %s", txHash, index, tx1HashHex, tx2HashHex))
	}

	if rpcTx.Status != gsTx.Status {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): Status differs - RPC: %s vs GS: %s", txHash, index, rpcTx.Status, gsTx.Status))
	}

	if rpcTx.ApplicationOrder != gsTx.ApplicationOrder {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): ApplicationOrder differs - RPC: %d vs GS: %d", txHash, index, rpcTx.ApplicationOrder, gsTx.ApplicationOrder))
	}

	// Compare envelope XDR
	if !bytes.Equal(rpcTx.EnvelopeXdr, gsTx.EnvelopeXdr) {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): EnvelopeXdr differs - RPC and GS have different envelope data", txHash, index))
	}

	// Compare result XDR
	if !bytes.Equal(rpcTx.ResultXdr, gsTx.ResultXdr) {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): ResultXdr differs - RPC and GS have different result data", txHash, index))
	}

	// Compare events if they exist
	if rpcTx.Events == nil && gsTx.Events != nil {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): Events differs - RPC: nil vs GS: present", txHash, index))
	} else if rpcTx.Events != nil && gsTx.Events == nil {
		differences = append(differences, fmt.Sprintf("Transaction %s (index %d): Events differs - RPC: present vs GS: nil", txHash, index))
	} else if rpcTx.Events != nil && gsTx.Events != nil {
		// Compare diagnostic events
		//if len(rpcTx.Events.DiagnosticEventsXdr) != len(gsTx.Events.DiagnosticEventsXdr) {
		//	differences = append(differences, fmt.Sprintf("Transaction %s (index %d): DiagnosticEvents count differs - RPC: %d vs GS: %d", txHash, index, len(rpcTx.Events.DiagnosticEventsXdr), len(gsTx.Events.DiagnosticEventsXdr)))
		//} else {
		//	for i := 0; i < len(rpcTx.Events.DiagnosticEventsXdr); i++ {
		//		if !bytes.Equal(rpcTx.Events.DiagnosticEventsXdr[i], gsTx.Events.DiagnosticEventsXdr[i]) {
		//			differences = append(differences, fmt.Sprintf("Transaction %s (index %d): DiagnosticEvent %d differs - RPC and GS have different event data", txHash, index, i))
		//		}
		//	}
		//}

		// Compare transaction events
		if len(rpcTx.Events.TransactionEventsXdr) != len(gsTx.Events.TransactionEventsXdr) {
			differences = append(differences, fmt.Sprintf("Transaction %s (index %d): TransactionEvents count differs - RPC: %d vs GS: %d", txHash, index, len(rpcTx.Events.TransactionEventsXdr), len(gsTx.Events.TransactionEventsXdr)))
		} else {
			for i := 0; i < len(rpcTx.Events.TransactionEventsXdr); i++ {
				if !bytes.Equal(rpcTx.Events.TransactionEventsXdr[i], gsTx.Events.TransactionEventsXdr[i]) {
					differences = append(differences, fmt.Sprintf("Transaction %s (index %d): TransactionEvent %d differs - RPC and GS have different event data", txHash, index, i))
				}
			}
		}

		// Compare contract events
		gsContractEventCount := 0
		for _, event := range gsTx.Events.ContractEventsXdr {
			if len(event.Events) > 0 {
				gsContractEventCount++
			}
		}

		rpcTxContractEventCount := 0
		for _, event := range rpcTx.Events.ContractEventsXdr {
			if len(event.Events) > 0 {
				rpcTxContractEventCount++
			}
		}

		if len(rpcTx.Events.ContractEventsXdr) != gsContractEventCount {
			differences = append(differences, fmt.Sprintf("Transaction %s (index %d): ContractEvents count differs - RPC: %d vs GS: %d", txHash, index, len(rpcTx.Events.ContractEventsXdr), len(gsTx.Events.ContractEventsXdr)))
		} else {
			for i := 0; i < len(rpcTx.Events.ContractEventsXdr); i++ {
				ce1 := rpcTx.Events.ContractEventsXdr[i]
				ce2 := gsTx.Events.ContractEventsXdr[i]
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

	return differences
}
