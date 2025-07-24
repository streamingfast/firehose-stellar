package main

import (
	"github.com/spf13/cobra"
	"github.com/streamingfast/cli"
	. "github.com/streamingfast/cli"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

// Injected at build time
var version = "<missing>"

var logger, _ = logging.PackageLogger("firestellar", "github.com/streamingfast/firehose-stellar")

func main() {
	logging.InstantiateLoggers(logging.WithDefaultLevel(zap.InfoLevel))

	Run(
		"firestellar",
		"Firehose Stellar block fetching and tooling",

		ConfigureVersion(version),
		ConfigureViper("FIRESTELLAR"),

		CobraCmd(NewToolDecodeBlockCmd()),
		CobraCmd(NewToolCreateAccountCmd()),
		CobraCmd(NewToolSendPaymentCmd()),
		CobraCmd(NewToolIssueAssetCmd()),
		CobraCmd(NewToolSendPaymentAssetCmd()),
		CobraCmd(NewToolDecodeSeedCmd()),

		OnCommandErrorLogAndExit(logger),
	)
}

func CobraCmd(cmd *cobra.Command) cli.CommandOption {
	return cli.CommandOptionFunc(func(parent *cobra.Command) {
		parent.AddCommand(cmd)
	})
}
