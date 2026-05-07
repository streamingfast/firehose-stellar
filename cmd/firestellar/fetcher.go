package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/streamingfast/cli/sflags"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-core/blockpoller"
	firecoreRPC "github.com/streamingfast/firehose-core/rpc"
	"github.com/streamingfast/firehose-stellar/rpc"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

func NewFetchRpcCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rpc <first-streamable-block>",
		Short: "fetch blocks from rpc endpoint",
		Args:  cobra.ExactArgs(1),
		RunE:  fetchRpcRunE(logger, tracer),
	}

	cmd.Flags().StringArray("endpoints", []string{}, "List of endpoints to use to fetch different method calls")
	cmd.Flags().String("state-dir", "/data/poller", "interval between fetch")
	cmd.Flags().Duration("interval-between-fetch", 0, "interval between fetch")
	cmd.Flags().Duration("latest-block-retry-interval", time.Second, "interval between fetch")
	cmd.Flags().Duration("max-block-fetch-duration", 3*time.Second, "maximum delay before considering a block fetch as failed")
	cmd.Flags().Int("block-fetch-batch-size", 1, "Number of blocks to fetch in a single batch")
	cmd.Flags().Int("transaction-fetch-limit", 200, "Maximum number of transactions to fetch at the same time")
	cmd.Flags().String("stellar-rpc-network", "testnet", "stellar network the rpc endpoint serves (mainnet, testnet, or custom)")
	cmd.Flags().String("stellar-rpc-network-passphrase", "", "override network passphrase (required for custom; overrides the value derived from --stellar-rpc-network when set)")

	// Deprecated: --is-mainnet was the original flag and is kept for
	// backwards compatibility. Prefer --stellar-rpc-network=mainnet|testnet
	// or --stellar-rpc-network-passphrase=... for explicit control. When
	// both are set, the new flags win.
	cmd.Flags().Bool("is-mainnet", false, "DEPRECATED: use --stellar-rpc-network=mainnet|testnet instead")
	_ = cmd.Flags().MarkDeprecated("is-mainnet", "use --stellar-rpc-network=mainnet|testnet instead")

	return cmd
}

func fetchRpcRunE(logger *zap.Logger, tracer logging.Tracer) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) (err error) {
		stateDir := sflags.MustGetString(cmd, "state-dir")

		startBlock, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse first streamable block %d: %w", startBlock, err)
		}

		fetchInterval := sflags.MustGetDuration(cmd, "interval-between-fetch")
		latestBlockRetryInterval := sflags.MustGetDuration(cmd, "latest-block-retry-interval")
		maxBlockFetchDuration := sflags.MustGetDuration(cmd, "max-block-fetch-duration")

		networkPassphrase, err := resolveRPCNetworkPassphrase(cmd)
		if err != nil {
			return err
		}

		logger.Info(
			"launching firehose-stellar poller",
			zap.String("state_dir", stateDir),
			zap.Uint64("first_streamable_block", startBlock),
			zap.Duration("interval_between_fetch", fetchInterval),
			zap.Duration("latest_block_retry_interval", latestBlockRetryInterval),
			zap.String("network_passphrase", networkPassphrase),
		)

		rollingStrategy := firecoreRPC.NewStickyRollingStrategy[*rpc.Client]()

		rpcEndpoints := sflags.MustGetStringArray(cmd, "endpoints")
		rpcClients := firecoreRPC.NewClients(maxBlockFetchDuration, rollingStrategy, logger)
		for _, rpcEndpoint := range rpcEndpoints {
			client := rpc.NewClient(rpcEndpoint, logger, tracer)
			rpcClients.Add(client)
		}

		transactionFetchLimit := sflags.MustGetInt(cmd, "transaction-fetch-limit")

		poller := blockpoller.New(
			rpc.NewFetcher(fetchInterval, latestBlockRetryInterval, transactionFetchLimit, networkPassphrase, logger),
			blockpoller.NewFireBlockHandler("type.googleapis.com/sf.stellar.type.v1.Block"),
			rpcClients,
			blockpoller.WithStoringState[*rpc.Client](stateDir),
			blockpoller.WithLogger[*rpc.Client](logger),
		)

		err = poller.Run(startBlock, nil, sflags.MustGetInt(cmd, "block-fetch-batch-size"))
		if err != nil {
			return fmt.Errorf("running poller: %w", err)
		}

		return nil
	}
}

// resolveRPCNetworkPassphrase derives the network passphrase to use for
// the rpc fetcher. Resolution order, highest precedence first:
//
//  1. --stellar-rpc-network-passphrase=<string>  (explicit override)
//  2. --stellar-rpc-network=mainnet|testnet|custom
//  3. --is-mainnet  (deprecated; only consulted if the new flags are
//     untouched)
//
// `custom` requires --stellar-rpc-network-passphrase to also be set.
func resolveRPCNetworkPassphrase(cmd *cobra.Command) (string, error) {
	networkName := sflags.MustGetString(cmd, "stellar-rpc-network")
	override := sflags.MustGetString(cmd, "stellar-rpc-network-passphrase")

	// Explicit override always wins.
	if override != "" {
		return override, nil
	}

	switch networkName {
	case "mainnet":
		return network.PublicNetworkPassphrase, nil
	case "testnet":
		// If the user only set --is-mainnet=true, honor it (back-compat).
		// `MustGetBool` returns false when the flag isn't present.
		if cmd.Flags().Changed("is-mainnet") && sflags.MustGetBool(cmd, "is-mainnet") {
			return network.PublicNetworkPassphrase, nil
		}
		return network.TestNetworkPassphrase, nil
	case "custom":
		return "", fmt.Errorf("--stellar-rpc-network-passphrase is required when --stellar-rpc-network=custom")
	default:
		return "", fmt.Errorf("unsupported stellar rpc network: %s (want mainnet|testnet|custom)", networkName)
	}
}
