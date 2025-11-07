package dto

import (
	"EventHunting/consts"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CreateBlogRequest struct {
	Title             string  `json:"title"`
	Content           string  `json:"content"`
	ContentHtml       string  `json:"content_html"`
	ThumbnailLink     *string `json:"thumbnail_link"`
	ThumbnailPublicId *string `json:"thumbnail_public_id"`
	//PublicImgIds      *[]string             `json:"public_img_ids,omitempty"`
	Medias *[]struct {
		Type        consts.MediaFormat `json:"type"` // Image, Video
		Url         string             `json:"url"`
		Status      consts.MediaStatus `json:"status"` // Process, Pending, Success, Error
		PublicUrlId string             `json:"public_url_id"`
	} `json:"medias"`
	TagIds *[]primitive.ObjectID `json:"tag_ids"`
}

type UpdateBlogRequest struct {
	Title             *string               `json:"title,omitempty"`
	Content           *string               `json:"content,omitempty"`
	ThumbnailLink     *string               `json:"thumbnail_link,omitempty"`
	ThumbnailPublicId *string               `json:"thumbnail_public_id,omitempty"`
	PublicImgIds      *[]string             `json:"public_img_ids,omitempty"`
	TagIds            *[]primitive.ObjectID `json:"tag_ids,omitempty"`
}
