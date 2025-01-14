package utils

import (
	"fmt"
	"strconv"
	"strings"
)

type Cursor struct {
	LedgerNumber     uint32
	TransactionIndex uint16
	OperationIndex   uint16
	Suffix           uint64
}

func DecodeCursor(cursor string) (*Cursor, error) {
	parts := strings.Split(cursor, "-")

	// Parse the primary part (TOID)
	toid, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid TOID: %s", parts[0])
	}

	// Extract the components from the TOID
	ledgerNumber := uint32(toid >> 32)
	transactionIndex := uint16((toid >> 16) & 0xFFFF)
	operationIndex := uint16(toid & 0xFFFF)

	c := &Cursor{
		LedgerNumber:     ledgerNumber,
		TransactionIndex: transactionIndex,
		OperationIndex:   operationIndex,
	}

	var suffix uint64
	if len(parts) == 2 {
		// Parse the suffix if it exists (in the case of events)
		suffix, err = strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid suffix: %s", parts[1])
		}
		c.Suffix = suffix
	}

	return c, nil
}
