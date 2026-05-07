package rpc

import (
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/streamingfast/logging"
)

var testLog, testTracer = logging.PackageLogger("rpc", "github.com/streamingfast/firehose-stellar/rpc")

func init() {
	logging.InstantiateLoggers()
}

// passphraseFor returns the matching network passphrase for a given test
// rpc endpoint. Mirrors the resolution that production code does via the
// --stellar-rpc-network flag, but keeps the tests self-contained.
func passphraseFor(endpoint string) string {
	if endpoint == RPC_MAINNET_ENDPOINT {
		return network.PublicNetworkPassphrase
	}
	return network.TestNetworkPassphrase
}
