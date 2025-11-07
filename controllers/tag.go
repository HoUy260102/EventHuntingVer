package controllers

import (
	"EventHunting/collections"
	"EventHunting/dto"
	"EventHunting/utils"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func CreateTag(c *gin.Context) {
	tagEntry := &collections.Tag{}

	// Bind và Validate Request Body
	var createTagRequest dto.CreateTagRequest
	if err := c.ShouldBindJSON(&createTagRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Dữ liệu không hợp lệ: " + err.Error()})
		return
	}

	if validationErrors := utils.ValidateTagCreate(createTagRequest); len(validationErrors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": validationErrors,
		})
		return
	}

	// Lấy ID người tạo
	creatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	// KIỂM TRA TRÙNG LẶP
	escapedName := regexp.QuoteMeta(createTagRequest.Name)
	filter := bson.M{
		"name": bson.M{
			"$regex":   "^" + escapedName + "$",
			"$options": "i",
		},
		"deleted_at": bson.M{"$exists": false}, // FIX: Chỉ kiểm tra tag chưa bị xóa
	}

	err := tagEntry.First(filter)

	// Xử lý kết quả của First
	if err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Tag này đã tồn tại (case-insensitive)",
		})
		return
	}

	if !errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi máy chủ khi kiểm tra tag: " + err.Error(),
		})
		return
	}

	// TẠO TAG MỚI
	newTag := &collections.Tag{
		Name:      createTagRequest.Name,
		CreatedAt: time.Now(),
		CreatedBy: creatorObjectId,
		UpdatedAt: time.Now(),
		UpdatedBy: creatorObjectId,
	}

	if createTagRequest.Slug != nil {
		newTag.Slug = *createTagRequest.Slug
	} else {
		newTag.Slug = utils.GenerateSlug(createTagRequest.Name)
	}

	if createTagRequest.Description != nil {
		newTag.Description = *createTagRequest.Description
	}

	err = newTag.Create()

	switch {
	case err == nil:
		c.JSON(http.StatusCreated, gin.H{
			"status":  http.StatusCreated,
			"message": "Đã tạo tag thành công",
			"entry":   utils.PrettyJSON(newTag.ParseEntry()),
		})
	case mongo.IsDuplicateKeyError(err):
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Lỗi: Tag này đã tồn tại (Trùng lặp Key/Slug)",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi khi tạo tag: " + err.Error(),
			"error":   err.Error(),
		})
	}
}

func UpdateTag(c *gin.Context) {
	tagEntry := &collections.Tag{}

	// Lấy ID Tag từ URL
	tagIDStr := c.Param("id")
	tagID, err := primitive.ObjectIDFromHex(tagIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "ID tag không hợp lệ"})
		return
	}

	// Bind và Validate Request Body
	var updateRequest dto.UpdateTagRequest
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Dữ liệu không hợp lệ: " + err.Error()})
		return
	}

	if validationErrors := utils.ValidateTagUpdate(updateRequest); len(validationErrors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": validationErrors})
		return
	}

	// Lấy ID người cập nhật
	updaterObjectID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	// KIỂM TRA TỒN TẠI
	filter := bson.M{
		"_id":        tagID,
		"deleted_at": bson.M{"$exists": false},
	}

	err = tagEntry.First(filter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "Không tìm thấy tag để cập nhật (hoặc đã bị xóa)"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi máy chủ: " + err.Error()})
		return
	}

	// XÂY DỰNG PAYLOAD UPDATE
	updatePayload := bson.M{}
	if updateRequest.Name != nil {
		updatePayload["name"] = *updateRequest.Name

		if updateRequest.Slug == nil || *updateRequest.Slug == "" {
			updatePayload["slug"] = utils.GenerateSlug(*updateRequest.Name)
		}
	}

	if updateRequest.Description != nil {
		updatePayload["description"] = *updateRequest.Description
	}

	// Nếu slug được cung cấp
	if updateRequest.Slug != nil && *updateRequest.Slug != "" {
		updatePayload["slug"] = *updateRequest.Slug
	}

	if len(updatePayload) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Không có thông tin nào để cập nhật"})
		return
	}

	updatePayload["updated_at"] = time.Now()
	updatePayload["updated_by"] = updaterObjectID

	err = tagEntry.Update(filter, bson.M{"$set": updatePayload})

	// XỬ LÝ RESPONSE BẰNG SWITCH
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":  http.StatusOK,
			"message": "Cập nhật tag thành công",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": "Không tìm thấy tag (lỗi từ Update)",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi khi cập nhật tag: " + err.Error(),
			"error":   err.Error(),
		})
	}
}

func SoftDeleteTag(c *gin.Context) {
	tagEntry := &collections.Tag{}

	//Lấy ID Tag từ URL
	tagIDStr := c.Param("id")
	tagID, err := primitive.ObjectIDFromHex(tagIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "ID tag không hợp lệ"})
		return
	}

	// Lấy ID người xóa
	deleterObjectID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	var (
		filter = bson.M{
			"_id":        tagID,
			"deleted_at": bson.M{"$exists": false},
		}

		update = bson.M{
			"$set": bson.M{
				"deleted_at": time.Now(),
				"deleted_by": deleterObjectID,
			},
		}
	)

	err = tagEntry.Update(filter, update)

	// Xử lý response
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":  http.StatusOK,
			"message": "Đã xóa tag thành công",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": "Không tìm thấy tag để xóa (hoặc đã bị xóa từ trước)",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi hệ thống khi xóa tag",
			"error":   err.Error(),
		})
	}
}

func RestoreTag(c *gin.Context) {
	tagEntry := &collections.Tag{}

	//Lấy ID Tag từ URL
	tagIDStr := c.Param("id")
	tagID, err := primitive.ObjectIDFromHex(tagIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "ID tag không hợp lệ"})
		return
	}

	// Lấy ID người khôi phục
	updaterObjectID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	var (
		filter = bson.M{
			"_id":        tagID,
			"deleted_at": bson.M{"$exists": true},
		}
		update = bson.M{
			"$unset": bson.M{
				"deleted_at": "",
				"deleted_by": "",
			},
			"$set": bson.M{
				"updated_at": time.Now(),
				"updated_by": updaterObjectID,
			},
		}
	)

	err = tagEntry.Update(filter, update)

	// Xử lý response bằng switch
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":  http.StatusOK,
			"message": "Đã khôi phục tag thành công",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": "Không tìm thấy tag để khôi phục (hoặc tag chưa bị xóa)",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi hệ thống khi khôi phục tag",
			"error":   err.Error(),
		})
	}
}
