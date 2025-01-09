package rpc

import (
	"context"
	"fmt"
	"time"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"go.uber.org/zap"
)

type LastBlockInfo struct {
	blockNum uint64
}

type Fetcher struct {
	fetchInterval            time.Duration
	latestBlockRetryInterval time.Duration
	logger                   *zap.Logger

	lastBlockInfo *LastBlockInfo
}

func NewFetcher(fetchInterval, latestBlockRetryInterval time.Duration, logger *zap.Logger) *Fetcher {
	return &Fetcher{
		fetchInterval:            fetchInterval,
		latestBlockRetryInterval: latestBlockRetryInterval,
		logger:                   logger,
		lastBlockInfo:            &LastBlockInfo{},
	}
}

func (f *Fetcher) Fetch(ctx context.Context, client *Client, requestBlockNum uint64) (b *pbbstream.Block, skipped bool, err error) {
	fmt.Println("Requested block number", requestBlockNum)

	sleepDuration := time.Duration(0)
	for f.lastBlockInfo.blockNum < requestBlockNum {
		time.Sleep(sleepDuration)

		f.lastBlockInfo.blockNum, err = client.GetLatestLedger()
		if err != nil {
			return nil, false, fmt.Errorf("fetching latest block num: %w", err)
		}

		f.logger.Info("got latest block num", zap.Uint64("latest_block_num", f.lastBlockInfo.blockNum), zap.Uint64("requested_block_num", requestBlockNum))

		if f.lastBlockInfo.blockNum >= requestBlockNum {
			break
		}
		sleepDuration = f.latestBlockRetryInterval
	}

	return nil, false, nil
}

func (f *Fetcher) IsBlockAvailable(blockNum uint64) bool {
	return blockNum <= f.lastBlockInfo.blockNum
}
