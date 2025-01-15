package rpc

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Test_Fetch(t *testing.T) {
	c := NewClient(os.Getenv("HORIZON_URL"), nil)
	f := NewFetcher(time.Second, time.Second, zap.NewNop())
	b, _, err := f.Fetch(context.Background(), c, 600000)
	require.NoError(t, err)
	require.NotNil(t, b)
}
