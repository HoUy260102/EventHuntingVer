package dto

import (
	"EventHunting/consts"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CommentCreateReq struct {
	ParentID primitive.ObjectID `json:"parent_id"`

	Content     string               `json:"content"`
	ContentHTML string               `json:"content_html"`
	MediaIds    []primitive.ObjectID `json:"media_ids"`

	DocumentID primitive.ObjectID `json:"document_id"`
	Category   consts.CommentType `json:"category"`
}

type CommentUpdateReq struct {
	Content     *string               `json:"content"`
	ContentHTML *string               `json:"content_html"`
	MediaIds    *[]primitive.ObjectID `json:"media_ids"`
}
