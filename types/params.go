package types

type Params struct {
	StartLedger uint64      `json:"startLedger"`
	Pagination  *Pagination `json:"pagination"`
}

func NewParams(startLedger uint64, pagination *Pagination) Params {
	return Params{
		StartLedger: startLedger,
		Pagination:  pagination,
	}
}
