package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
	"time"
)

var logger, tracer = logging.PackageLogger("firestellar", "github.com/streamingfast/firehose-stellar")
var rootCmd = &cobra.Command{
	Use:   "firesol",
	Short: "firesol fetching and tooling",
}

func init() {
	logging.InstantiateLoggers(logging.WithDefaultLevel(zap.InfoLevel))
	rootCmd.AddCommand(newFetchCmd(logger, tracer))
}

func main() {
	fmt.Println("Goodbye!")
}

func newFetchCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "fetch blocks from different sources",
		Args:  cobra.ExactArgs(2),
	}
	time.Now().UnixMilli()
	cmd.AddCommand(NewFetchCmd(logger, tracer))
	return cmd
}
