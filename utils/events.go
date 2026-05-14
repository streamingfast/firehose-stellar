package utils

import (
	"github.com/stellar/go-stellar-sdk/xdr"
)

// IsNonDeterministicDiagnosticEvent reports whether a Soroban
// diagnostic event is wall-clock dependent and therefore varies across
// fetchers (captive-core replay vs RPC live observation).
//
// Currently only matches core_metrics.invoke_time_nsecs, which the
// host emits on every Soroban invocation with the measured wall-clock
// duration of the call. Other core_metrics (cpu_insns, mem_bytes,
// ledger_*_count) are deterministic functions of execution and match
// across fetchers.
//
// Stripping these before persisting blocks makes outputs byte-identical
// across fetchers.
func IsNonDeterministicDiagnosticEvent(ev xdr.DiagnosticEvent) bool {
	body := ev.Event.Body.V0
	if body == nil || len(body.Topics) < 2 {
		return false
	}
	t0, t1 := body.Topics[0], body.Topics[1]
	if t0.Type != xdr.ScValTypeScvSymbol || t1.Type != xdr.ScValTypeScvSymbol {
		return false
	}
	if t0.Sym == nil || t1.Sym == nil {
		return false
	}
	return string(*t0.Sym) == "core_metrics" && string(*t1.Sym) == "invoke_time_nsecs"
}

// IsNonDeterministicDiagnosticEventBytes is the raw-bytes variant: it
// decodes the XDR blob first, then defers to
// IsNonDeterministicDiagnosticEvent. Returns false if the bytes do not
// decode as a DiagnosticEvent.
func IsNonDeterministicDiagnosticEventBytes(raw []byte) bool {
	var ev xdr.DiagnosticEvent
	if err := ev.UnmarshalBinary(raw); err != nil {
		return false
	}
	return IsNonDeterministicDiagnosticEvent(ev)
}
