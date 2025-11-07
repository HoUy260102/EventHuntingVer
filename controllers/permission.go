package controllers

import (
	"EventHunting/collections"
	"EventHunting/dto"
	"EventHunting/utils"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CreatePermission(c *gin.Context) {
	var (
		permissionRequest dto.PermissionRequest
		permissionEntry   = &collections.Permission{}
	)
	if err := c.ShouldBindJSON(&permissionRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	checkFilter := bson.M{
		"subject": permissionRequest.Subject,
		"action":  permissionRequest.Action,
	}

	checkExistedErr := permissionEntry.First(checkFilter)

	if checkExistedErr == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Permission đã tồn tại!",
		})
		return
	}

	if !errors.Is(checkExistedErr, mongo.ErrNoDocuments) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi khi kiểm tra permission: " + checkExistedErr.Error(),
		})
		return
	}

	creatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	var activeStatus string
	if permissionRequest.Active == nil {
		activeStatus = "active"
	} else {
		activeStatus = *permissionRequest.Active
	}

	var disableStatus bool
	if permissionRequest.Disable == nil {
		disableStatus = true
	} else {
		disableStatus = *permissionRequest.Disable
	}

	newPermission := collections.Permission{
		Name:      permissionRequest.Name,
		Subject:   permissionRequest.Subject,
		Action:    permissionRequest.Action,
		Active:    activeStatus,
		Disable:   disableStatus,
		CreatedAt: time.Now(),
		CreatedBy: creatorObjectId,
		UpdatedAt: time.Now(),
		UpdatedBy: creatorObjectId,
	}

	err := newPermission.Create()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":     http.StatusCreated,
		"timestamps": time.Now(),
		"message":    "Quyền đã được tạo.",
		"entry":      newPermission,
	})
}

func LockPermission(c *gin.Context) {
	permissionId := c.Param("id")
	permissionObjectId, err := primitive.ObjectIDFromHex(permissionId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "ID không hợp lệ"})
		return
	}

	updatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	permissionEntry := &collections.Permission{}
	filter := bson.M{"_id": permissionObjectId}

	checkExisted := permissionEntry.First(filter)

	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  http.StatusNotFound,
				"message": "Không tìm thấy permission!",
				"error":   checkExisted.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   checkExisted.Error(),
		})
		return
	}

	// Kiểm tra trạng thái hiện tại
	if permissionEntry.Active == "inactive" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Permission đã bị khóa từ trước!",
		})
		return
	}

	// Cập nhật
	update := bson.M{
		"$set": bson.M{
			"active":     "inactive",
			"updated_at": time.Now(),
			"updated_by": updatorObjectId,
		},
	}

	err = permissionEntry.UpdateOne(filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  http.StatusOK,
		"message": "Đã khóa thành công.",
	})
}

func UnLockPermission(c *gin.Context) {
	permissionId := c.Param("id")
	permissionObjectId, err := primitive.ObjectIDFromHex(permissionId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "ID không hợp lệ!"})
		return
	}

	updatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	permissionEntry := &collections.Permission{}
	filter := bson.M{"_id": permissionObjectId}

	checkExisted := permissionEntry.First(filter)
	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  http.StatusNotFound,
				"message": "Không tìm thấy permission!",
				"error":   checkExisted.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   checkExisted.Error(),
		})
		return
	}

	if permissionEntry.Active == "active" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Permission này hiện tại chưa được khóa!",
		})
		return
	}

	// Cập nhật
	update := bson.M{
		"$set": bson.M{
			"active":     "active",
			"updated_at": time.Now(),
			"updated_by": updatorObjectId,
		},
	}

	err = permissionEntry.UpdateOne(filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  http.StatusOK,
		"message": "Đã mở khóa thành công",
	})
}

func DisablePermission(c *gin.Context) {
	permissionId := c.Param("id")
	permissionObjectId, err := primitive.ObjectIDFromHex(permissionId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "ID không hợp lệ!"})
		return
	}

	updatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	permissionEntry := &collections.Permission{}
	filter := bson.M{"_id": permissionObjectId}

	// 1. Tìm
	checkExisted := permissionEntry.First(filter)
	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  http.StatusNotFound,
				"message": "Không tìm thấy permission!",
				"error":   checkExisted.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   checkExisted.Error(),
		})
		return
	}

	// 2. Kiểm tra trạng thái
	if permissionEntry.Disable == true {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Permission đã bị vô hiệu từ trước",
		})
		return
	}

	// 3. Cập nhật
	update := bson.M{
		"$set": bson.M{
			"disable":    true,
			"updated_at": time.Now(),
			"updated_by": updatorObjectId,
		},
	}

	err = permissionEntry.UpdateOne(filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":   http.StatusInternalServerError,
			"messaage": "Lỗi do hệ thống!",
			"error":    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  http.StatusOK,
		"message": "Đã vô hiệu thành công.",
	})
}

func EnablePermission(c *gin.Context) {
	permissionId := c.Param("id")
	permissionObjectId, err := primitive.ObjectIDFromHex(permissionId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "ID không hợp lệ!"})
		return
	}

	updatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	permissionEntry := &collections.Permission{}
	filter := bson.M{"_id": permissionObjectId}

	// Tìm
	checkExisted := permissionEntry.First(filter)
	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  http.StatusNotFound,
				"message": "Không tìm thấy permission!",
				"error":   checkExisted.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   checkExisted.Error(),
		})
		return
	}

	// Kiểm tra trạng thái
	if permissionEntry.Disable == false {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Permission này hiện tại đang được kích hoạt!",
		})
		return
	}

	// Cập nhật
	update := bson.M{
		"$set": bson.M{
			"disable":    false,
			"updated_at": time.Now(),
			"updated_by": updatorObjectId,
		},
	}

	err = permissionEntry.UpdateOne(filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  http.StatusOK,
		"message": "Đã kích hoạt thành công.",
	})
}

func FindPermissions(c *gin.Context) {
	// Không cần ctx, vì model tự tạo
	queryMap := c.Request.URL.Query()

	// Lấy 'page'
	pageStr := c.DefaultQuery("page", "1")
	page, err := strconv.ParseInt(pageStr, 10, 64)
	if err != nil || page < 1 {
		page = 1
	}

	// Lấy 'limit'
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit < 1 {
		limit = 10
	}

	skip := (page - 1) * limit

	filter := bson.M{
		"disable": false,
	}

	dynamicFilter := utils.BuildPermissionSearchFilter(queryMap)
	for key, value := range dynamicFilter {
		filter[key] = value
	}

	findOptions := options.Find()
	findOptions.SetLimit(limit)
	findOptions.SetSkip(skip)
	findOptions.SetSort(bson.D{{"created_at", 1}})

	permissionEntry := &collections.Permission{}

	permissions, err := permissionEntry.Find(filter, findOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   err.Error(),
		})
		return
	}

	if len(permissions) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": "Không có permission nào phù hợp!",
		})
		return
	}

	totalDocs, err := permissionEntry.CountDocuments(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống: Lỗi khi đếm!",
			"error":   err.Error(),
		})
		return
	}

	// Tính toán tổng số trang
	totalPages := int64(math.Ceil(float64(totalDocs) / float64(limit)))

	c.JSON(http.StatusOK, gin.H{
		"status":  http.StatusOK,
		"message": "Thành công.",
		"data":    permissions,
		"pagination": gin.H{
			"current_page": page,
			"per_page":     limit,
			"total_pages":  totalPages,
			"total_items":  totalDocs,
		},
	})
}
