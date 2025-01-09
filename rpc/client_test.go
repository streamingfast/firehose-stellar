package rpc

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_GetLatestLedger(t *testing.T) {
	c := NewClient(os.Getenv("HORIZON_URL"), nil)
	ledger, err := c.GetLatestLedger()
	require.NoError(t, err)
	require.NotZero(t, ledger)
}
