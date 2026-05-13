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
