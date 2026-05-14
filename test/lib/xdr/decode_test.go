package xdr

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestNormalizeAccountIDs(t *testing.T) {
	// Synthesize a tree containing every shape we expect in real
	// xdr-encoded JSON and verify each gets collapsed to a strkey string
	// (or correctly skipped). Numbers use json.Number to match what
	// roundtrip() produces at runtime (UseNumber-enabled decoder).
	src := map[string]any{
		"envelope": map[string]any{
			// Shape A: MuxedAccount with explicit Type discriminator. This
			// is what xdr.MuxedAccount marshals as via Go's json package.
			"SourceAccount": map[string]any{
				"Type":    json.Number("0"),
				"Ed25519": jsonNumberSlice(make32Bytes(0x42)),
			},
			"Other": "ignored",
		},
		"events": []any{
			map[string]any{
				"Address": map[string]any{
					// Shape B: AccountId — just Ed25519.
					"AccountId": map[string]any{
						"Ed25519": jsonNumberSlice(make32Bytes(0x37)),
					},
				},
			},
		},
		// Shape C: a non-account-id variant of the same union. Type != 0
		// means a different discriminator (e.g. SignerKey HashX), so we
		// must NOT collapse it.
		"signerKey": map[string]any{
			"Type":    json.Number("2"),
			"Ed25519": jsonNumberSlice(make32Bytes(0x99)),
		},
		// Shape D: 64-byte ed25519 (signature, not pubkey) — must remain.
		"signature": map[string]any{
			"Ed25519": jsonNumberSlice(make([]byte, 64)),
		},
		// Shape E: explicit-null sibling fields (Go json renders nil
		// pointers as null), with Type=0 — should still collapse.
		"muxedAccountWithNullSiblings": map[string]any{
			"Type":     json.Number("0"),
			"Ed25519":  jsonNumberSlice(make32Bytes(0x55)),
			"Med25519": nil,
		},
	}

	normalizeAccountIDs(src)

	// SourceAccount should now be a strkey string starting with "G".
	envelope := src["envelope"].(map[string]any)
	got, ok := envelope["SourceAccount"].(string)
	if !ok || len(got) != 56 || got[0] != 'G' {
		t.Fatalf("envelope.SourceAccount: expected strkey 'G…' (56 chars), got %T = %v", envelope["SourceAccount"], envelope["SourceAccount"])
	}

	// AccountId nested inside Address should likewise be collapsed.
	ev := src["events"].([]any)[0].(map[string]any)
	addr := ev["Address"].(map[string]any)
	if _, ok := addr["AccountId"].(string); !ok {
		t.Fatalf("Address.AccountId: expected strkey string, got %T", addr["AccountId"])
	}

	// 64-byte signature must remain a byte array (not the AccountID shape).
	sig := src["signature"].(map[string]any)
	if _, ok := sig["Ed25519"].([]any); !ok {
		t.Fatalf("signature.Ed25519: should still be byte array, got %T", sig["Ed25519"])
	}

	// Type != 0 (different union variant) must NOT be collapsed.
	sk := src["signerKey"].(map[string]any)
	if _, ok := sk["Ed25519"].([]any); !ok {
		t.Fatalf("signerKey: Type=2 means non-account variant, must not collapse; got %T", sk["Ed25519"])
	}

	// Explicit-null siblings (e.g. Med25519: null) should still allow
	// collapse because the variant is determined by Type=0.
	if _, ok := src["muxedAccountWithNullSiblings"].(string); !ok {
		t.Fatalf("muxedAccountWithNullSiblings: expected strkey, got %T", src["muxedAccountWithNullSiblings"])
	}
}

// make32Bytes fills a 32-byte slice with the given byte value, easy to
// recognise in test failures.
func make32Bytes(b byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = b
	}
	return out
}

// jsonNumberSlice produces `[]any{json.Number, json.Number, …}` — the
// exact shape the runtime walker sees after roundtrip()'s UseNumber
// decode of an xdr Uint256 (a fixed [32]byte array, marshaled by Go as
// a JSON number array, not as base64 like a []byte slice would be).
func jsonNumberSlice(b []byte) []any {
	out := make([]any, len(b))
	for i, v := range b {
		out[i] = json.Number(strconv.Itoa(int(v)))
	}
	return out
}

// TestNormalizeDynamicFields covers the per-run-varying fields that
// normalizeDynamicFields rewrites to placeholder strings.
func TestNormalizeDynamicFields(t *testing.T) {
	src := map[string]any{
		"V1": map[string]any{
			"Tx": map[string]any{
				// SeqNum gets replaced. json.Number mirrors the runtime
				// UseNumber decode and exercises numbers above 2^53 that
				// would lose precision as float64.
				"SeqNum": json.Number("9007199254740993"),
				"Operations": []any{
					map[string]any{
						"Body": map[string]any{
							// BumpTo gets replaced.
							"BumpSequenceOp": map[string]any{
								"BumpTo": json.Number("999"),
							},
						},
					},
				},
				// Signature {Hint, Signature} also gets replaced (existing).
				"Signatures": []any{
					map[string]any{
						"Hint":      jsonNumberSlice([]byte{1, 2, 3, 4}),
						"Signature": "AAAA",
					},
				},
			},
		},
		"Result": map[string]any{
			"InnerResultPair": map[string]any{
				// TransactionHash 32-byte array gets replaced.
				"TransactionHash": jsonNumberSlice(make32Bytes(0xAB)),
			},
		},
		"events": []any{
			map[string]any{
				// ContractId 32-byte array gets replaced.
				"ContractId": jsonNumberSlice(make32Bytes(0xCD)),
			},
		},
		// Should NOT be touched: a 32-byte array under an unrelated key.
		"randomHash": jsonNumberSlice(make32Bytes(0xEF)),
	}

	normalizeDynamicFields(src)

	tx := src["V1"].(map[string]any)["Tx"].(map[string]any)
	if got := tx["SeqNum"]; got != "$seqNum" {
		t.Errorf("SeqNum: want $seqNum, got %v", got)
	}
	op := tx["Operations"].([]any)[0].(map[string]any)["Body"].(map[string]any)["BumpSequenceOp"].(map[string]any)
	if got := op["BumpTo"]; got != "$bumpTo" {
		t.Errorf("BumpTo: want $bumpTo, got %v", got)
	}
	sig := tx["Signatures"].([]any)[0].(map[string]any)
	if got := sig["Signature"]; got != "$signature" {
		t.Errorf("Signature: want $signature, got %v", got)
	}
	pair := src["Result"].(map[string]any)["InnerResultPair"].(map[string]any)
	if got := pair["TransactionHash"]; got != "$innerTransactionHash" {
		t.Errorf("TransactionHash: want $innerTransactionHash, got %v", got)
	}
	ev := src["events"].([]any)[0].(map[string]any)
	contractStrkey, ok := ev["ContractId"].(string)
	if !ok || len(contractStrkey) != 56 || contractStrkey[0] != 'C' {
		t.Errorf("ContractId: expected 'C…' strkey (56 chars), got %T = %v", ev["ContractId"], ev["ContractId"])
	}
	// randomHash must remain a byte array — not a known dynamic field key.
	if _, ok := src["randomHash"].([]any); !ok {
		t.Errorf("randomHash: should remain a byte array, got %T", src["randomHash"])
	}
}
