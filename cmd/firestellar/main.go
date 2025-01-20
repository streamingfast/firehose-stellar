package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

var logger, tracer = logging.PackageLogger("firestellar", "github.com/streamingfast/firehose-stellar")
var rootCmd = &cobra.Command{
	Use:   "firestellar",
	Short: "Firehose Stellar fetching and tooling cli",
	Args:  cobra.ExactArgs(1),
}

func init() {
	logging.InstantiateLoggers(logging.WithDefaultLevel(zap.InfoLevel))
	rootCmd.AddCommand(newFetchCmd(logger, tracer))

	// Tool commands
	rootCmd.AddCommand(NewToolCreateAccountCmd(logger, tracer))
	rootCmd.AddCommand(NewToolDecodeBlockCmd(logger, tracer))
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		os.Exit(1)
	}
}

func newFetchCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "fetch blocks from different sources",
		Args:  cobra.ExactArgs(2),
	}
	cmd.AddCommand(NewFetchCmd(logger, tracer))
	return cmd
}
