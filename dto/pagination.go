package dto

import (
	"EventHunting/configs"

	"github.com/gin-gonic/gin"
)

type Pagination struct {
	Page      int `form:"page" json:"page,omitempty"`
	Length    int `form:"length" json:"length,omitempty"`
	Total     int `json:"total,omitempty"`
	TotalDocs int `json:"total_docs,omitempty"`

	LastId  string `form:"last_id" json:"last_id,omitempty"`
	HasMore bool   `json:"has_more,omitempty"`
}

// primaryPagination: length = 12, max_length = 50
// secondaryPagination: length: 15, max_length = 100 --> config

func GetPagination(c *gin.Context, typePagination string) Pagination {
	var request Pagination
	_ = c.BindQuery(&request)

	defaultLength, defaultMaxLength := configs.GetDefaultPagination(typePagination)

	if request.Length <= 0 {
		request.Length = defaultLength
	}

	if request.Length > defaultMaxLength {
		request.Length = defaultMaxLength
	}

	if request.Page <= 0 {
		request.Page = 1
	}

	return request

}

func (u *Pagination) BuildPagination() {
	if u.LastId != "" {
		if u.TotalDocs > u.Length {
			u.HasMore = true
		}
		
		u.Length = 0
		u.TotalDocs = 0
		u.Page = 0
		return
	}

	totalPages := (u.TotalDocs + u.Length - 1) / (u.Length)
	u.Total = totalPages
}
