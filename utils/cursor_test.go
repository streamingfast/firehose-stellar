package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_DecodeCursor(t *testing.T) {
	tests := []struct {
		name        string
		cursor      string
		expected    *Cursor
		expectedErr bool
	}{
		{
			name:   "valid cursor with suffix",
			cursor: "0237311318360358912-0000000005",
			expected: &Cursor{
				LedgerNumber:     55253347,
				TransactionIndex: 12,
				OperationIndex:   32768,
				Suffix:           5,
			},
		},
		{
			name:   "valid cursor without suffix",
			cursor: "237311309769850881",
			expected: &Cursor{
				LedgerNumber:     55253345,
				TransactionIndex: 3,
				OperationIndex:   49153,
				Suffix:           0,
			},
		},
		{
			name:        "invalid cursor",
			cursor:      "invalid-cursor",
			expectedErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, err := DecodeCursor(test.cursor)
			if test.expectedErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, c)
			require.Equal(t, test.expected.LedgerNumber, c.LedgerNumber)
			require.Equal(t, test.expected.TransactionIndex, c.TransactionIndex)
			require.Equal(t, test.expected.OperationIndex, c.OperationIndex)
			require.Equal(t, test.expected.Suffix, c.Suffix)
		})
	}
}
