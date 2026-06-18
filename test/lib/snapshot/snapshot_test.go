package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareWithBindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "case.expected.json")

	expected := `{
  "from": "$source",
  "to": "$dest",
  "amount": "10",
  "hash": "$hash"
}`
	if err := os.WriteFile(path, []byte(expected), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Bind("source", "GAAA")
	s.Bind("dest", "GBBB")
	s.Bind("hash", "abcd")

	actual := map[string]any{
		"from":   "GAAA",
		"to":     "GBBB",
		"amount": "10",
		"hash":   "abcd",
	}
	if err := s.Compare(actual); err != nil {
		t.Fatalf("expected match: %v", err)
	}

	mismatch := map[string]any{
		"from":   "GAAA",
		"to":     "GCCC", // diverges
		"amount": "10",
		"hash":   "abcd",
	}
	err = s.Compare(mismatch)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	if !strings.Contains(err.Error(), "to:") {
		t.Fatalf("expected diff to mention `to:`, got: %v", err)
	}
}

func TestSnapshotsUpdate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "regen.expected.json")

	t.Setenv("SNAPSHOTS_UPDATE", ".")

	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Bind("source", "GAAA")

	actual := map[string]any{"from": "GAAA", "amount": "1"}
	if err := s.Compare(actual); err != nil {
		t.Fatal(err)
	}

	blob, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(blob), `"$source"`) {
		t.Fatalf("expected $source placeholder in regenerated snapshot, got:\n%s", blob)
	}
}

// TestBindMatchesWholeValueNotSubstring guards the ledger-sequence collision: a
// short bound value (e.g. "43") must template a field only when it is the whole
// value, never when it appears as a substring of an unrelated field such as a
// deterministic account address. Regression for a snapshot mismatch where the
// ledger sequence landed inside the source-account address on a fresh chain.
func TestBindMatchesWholeValueNotSubstring(t *testing.T) {
	s := &Snapshot{bindings: map[string]string{}}
	s.Bind("ledger", "43")

	const address = "GD65IG7OO6TUGRY6WVFJDZAMSL5FKNKH7VSKV46A436QDWQD42PLCITV"
	normalized := s.normalize(map[string]any{
		"source": address, // contains "43" as a substring — must stay intact
		"seq":    "43",     // whole value — must be templated
	}).(map[string]any)

	if got := normalized["source"]; got != address {
		t.Fatalf("address corrupted by substring match: got %q, want %q", got, address)
	}
	if got := normalized["seq"]; got != "$ledger" {
		t.Fatalf("whole-value field not templated: got %q, want %q", got, "$ledger")
	}
}
