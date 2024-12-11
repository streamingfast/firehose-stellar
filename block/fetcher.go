package block

import (
	"context"
	"time"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/firehose-stellar/rpc"
	"go.uber.org/zap"
)

type Fetcher struct {
}

func NewFetcher(time.Duration, *zap.Logger) *Fetcher {
	return &Fetcher{}
}

func (f *Fetcher) IsBlockAvailable(requestedSlot uint64) bool {
	//TODO implement me
	panic("implement me")
}

func (f *Fetcher) Fetch(ctx context.Context, client *rpc.Client, blkNum uint64) (b *pbbstream.Block, skipped bool, err error) {
	//TODO implement me
	panic("implement me")
}
