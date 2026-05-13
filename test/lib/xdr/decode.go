// Package xdr decodes Stellar XDR payloads (envelope, result, events) into
// JSON-friendly Go values. Battlefield snapshots compare structurally rather
// than byte-for-byte so that signatures, sequence numbers and other run-local
// noise can be normalized.
package xdr

import (
	"encoding/json"
	"fmt"

	"github.com/stellar/go-stellar-sdk/strkey"
	sdkxdr "github.com/stellar/go-stellar-sdk/xdr"

	pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"
)

// TxView is the structural projection of a single transaction we feed into
// snapshot comparison. Field names mirror the Stellar XDR vocabulary so
// snapshot diffs are recognizable.
type TxView struct {
	Hash             string         `json:"hash"`
	Status           string         `json:"status"`
	ApplicationOrder uint64         `json:"applicationOrder"`
	CreatedAt        string         `json:"createdAt"`
	Envelope         map[string]any `json:"envelope"`
	Result           map[string]any `json:"result"`
	Events           map[string]any `json:"events,omitempty"`
}

// FromTransaction decodes a firehose transaction into the structural view.
func FromTransaction(tx *pbstellar.Transaction) (*TxView, error) {
	envelope, err := decodeEnvelope(tx.EnvelopeXdr)
	if err != nil {
		return nil, fmt.Errorf("envelope: %w", err)
	}
	result, err := decodeResult(tx.ResultXdr)
	if err != nil {
		return nil, fmt.Errorf("result: %w", err)
	}
	events, err := decodeEvents(tx.Events)
	if err != nil {
		return nil, fmt.Errorf("events: %w", err)
	}
	view := &TxView{
		Hash:             fmt.Sprintf("%x", tx.Hash),
		Status:           tx.Status.String(),
		ApplicationOrder: tx.ApplicationOrder,
		Envelope:         envelope,
		Result:           result,
		Events:           events,
	}
	if tx.CreatedAt != nil {
		view.CreatedAt = tx.CreatedAt.AsTime().UTC().Format("2006-01-02T15:04:05Z")
	}
	return view, nil
}

func decodeEnvelope(blob []byte) (map[string]any, error) {
	var env sdkxdr.TransactionEnvelope
	if err := env.UnmarshalBinary(blob); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return roundtrip(env)
}

func decodeResult(blob []byte) (map[string]any, error) {
	var res sdkxdr.TransactionResult
	if err := res.UnmarshalBinary(blob); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}
	return roundtrip(res)
}

