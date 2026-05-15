// Package cursor persists the last fired block to disk so a fetcher can
// resume across restarts. The on-disk schema (cursor.json) matches
// firehose-core/blockpoller, so a single state-dir can be shared
// between fetchers that use this package and ones backed by the
// upstream blockpoller. Stellar is final at close, so Lib ==
// LastFiredBlock and the Blocks fork-history slice stays empty.
package cursor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
)

type BlockRef struct {
	Id  string `json:"id"`
	Num uint64 `json:"num"`
}

type BlockRefWithPrev struct {
	BlockRef
	PrevBlockId string `json:"previous_ref_id"`
}

// State is the on-disk cursor schema. The outer fields intentionally
// carry no JSON tags so they serialize as Lib / LastFiredBlock / Blocks,
// matching firehose-core/blockpoller's unexported stateFile struct
// byte-for-byte. Adding snake_case tags here would silently break the
// shared --state-dir compatibility we advertise in the README.
type State struct {
	Lib            BlockRef
	LastFiredBlock BlockRefWithPrev
	Blocks         []BlockRefWithPrev
}

const fileName = "cursor.json"

func path(stateDir string) string {
	return filepath.Join(stateDir, fileName)
}

// Load returns the persisted state, or (nil, nil) if stateDir is empty
// or the file does not yet exist.
func Load(stateDir string) (*State, error) {
	if stateDir == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path(stateDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read cursor: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("decode cursor: %w", err)
	}
	return &s, nil
}

// Save records blk as the last fired block. No-op when stateDir is
// empty.
func Save(stateDir string, blk *pbbstream.Block) error {
	if stateDir == "" {
		return nil
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}
	s := State{
		Lib: BlockRef{Id: blk.Id, Num: blk.Number},
		LastFiredBlock: BlockRefWithPrev{
			BlockRef:    BlockRef{Id: blk.Id, Num: blk.Number},
			PrevBlockId: blk.ParentId,
		},
		Blocks: []BlockRefWithPrev{},
	}
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal cursor: %w", err)
	}
	if err := os.WriteFile(path(stateDir), data, 0o644); err != nil {
		return fmt.Errorf("write cursor: %w", err)
	}
	return nil
}
