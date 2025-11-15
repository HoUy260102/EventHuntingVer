package dto

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CreateBlogRequest struct {
	Title       string                `json:"title"`
	Content     string                `json:"content"`
	ContentHtml string                `json:"content_html"`
	ThumbnailID *primitive.ObjectID   `json:"thumbnail_id"`
	MediaIDs    *[]primitive.ObjectID `json:"media_ids"`
	TagIds      *[]primitive.ObjectID `json:"tag_ids"`
}

type UpdateBlogRequest struct {
	Title       *string               `json:"title,omitempty"`
	Content     *string               `json:"content,omitempty"`
	ContentHtml *string               `json:"content_html"`
	ThumbnailID *primitive.ObjectID   `json:"thumbnail_id,omitempty"`
	MediaIDs    *[]primitive.ObjectID `json:"media_ids,omitempty"`
	TagIds      *[]primitive.ObjectID `json:"tag_ids,omitempty"`
}
