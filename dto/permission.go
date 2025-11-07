package dto

type PermissionRequest struct {
	Name    string  `json:"name" binding:"required"`
	Subject string  `json:"subject" binding:"required"`
	Action  string  `json:"action" binding:"required"`
	Disable *bool   `json:"disable"`
	Active  *string `json:"active"`
}
