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
		"deleted_at": bson.M{"$exists": false},
	}

	err := tagEntry.First(filter)

	// Xử lý kết quả
	if err == nil {
		utils.ResponseError(c, http.StatusConflict, "Lỗi: Tag này đã tồn tại (Trùng lặp Key/Slug)!", nil)
		return
	}

	if !errors.Is(err, mongo.ErrNoDocuments) {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
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
		utils.ResponseSuccess(c, http.StatusOK, "", utils.PrettyJSON(newTag.ParseEntry()), nil)
	case mongo.IsDuplicateKeyError(err):
		utils.ResponseError(c, http.StatusConflict, "Lỗi: Tag này đã tồn tại (Trùng lặp Key/Slug)!", err.Error())
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
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
			utils.ResponseError(c, http.StatusNotFound, "Không tìm thấy tag để cập nhật (hoặc đã bị xóa)!", err.Error())
			return
		}
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
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
		utils.ResponseSuccess(c, http.StatusOK, "", nil, nil)
	case errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusNotFound, "Không tìm thấy tag (lỗi từ Update)!", err.Error())
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
	}
}

func SoftDeleteTag(c *gin.Context) {
	tagEntry := &collections.Tag{}

	//Lấy ID Tag từ URL
	tagIDStr := c.Param("id")
	tagID, err := primitive.ObjectIDFromHex(tagIDStr)
	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "ID tag không hợp lệ!", err.Error())
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
		utils.ResponseSuccess(c, http.StatusOK, "Đã xóa tag thành công.", nil, nil)
	case errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusNotFound, "Không tìm thấy tag để khôi phục (hoặc tag chưa bị xóa)!", err.Error())
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
	}
}

func RestoreTag(c *gin.Context) {
	tagEntry := &collections.Tag{}

	//Lấy ID Tag từ URL
	tagIDStr := c.Param("id")
	tagID, err := primitive.ObjectIDFromHex(tagIDStr)
	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "ID tag không hợp lệ!", err.Error())
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

	switch {
	case err == nil:
		utils.ResponseSuccess(c, http.StatusOK, "Đã khôi phục tag thành công.", nil, nil)
	case errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusNotFound, "Không tìm thấy tag để khôi phục (hoặc tag chưa bị xóa).", err.Error())
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
	}
}

func FindTag(c *gin.Context) {
	var (
		tagEntry   = &collections.Tag{}
		baseFilter = bson.M{
			"deleted_at": bson.M{"$exists": false},
		}
	)
	tags, err := tagEntry.Find(baseFilter)

	switch {
	case err != nil && !errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
	case len(tags) == 0:
		utils.ResponseError(c, http.StatusNotFound, "", nil)
	default:
		utils.ResponseSuccess(c, http.StatusOK, "", tags, nil)
	}
}
