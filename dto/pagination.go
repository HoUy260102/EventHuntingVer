package dto

import "go.mongodb.org/mongo-driver/bson/primitive"

type Pagination struct {
	PageNo     int `json:"page_no"`
	PageSize   int `json:"page_size"`
	PageCount  int `json:"page_count"`
	TotalItems int `json:"total_items"`
}

type PaginationLoadMore struct {
	NextLastId  primitive.ObjectID `json:"next_last_id"`
	HasMore     bool               `json:"has_more"`
	TotalLoaded int                `json:"total_loaded"`
}
