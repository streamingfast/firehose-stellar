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
		return nil, fmt.Errorf("failed to unmarshal ledger metadata XDR: %w", err)
	}
	return &ledgerMetadata, nil
}

func (d *Decoder) DecodeTransactionEnvelope(envelopeXdr string) (*xdrTypes.TransactionEnvelope, error) {
	data, err := base64.StdEncoding.DecodeString(envelopeXdr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	var envelope xdrTypes.TransactionEnvelope
	_, err = xdr.Unmarshal(bytes.NewBuffer(data), &envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction envelope XDR: %w", err)
	}
	return &envelope, nil
}

func (d *Decoder) DecodeTransactionEnvelopeFromBytes(envelopeXdr []byte) (*xdrTypes.TransactionEnvelope, error) {
	var envelope xdrTypes.TransactionEnvelope
	_, err := xdr.Unmarshal(bytes.NewBuffer(envelopeXdr), &envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction envelope XDR: %w", err)
	}
	return &envelope, nil
}

// TODO: convert all the result types of the operations in their protobuf equivalent
func (d *Decoder) DecodeTransactionResult(resultXdr string) (*xdrTypes.TransactionResult, error) {
	data, err := base64.StdEncoding.DecodeString(resultXdr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	var result xdrTypes.TransactionResult
	_, err = xdr.Unmarshal(bytes.NewBuffer(data), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction result XDR: %w", err)
	}
	return &result, nil
}

func (d *Decoder) DecodeTransactionResultFromBytes(resultXdr []byte) (*xdrTypes.TransactionResult, error) {
	var result xdrTypes.TransactionResult
	_, err := xdr.Unmarshal(bytes.NewBuffer(resultXdr), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction result XDR: %w", err)
	}
	return &result, nil
}

func (d *Decoder) DecodeTransactionResultMeta(resultMetaXd string) (*xdrTypes.TransactionMeta, error) {
	data, err := base64.StdEncoding.DecodeString(resultMetaXd)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	var transactionMeta xdrTypes.TransactionMeta
	_, err = xdr.Unmarshal(bytes.NewBuffer(data), &transactionMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction meta XDR: %w", err)
	}
	return &transactionMeta, nil
}

func (d *Decoder) DecodeTransactionResultMetaFromBytes(resultMetaBytes []byte) (*xdrTypes.TransactionMeta, error) {
	var transactionMeta xdrTypes.TransactionMeta
	_, err := xdr.Unmarshal(bytes.NewBuffer(resultMetaBytes), &transactionMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction meta XDR: %w", err)
	}
	return &transactionMeta, nil
}