func decodeEvents(events *pbstellar.Events) (map[string]any, error) {
	if events == nil {
		return nil, nil
	}
	out := map[string]any{}

	if diags := decodeDiagnosticEvents(events.DiagnosticEventsXdr); diags != nil {
		out["diagnostic"] = diags
	}
	if txs := decodeTxEvents(events.TransactionEventsXdr); txs != nil {
		out["transaction"] = txs
	}
	if contracts := decodeContractEvents(events.ContractEventsXdr); contracts != nil {
		out["contract"] = contracts
	}

	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func decodeDiagnosticEvents(blobs [][]byte) []any {
	if len(blobs) == 0 {
		return nil
	}
	var out []any
	for _, blob := range blobs {
		var ev sdkxdr.DiagnosticEvent
		if err := ev.UnmarshalBinary(blob); err != nil {
			out = append(out, map[string]any{"_decodeError": err.Error()})
			continue
		}
		v, _ := roundtrip(ev)
		out = append(out, v)
	}
	return out
}

func decodeTxEvents(blobs [][]byte) []any {
	if len(blobs) == 0 {
		return nil
	}
	var out []any
	for _, blob := range blobs {
		var ev sdkxdr.TransactionEvent
		if err := ev.UnmarshalBinary(blob); err != nil {
			out = append(out, map[string]any{"_decodeError": err.Error()})
			continue
		}
		v, _ := roundtrip(ev)
		out = append(out, v)
	}
	return out
}

func decodeContractEvents(groups []*pbstellar.ContractEvent) []any {
	var out []any
	for _, group := range groups {
		var perOp []any
		for _, blob := range group.Events {
			var ev sdkxdr.ContractEvent
			if err := ev.UnmarshalBinary(blob); err != nil {
				perOp = append(perOp, map[string]any{"_decodeError": err.Error()})
				continue
			}
			v, _ := roundtrip(ev)
			perOp = append(perOp, v)
		}
		if perOp != nil {
			out = append(out, perOp)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// roundtrip serializes the value to JSON and back to map[string]any so the
// snapshot library has a uniform shape to walk, then applies three
// normalization passes that make snapshots stable and readable:
//
//  1. normalizeAccountIDs collapses `{Ed25519: [32 bytes]}` shapes to
//     their strkey "G…" form so $name-substitution can match them.
//
//  2. normalizeDynamicFields replaces per-run-varying fields (signatures,
//     signature hints, sequence numbers) with fixed placeholder strings.
//     These vary every run because the test creates fresh keypairs and
//     submits new transactions; without this pass every snapshot would
//     fail cross-run validation.
//
//  3. stripNulls removes nil entries from objects. The Go SDK marshals
//     discriminated unions as a struct with one populated field plus null
//     siblings for every other variant — pruning the nulls cuts snapshot
//     size by ~70% and keeps diffs surgical.
func roundtrip(v any) (map[string]any, error) {
	blob, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(blob, &out); err != nil {
		return nil, err
	}
	normalizeAccountIDs(out)
	normalizeDynamicFields(out)
	stripNulls(out)
	return out, nil
}

// stripNulls walks `v` recursively and deletes any map entry whose value is
// nil (Go's json.Unmarshal renders JSON null as a Go nil interface). Stellar
// XDR discriminated unions are heavy with these — only one variant is set
// per union, the rest marshal to null.
func stripNulls(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if val == nil {
				delete(x, k)
				continue
			}
			stripNulls(val)
		}
	case []any:
		for _, val := range x {
			stripNulls(val)
		}
	}
}

// normalizeDynamicFields walks `v` recursively and replaces per-run-
// varying fields with stable placeholder strings:
//
//   - DecoratedSignature: {Hint: [4 bytes], Signature: <base64 string>}
//     becomes {Hint: "$hint", Signature: "$signature"}
//   - Transaction.SeqNum (uint64)              → "$seqNum"
//   - BumpSequenceOp.BumpTo (uint64)           → "$bumpTo"
//   - InnerResultPair.TransactionHash ([32]B)  → "$innerTransactionHash"
//   - ContractEvent.ContractId ([32]B)         → "$contractId"
//
// Tests submit a fresh transaction every run with new keypairs and the
// horizon-assigned next-sequence-number for that account, so these
// values are inherently per-run noise. The contract IDs of Stellar Asset
// Contracts (SACs) are deterministic given the issuer keypair, but the
// issuer is fresh every run, so the resulting SAC ID also varies.
//
// We test that the field is *present and well-shaped*, not that the
// bytes match across runs — backend equivalence at the byte level is
// reasoned about transitively from the envelope (Ed25519 signatures and
// SAC contract IDs are deterministic functions of their inputs, so
// matching envelopes ⇒ matching signatures and SAC IDs).
func normalizeDynamicFields(v any) {
	switch x := v.(type) {
	case map[string]any:
		if isDecoratedSignature(x) {
			x["Hint"] = "$hint"
			x["Signature"] = "$signature"
			return
		}
		for k, val := range x {
			if placeholder, ok := dynamicFieldPlaceholder(k, val); ok {
				x[k] = placeholder
				continue
			}
			normalizeDynamicFields(val)
		}
	case []any:
		for _, val := range x {
			normalizeDynamicFields(val)
		}
	}
}

// dynamicFieldPlaceholder maps a (key, value) pair to its placeholder
// string if it's one of the known per-run-varying fields. Returns
// ("", false) otherwise.
func dynamicFieldPlaceholder(key string, val any) (string, bool) {
	switch key {
	case "SeqNum":
		if _, ok := val.(float64); ok {
			return "$seqNum", true
		}
	case "BumpTo":
		if _, ok := val.(float64); ok {
			return "$bumpTo", true
		}
	case "OfferId":
		// OfferId is a per-network monotonic counter assigned when a
		// ManageSellOffer/ManageBuyOffer creates a new offer. Stable
		// within a run, varies across runs.
		if _, ok := val.(float64); ok {
			return "$offerId", true
		}
	case "TransactionHash":
		if isThirtyTwoByteArray(val) {
			return "$innerTransactionHash", true
		}
	case "ContractId":
		// SAC contract IDs are deterministic functions of (passphrase,
		// asset, issuer pubkey). With deterministic test keypairs (set
		// via runner.Config.AccountSeedScope) the issuer is stable
		// across runs, so the contract ID is too. Render it as the
		// canonical "C…" strkey instead of a placeholder so the snapshot
		// shows real, debuggable values.
		if encoded, ok := tryEncodeStrkey(val, strkey.VersionByteContract); ok {
			return encoded, true
		}
	}
	return "", false
}

// isThirtyTwoByteArray returns true if v is a JSON-decoded `[]any` of
// 32 number-typed elements (the shape of a [32]byte field after a
// JSON round-trip).
func isThirtyTwoByteArray(v any) bool {
	arr, ok := v.([]any)
	if !ok || len(arr) != 32 {
		return false
	}
	for _, b := range arr {
		if _, ok := b.(float64); !ok {
			return false
		}
	}
	return true
}

// tryEncodeStrkey treats v as a 32-byte JSON-decoded array and encodes
// it with the given strkey version (e.g. VersionByteContract for "C…",
// VersionByteAccountID for "G…"). Returns ("", false) if the shape
// doesn't match.
func tryEncodeStrkey(v any, version strkey.VersionByte) (string, bool) {
	if !isThirtyTwoByteArray(v) {
		return "", false
	}
	arr := v.([]any)
	bytes := make([]byte, 32)
	for i, b := range arr {
		bytes[i] = byte(b.(float64))
	}
	encoded, err := strkey.Encode(version, bytes)
	if err != nil {
		return "", false
	}
	return encoded, true
}

// isDecoratedSignature matches the JSON shape Go's encoder produces for
// xdr.DecoratedSignature: a 2-key map with a 4-byte Hint array and a
// base64 Signature string.
func isDecoratedSignature(m map[string]any) bool {
	if len(m) != 2 {
		return false
	}
	hint, hasHint := m["Hint"].([]any)
	if !hasHint || len(hint) != 4 {
		return false
	}
	sig, hasSig := m["Signature"].(string)
	if !hasSig || sig == "" {
		return false
	}
	return true
}

// normalizeAccountIDs walks `v` recursively and replaces any nested
// `{Ed25519: [32 ints]}` shape with the canonical strkey "G…" string.
//
// Stellar XDR represents AccountID, MuxedAccount, and PublicKey as a
// discriminated union whose only currently-used variant carries the raw
// ed25519 public key. Marshaled to JSON without a Type discriminator
// (the SDK omits zero-valued types), they all share the
// `{Ed25519: byte-array}` shape, so a single normalizer covers them all.
//
// Unsupported variants (Med25519 muxed accounts) are left untouched —
// they have additional fields and don't match the single-key pattern.
func normalizeAccountIDs(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if encoded, ok := tryEncodeAccountID(val); ok {
				x[k] = encoded
				continue
			}
			normalizeAccountIDs(val)
		}
	case []any:
		for i, val := range x {
			if encoded, ok := tryEncodeAccountID(val); ok {
				x[i] = encoded
				continue
			}
			normalizeAccountIDs(val)
		}
	}
}

