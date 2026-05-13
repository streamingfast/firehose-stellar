package snapshot

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DiffViews structurally compares two arbitrary values (typically *xdr.TxView
// instances coming from different backends) and returns nil iff they are
// deep-equal. Errors include a path-prefixed list of differences to make
// backend disagreements debuggable.
//
// Unlike Snapshot.Compare, this does no $var substitution — both inputs are
// expected to already be in the same canonical form.
func DiffViews(a, b any) error {
	aMap, err := toGeneric(a)
	if err != nil {
		return fmt.Errorf("normalize a: %w", err)
	}
	bMap, err := toGeneric(b)
	if err != nil {
		return fmt.Errorf("normalize b: %w", err)
	}
	diffs := diff("", aMap, bMap)
	if len(diffs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(diffs, "\n  - "))
}

func toGeneric(v any) (any, error) {
	blob, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(blob, &out); err != nil {
		return nil, err
	}
	return out, nil
}
