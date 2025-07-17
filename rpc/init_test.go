package rpc

import "github.com/streamingfast/logging"

var testLog, testTracer = logging.PackageLogger("rpc", "github.com/streamingfast/firehose-stellar/rpc")

func init() {
	logging.InstantiateLoggers()
}
