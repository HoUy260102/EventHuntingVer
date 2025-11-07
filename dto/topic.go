package dto

type CreateTopicRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Slug        *string `json:"slug"`
}

type UpdateTopicRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Slug        *string `json:"slug"`
}
