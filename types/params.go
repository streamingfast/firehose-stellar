package types

type Params struct {
	StartLedger uint64      `json:"startLedger"`
	Pagination  *Pagination `json:"pagination"`
}

func NewParams(startLedger uint64, pagination *Pagination) Params {
	params := Params{
		Pagination: pagination,
	}

	if pagination.Cursor == "" {
		params.StartLedger = startLedger
	}
	return params
}
