package types

type Pagination struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}

func NewPagination(limit int, cursor string) *Pagination {
	return &Pagination{
		Limit:  limit,
		Cursor: cursor,
	}
}
