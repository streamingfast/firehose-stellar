package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Test_Fetch(t *testing.T) {
	c := NewClient("https://mainnet.sorobanrpc.com", nil)
	ledger, err := c.GetLatestLedger()
	f := NewFetcher(time.Second, time.Second, 200, zap.NewNop())
	b, _, err := f.Fetch(context.Background(), c, uint64(ledger.Sequence))
	require.NoError(t, err)
	require.NotNil(t, b)
}
