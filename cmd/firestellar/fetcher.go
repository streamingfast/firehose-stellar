package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/streamingfast/cli/sflags"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-core/blockpoller"
	firecoreRPC "github.com/streamingfast/firehose-core/rpc"
	"github.com/streamingfast/firehose-stellar/rpc"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

func NewFetchCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rpc <first-streamable-block>",
		Short: "fetch blocks from rpc endpoint",
		Args:  cobra.ExactArgs(1),
		RunE:  fetchRunE(logger, tracer),
	}

	cmd.Flags().StringArray("endpoints", []string{}, "List of endpoints to use to fetch different method calls")
	cmd.Flags().String("state-dir", "/data/poller", "interval between fetch")
	cmd.Flags().Duration("interval-between-fetch", 0, "interval between fetch")
	cmd.Flags().Duration("latest-block-retry-interval", time.Second, "interval between fetch")
	cmd.Flags().Duration("max-block-fetch-duration", 3*time.Second, "maximum delay before considering a block fetch as failed")
	cmd.Flags().Int("block-fetch-batch-size", 1, "Number of blocks to fetch in a single batch")
	cmd.Flags().Int("client-limit", 200, "Limit for clients to fetch transactions")

	return cmd
}

func fetchRunE(logger *zap.Logger, _ logging.Tracer) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) (err error) {
		stateDir := sflags.MustGetString(cmd, "state-dir")

		startBlock, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse first streamable block %d: %w", startBlock, err)
		}

		fetchInterval := sflags.MustGetDuration(cmd, "interval-between-fetch")
		latestBlockRetryInterval := sflags.MustGetDuration(cmd, "latest-block-retry-interval")
		maxBlockFetchDuration := sflags.MustGetDuration(cmd, "max-block-fetch-duration")

		logger.Info(
			"launching firehose-stellar poller",
			zap.String("state_dir", stateDir),
			zap.Uint64("first_streamable_block", startBlock),
			zap.Duration("interval_between_fetch", fetchInterval),
			zap.Duration("latest_block_retry_interval", latestBlockRetryInterval),
		)

		rollingStrategy := firecoreRPC.NewStickyRollingStrategy[*rpc.Client]()

		rpcEndpoints := sflags.MustGetStringArray(cmd, "endpoints")
		rpcClients := firecoreRPC.NewClients(maxBlockFetchDuration, rollingStrategy, logger)
		for _, rpcEndpoint := range rpcEndpoints {
			client := rpc.NewClient(rpcEndpoint, logger)
			rpcClients.Add(client)
		}

		clientLimit := sflags.MustGetInt(cmd, "client-limit")

		poller := blockpoller.New(
			rpc.NewFetcher(fetchInterval, latestBlockRetryInterval, clientLimit, logger),
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
