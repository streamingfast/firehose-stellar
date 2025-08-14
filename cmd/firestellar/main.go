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

var logger, tracer = logging.PackageLogger("firestellar", "github.com/streamingfast/firehose-stellar")

func main() {
	logging.InstantiateLoggers(logging.WithDefaultLevel(zap.InfoLevel))

	Run(
		"firestellar",
		"Firehose Stellar block fetching and tooling",
		Description(`
			Firehose Stellar implements the Firehose Reader protocol for Stellar,
			via 'firestellar rpc fetch <flags>' (see 'firestellar rpc fetch --help').

			It is expected to be used with the Firehose Stack by operating 'firecore'
			binary which spans Firehose Stellar Reader as a subprocess and reads from
			it producing blocks and offering Firehose & Substreams APIs.

			Read the Firehose documentation at firehose.streamingfast.io for more
			information how to use this binary.

			The binary also contains a few commands to test the Stellar block
			fetching capabilities, such as fetching a block by number or hash.
		`),

		ConfigureVersion(version),
		ConfigureViper("FIRESTELLAR"),

		Group("fetch", "Reader Node fetch RPC command",
			CobraCmd(NewFetchCmd(logger, tracer)),
		),

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
