package dto

import (
	"EventHunting/consts"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CommentCreateReq struct {
	ParentID primitive.ObjectID `json:"parent_id"`

	Content     string `json:"content"`
	ContentHTML string `json:"content_html"`
	Medias      []struct {
		Type        consts.MediaFormat `json:"type"` // Image, Video
		Url         string             `json:"url"`
		Status      consts.MediaStatus `json:"status"` // Process, Pending, Success, Error
		PublicUrlId string             `json:"public_url_id"`
	} `json:"medias"`

	BlogID   primitive.ObjectID `json:"blog_id"`
	Category consts.CommentType `json:"category"`
}

func ValidateCommentCreate(req CommentCreateReq) []string {
	var errors []string

	// Category là bắt buộc
	if req.Category == "" {
		errors = append(errors, "Loại (category) bình luận là bắt buộc.")
	}

	// Nếu Category là Blog, thì BlogID là bắt buộc
	if req.Category == consts.COMMENT_TYPE_BLOG && req.BlogID.IsZero() {
		errors = append(errors, "BlogID là bắt buộc khi bình luận cho blog.")
	}

	// Phải có Content hoặc Medias
	if strings.TrimSpace(req.Content) == "" && len(req.Medias) == 0 {
		errors = append(errors, "Bình luận phải có nội dung văn bản hoặc media.")
	}

	// Kiểm tra bên trong Medias
	if len(req.Medias) > 0 {
		for i, media := range req.Medias {
			if media.Url == "" {
				errors = append(errors, fmt.Sprintf("Media #%d bị thiếu URL.", i+1))
			}
			if media.Type == "" {
				errors = append(errors, fmt.Sprintf("Media #%d bị thiếu 'Type' (loại).", i+1))
			}
			if media.Type != consts.MEDIA_IMAGE && media.Type != consts.MEDIA_VIDEO {
				errors = append(errors, fmt.Sprintf("Media #%d 'Type' không hợp lệ.", i+1))
			}
		}
	}

	return errors
}

type CommentUpdateReq struct {
	Content     *string `json:"content"`
	ContentHTML *string `json:"content_html"`
	Medias      *[]struct {
		Type        consts.MediaFormat `json:"type"`
		Url         string             `json:"url"`
		Status      consts.MediaStatus `json:"status"`
		PublicUrlId string             `json:"public_url_id"`
	} `json:"medias"`
}

func ValidateCommentUpdate(req CommentUpdateReq) []string {
	var errors []string

	// Phải có *ít nhất một* trường được gửi lên để cập nhật
	if req.Content == nil && req.ContentHTML == nil && req.Medias == nil {
		errors = append(errors, "Phải cung cấp ít nhất một trường (content, content_html, hoặc medias) để cập nhật.")
		return errors
	}

	// Nếu gửi Content, nó không được là chuỗi rỗng
	if req.Content != nil && strings.TrimSpace(*req.Content) == "" {
		errors = append(errors, "Nội dung (content) không được là chuỗi rỗng.")
	}

	// Kiểm tra sâu bên trong Medias nếu gửi
	if req.Medias != nil && len(*req.Medias) > 0 {
		for i, media := range *req.Medias {
			if media.Url == "" {
				errors = append(errors, fmt.Sprintf("Media #%d bị thiếu URL.", i+1))
			}
			if media.Type == "" {
				errors = append(errors, fmt.Sprintf("Media #%d bị thiếu 'Type' (loại).", i+1))
			}
			if media.Type != consts.MEDIA_IMAGE && media.Type != consts.MEDIA_VIDEO {
				errors = append(errors, fmt.Sprintf("Media #%d 'Type' không hợp lệ.", i+1))
			}
		}
	}

	return errors
}
