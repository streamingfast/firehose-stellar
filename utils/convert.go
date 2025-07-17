package utils

import pbstellar "github.com/streamingfast/firehose-stellar/pb/sf/stellar/type/v1"

func ConvertTransactionStatus(status string) pbstellar.TransactionStatus {
	if status == "SUCCESS" {
		return pbstellar.TransactionStatus_SUCCESS
	}

	return pbstellar.TransactionStatus_FAILED
}

func ConvertTransactionEvents(events *pbstellar.Events) *pbstellar.Events {
	return nil
}
