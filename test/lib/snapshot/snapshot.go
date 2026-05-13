// Package snapshot provides JSON expected-vs-actual comparison with $var
// templating, modeled on the battlefield-ethereum snapshot library.
//
// Workflow:
//
//	expected, err := snapshot.Load("snapshots/payment/native.expected.json")
//	expected.Bind("source", srcAddr)
//	expected.Bind("dest", destAddr)
//	if err := expected.Compare(actual); err != nil { … }
//
// When SNAPSHOTS_UPDATE matches the snapshot path (regex), Compare writes
// `actual` to the expected file with bound variables re-substituted as $var
// placeholders, instead of returning a diff.
package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

type Snapshot struct {
	Path     string
	Expected any
	bindings map[string]string
}

// Load reads an expected snapshot from disk. A missing file is treated as an
// empty expectation (useful for first-run regeneration via SNAPSHOTS_UPDATE).
func Load(path string) (*Snapshot, error) {
	s := &Snapshot{Path: path, bindings: map[string]string{}}

	blob, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read snapshot %s: %w", path, err)
	}
	if err := json.Unmarshal(blob, &s.Expected); err != nil {
		return nil, fmt.Errorf("parse snapshot %s: %w", path, err)
	}
	return s, nil
}

// Bind associates a $var placeholder with a runtime value. During Compare,
// every occurrence of the value in `actual` is replaced with `$name` before
// the deep equality check, so snapshots stay stable across runs.
func (s *Snapshot) Bind(name, value string) {
	if value == "" {
		return
	}
	s.bindings["$"+name] = value
}

// Compare normalizes `actual` by substituting bound values with their $var
// placeholders, then deep-compares against the expected JSON.
//
// If the SNAPSHOTS_UPDATE env var is set and matches s.Path as a regex, the
// normalized actual is written to disk and Compare returns nil.
func (s *Snapshot) Compare(actual any) error {
	normalized := s.normalize(actual)

	if pattern := os.Getenv("SNAPSHOTS_UPDATE"); pattern != "" {
		matched, err := regexp.MatchString(pattern, s.Path)
		if err != nil {
			return fmt.Errorf("invalid SNAPSHOTS_UPDATE regex %q: %w", pattern, err)
		}
		if matched {
			return s.write(normalized)
		}
	}

	if s.Expected == nil {
		return fmt.Errorf("no snapshot at %s — set SNAPSHOTS_UPDATE=%s to record one", s.Path, regexp.QuoteMeta(s.Path))
	}

	diffs := diff("", s.Expected, normalized)
	if len(diffs) == 0 {
		return nil
	}
	return fmt.Errorf("snapshot mismatch %s:\n  - %s", s.Path, strings.Join(diffs, "\n  - "))
}

func (s *Snapshot) write(normalized any) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", s.Path, err)
	}
	blob, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	if err := os.WriteFile(s.Path, append(blob, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", s.Path, err)
	}
	return nil
}

func (s *Snapshot) normalize(v any) any {
	blob, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var copy any
	if err := json.Unmarshal(blob, &copy); err != nil {
		return v
	}
	reps := sortedReplacements(s.bindings)
	return walk(copy, reps)
}

func walk(v any, reps []replacement) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			out[k] = walk(val, reps)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = walk(val, reps)
		}
		return out
	case string:
		return substitute(x, reps)
	default:
		return v
	}
}

type replacement struct {
	from string
	to   string
}

// sortedReplacements returns longest-value-first so that overlapping bindings
// don't shadow each other (e.g. "ABCDEF" replaces before "ABC").
func sortedReplacements(bindings map[string]string) []replacement {
	out := make([]replacement, 0, len(bindings))
	for placeholder, value := range bindings {
		out = append(out, replacement{from: value, to: placeholder})
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i].from) > len(out[j].from) })
	return out
}

func substitute(s string, reps []replacement) string {
	for _, r := range reps {
		if r.from == "" {
			continue
		}
		s = strings.ReplaceAll(s, r.from, r.to)
	}
	return s
}

func diff(path string, expected, actual any) []string {
	if reflect.DeepEqual(expected, actual) {
		return nil
	}
	switch e := expected.(type) {
	case map[string]any:
		a, ok := actual.(map[string]any)
		if !ok {
			return []string{fmt.Sprintf("%s: expected object, got %T", displayPath(path), actual)}
		}
		var out []string
		seen := map[string]bool{}
		for k, ev := range e {
			seen[k] = true
			out = append(out, diff(joinPath(path, k), ev, a[k])...)
		}
		for k, av := range a {
			if seen[k] {
				continue
			}
			out = append(out, fmt.Sprintf("%s: unexpected key with value %s", displayPath(joinPath(path, k)), summary(av)))
		}
		return out
	case []any:
		a, ok := actual.([]any)
		if !ok {
			return []string{fmt.Sprintf("%s: expected array, got %T", displayPath(path), actual)}
		}
		if len(e) != len(a) {
			return []string{fmt.Sprintf("%s: array length differs — expected %d, actual %d", displayPath(path), len(e), len(a))}
		}
		var out []string
		for i := range e {
			out = append(out, diff(fmt.Sprintf("%s[%d]", path, i), e[i], a[i])...)
		}
		return out
	default:
		return []string{fmt.Sprintf("%s: expected %s, actual %s", displayPath(path), summary(expected), summary(actual))}
	}
}

func joinPath(p, k string) string {
	if p == "" {
		return k
	}
	return p + "." + k
}

func displayPath(p string) string {
	if p == "" {
		return "<root>"
	}
	return p
}

func summary(v any) string {
	blob, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	if len(blob) > 200 {
		return string(blob[:200]) + "…"
	}
	return string(blob)
}
