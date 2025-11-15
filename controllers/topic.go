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

func CreateTopic(c *gin.Context) {
	topicEntry := &collections.Topic{}

	// Bind và Validate Request Body
	var createTopicRequest dto.CreateTopicRequest
	if err := c.ShouldBindJSON(&createTopicRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Dữ liệu không hợp lệ: " + err.Error()})
		return
	}

	if validationErrors := utils.ValidateTopicCreate(createTopicRequest); len(validationErrors) > 0 {
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
	escapedName := regexp.QuoteMeta(createTopicRequest.Name)
	filter := bson.M{
		"name": bson.M{
			"$regex":   "^" + escapedName + "$",
			"$options": "i",
		},
		"deleted_at": bson.M{"$exists": false},
	}

	err := topicEntry.First(filter)

	// Xử lý kết quả của First
	if err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Topic này đã tồn tại (case-insensitive)",
		})
		return
	}

	if !errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi máy chủ khi kiểm tra topic: " + err.Error(),
		})
		return
	}

	// TẠO TAG MỚI
	newTopic := &collections.Topic{
		Name:      createTopicRequest.Name,
		CreatedAt: time.Now(),
		CreatedBy: creatorObjectId,
		UpdatedAt: time.Now(),
		UpdatedBy: creatorObjectId,
	}

	if createTopicRequest.Slug != nil {
		newTopic.Slug = *createTopicRequest.Slug
	} else {
		newTopic.Slug = utils.GenerateSlug(createTopicRequest.Name)
	}

	if createTopicRequest.Description != nil {
		newTopic.Description = *createTopicRequest.Description
	}

	err = newTopic.Create()

	switch {
	case err == nil:
		c.JSON(http.StatusCreated, gin.H{
			"status":  http.StatusCreated,
			"message": "Đã tạo topic thành công",
			"entry":   utils.PrettyJSON(newTopic.ParseEntry()),
		})
	case mongo.IsDuplicateKeyError(err):
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Lỗi: Topic này đã tồn tại (Trùng lặp Key/Slug)",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi khi tạo topic: " + err.Error(),
			"error":   err.Error(),
		})
	}
}

func UpdateTopic(c *gin.Context) {
	topicEntry := &collections.Topic{}

	// Lấy ID Topic từ URL
	topicIDStr := c.Param("id")
	topicID, err := primitive.ObjectIDFromHex(topicIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "ID topic không hợp lệ"})
		return
	}

	// Bind và Validate Request Body
	var updateRequest dto.UpdateTopicRequest
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Dữ liệu không hợp lệ: " + err.Error()})
		return
	}

	if validationErrors := utils.ValidateTopicUpdate(updateRequest); len(validationErrors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": validationErrors})
		return
	}

	// Lấy ID người cập nhật
	updaterObjectID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	//Lấy roles từ context
	roles, err := utils.GetRoles(c)
	if err != nil {
		return
	}

	// KIỂM TRA TỒN TẠI
	filter := bson.M{
		"_id":        topicID,
		"deleted_at": bson.M{"$exists": false},
	}

	err = topicEntry.First(filter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "Không tìm thấy topic để cập nhật (hoặc đã bị xóa)"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi máy chủ: " + err.Error()})
		return
	}

	//Kiểm tra có phải là chính chủ hay có quyền chỉnh sửa
	if !utils.CanModifyResource(topicEntry.CreatedBy, updaterObjectID, roles) {
		utils.ResponseError(c, http.StatusForbidden, "", "Bạn không có quyền truy câp tài nguyên này!")
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

	err = topicEntry.Update(filter, bson.M{"$set": updatePayload})

	// XỬ LÝ RESPONSE BẰNG SWITCH
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":  http.StatusOK,
			"message": "Cập nhật topic thành công",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": "Không tìm thấy topic (lỗi từ Update)",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi khi cập nhật topic: " + err.Error(),
			"error":   err.Error(),
		})
	}
}

func SoftDeleteTopic(c *gin.Context) {
	topicEntry := &collections.Topic{}

	//Lấy ID Topic từ URL
	topicIDStr := c.Param("id")
	topicID, err := primitive.ObjectIDFromHex(topicIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "ID topic không hợp lệ"})
		return
	}

	// Lấy ID người xóa
	deleterObjectID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	//Lấy roles từ context
	roles, err := utils.GetRoles(c)
	if err != nil {
		return
	}

	var (
		filter = bson.M{
			"_id":        topicID,
			"deleted_at": bson.M{"$exists": false},
		}

		update = bson.M{
			"$set": bson.M{
				"deleted_at": time.Now(),
				"deleted_by": deleterObjectID,
			},
		}
	)

	err = topicEntry.First(filter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "Không tìm thấy topic để cập nhật (hoặc đã bị xóa)"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi máy chủ: " + err.Error()})
		return
	}

	//Kiểm tra có phải là chính chủ hay có quyền chỉnh sửa
	if !utils.CanModifyResource(topicEntry.CreatedBy, deleterObjectID, roles) {
		utils.ResponseError(c, http.StatusForbidden, "", "Bạn không có quyền truy câp tài nguyên này!")
		return
	}

	err = topicEntry.Update(filter, update)

	// Xử lý response
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":  http.StatusOK,
			"message": "Đã xóa topic thành công",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": "Không tìm thấy topic để xóa (hoặc đã bị xóa từ trước)",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi hệ thống khi xóa topic",
			"error":   err.Error(),
		})
	}
}

func RestoreTopic(c *gin.Context) {
	topicEntry := &collections.Topic{}

	//Lấy ID Topic từ URL
	topicIDStr := c.Param("id")
	topicID, err := primitive.ObjectIDFromHex(topicIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "ID topic không hợp lệ"})
		return
	}

	// Lấy ID người khôi phục
	updaterObjectID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	//Lấy roles từ context
	roles, err := utils.GetRoles(c)
	if err != nil {
		return
	}

	var (
		filter = bson.M{
			"_id":        topicID,
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

	err = topicEntry.First(filter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "Không tìm thấy topic để cập nhật (hoặc đã bị xóa)"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi máy chủ: " + err.Error()})
		return
	}

	//Kiểm tra có phải là chính chủ hay có quyền chỉnh sửa
	if !utils.CanModifyResource(topicEntry.CreatedBy, updaterObjectID, roles) {
		utils.ResponseError(c, http.StatusForbidden, "", "Bạn không có quyền truy câp tài nguyên này!")
		return
	}

	err = topicEntry.Update(filter, update)

	// Xử lý response bằng switch
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":  http.StatusOK,
			"message": "Đã khôi phục topic thành công",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": "Không tìm thấy topic để khôi phục (hoặc topic chưa bị xóa)",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi hệ thống khi khôi phục topic",
			"error":   err.Error(),
		})
	}
}

func FindTopic(c *gin.Context) {
	var (
		topicEntry = &collections.Topic{}
		baseFilter = bson.M{
			"deleted_at": bson.M{"$exists": false},
		}
	)
	topics, err := topicEntry.Find(baseFilter)

	switch {
	case err != nil && !errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
	case len(topics) == 0:
		utils.ResponseError(c, http.StatusNotFound, "", nil)
	default:
		utils.ResponseSuccess(c, http.StatusOK, "", topics, nil)
	}
}
