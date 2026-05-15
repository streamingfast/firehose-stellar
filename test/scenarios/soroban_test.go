// Soroban scenarios — placeholders. These exercise transaction shapes that
// are the most-likely sources of poller↔captive-core divergence (diagnostic
// events, contract events, failed-but-included transactions). They are
// gated behind BATTLEFIELD_SOROBAN=1 because they need:
//
//   - a deployed test contract (WASM under contracts/, deployed once and
//     pinned via env or fixture)
//   - a quickstart docker with --enable-soroban-rpc
//
// TODO(phase-2): port the substreams-stellar-soroban example contracts and
// add invoke/event/failure scenarios here.
package scenarios

import (
	"os"
	"testing"
)

func skipUnlessSoroban(t *testing.T) {
	t.Helper()
	if os.Getenv("BATTLEFIELD_SOROBAN") != "1" {
		t.Skip("set BATTLEFIELD_SOROBAN=1 to run soroban scenarios")
	}
}

func TestSorobanInvoke(t *testing.T)             { skipUnlessSoroban(t); t.Skip("TODO: contract deploy + invoke") }
func TestSorobanFailedInvoke(t *testing.T)       { skipUnlessSoroban(t); t.Skip("TODO: contract panics, tx INCLUDED with status=FAILED") }
func TestSorobanContractEvents(t *testing.T)     { skipUnlessSoroban(t); t.Skip("TODO: emit multiple events from a single op") }
func TestSorobanCrossContract(t *testing.T)      { skipUnlessSoroban(t); t.Skip("TODO: contract A calls contract B") }
func TestSorobanDiagnosticEvents(t *testing.T)   { skipUnlessSoroban(t); t.Skip("TODO: poller and captive-core differ on diag event presence") }
