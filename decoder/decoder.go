package decoder

import (
	"bytes"
	"encoding/base64"
	"fmt"

	xdr "github.com/stellar/go-xdr/xdr3"
	xdrTypes "github.com/stellar/go/xdr"

	"go.uber.org/zap"
)

type Decoder struct {
	logger *zap.Logger
}

func NewDecoder(logger *zap.Logger) *Decoder {
	return &Decoder{
		logger: logger,
	}
}

func (d *Decoder) DecodeLedgerMetadata(metadataXdr string) (*xdrTypes.LedgerCloseMeta, error) {
	data, err := base64.StdEncoding.DecodeString(metadataXdr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	var ledgerMetadata xdrTypes.LedgerCloseMeta
	_, err = xdr.Unmarshal(bytes.NewBuffer(data), &ledgerMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal XDR: %w", err)
	}
	return &ledgerMetadata, nil
}
