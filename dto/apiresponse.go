package dto

type ApiResponse struct {
	Status             int                 `json:"status"`
	Message            string              `json:"message"`
	Error              interface{}         `json:"error,omitempty"`
	Data               interface{}         `json:"data,omitempty"`
	Pagination         *Pagination         `json:"pagination,omitempty"`
	PaginationLoadMore *PaginationLoadMore `json:"pagination_load_more,omitempty"`
}
