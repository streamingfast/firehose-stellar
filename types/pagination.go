package types

type Pagination struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}