// tryEncodeAccountID returns the strkey form of an object whose only
// payload is a 32-byte Ed25519 public key, or ("", false) if the value
// isn't that shape. The pattern matches:
//
//   {Ed25519: [32 bytes]}                           // AccountId, PublicKey
//   {Type: 0, Ed25519: [32 bytes]}                  // MuxedAccount (KeyTypeEd25519)
//   {Type: 0, Ed25519: [32 bytes], Med25519: null}  // ditto, with explicit nulls
//
// The 32-byte length check guards against rewriting unrelated fields
// (signature blobs are 64 bytes; contract IDs are 32 bytes but live
// inside `ContractId.Hash`, not under `Ed25519`).
//
// If a `Type` field is present it must be numeric and zero (the only
// stellar discriminator under which Ed25519 means "an account address").
// All other map keys must be either absent or hold the JSON value `nil`
// — that's how Go's json.Marshal renders nil pointers in the
// alternative-variant fields of a discriminated union.
func tryEncodeAccountID(v any) (string, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", false
	}
	raw, ok := m["Ed25519"].([]any)
	if !ok || len(raw) != 32 {
		return "", false
	}
	for k, val := range m {
		switch k {
		case "Ed25519":
			continue
		case "Type":
			num, ok := val.(float64)
			if !ok || num != 0 {
				return "", false
			}
		default:
			// Sibling fields (Med25519, MuxedAccountId, etc.) must be
			// explicit nulls — non-nil siblings mean we're looking at a
			// different variant of the union.
			if val != nil {
				return "", false
			}
		}
	}
	bytes := make([]byte, 32)
	for i, b := range raw {
		num, ok := b.(float64)
		if !ok {
			return "", false
		}
		bytes[i] = byte(num)
	}
	encoded, err := strkey.Encode(strkey.VersionByteAccountID, bytes)
	if err != nil {
		return "", false
	}
	return encoded, true
}
