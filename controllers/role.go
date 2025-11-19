package controllers

import (
	"EventHunting/collections"
	"EventHunting/utils"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type CreateRoleInput struct {
	Name          string               `json:"name" binding:"required"`
	Status        string               `json:"status"`
	PermissionIds []primitive.ObjectID `json:"permission_ids"`
}

func AssignPermissionsToRole(c *gin.Context) {
	var (
		roleCollection     = &collections.Role{}
		roleId             string
		roleObjectId       primitive.ObjectID
		err                error
		permissionRequests []primitive.ObjectID
		updatorObjectId    primitive.ObjectID
		ok                 bool
		filter             bson.M
		update             bson.M
	)

	// Lấy ID từ URL và validate
	roleId = c.Param("id")
	roleObjectId, err = primitive.ObjectIDFromHex(roleId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Lỗi: ID của role không hợp lệ",
		})
		return
	}

	// Bind JSON body
	if err = c.BindJSON(&permissionRequests); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	// Role tồn tại
	err = roleCollection.First(bson.M{"_id": roleObjectId})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  http.StatusBadRequest,
				"message": "Không tìm thấy role",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  http.StatusInternalServerError,
				"message": err.Error(),
			})
		}
		return
	}

	// Lấy accountId từ context
	updatorObjectId, ok = utils.GetAccountID(c)
	if !ok {
		return
	}

	filter = bson.M{
		"_id":        roleCollection.Id,
		"deleted_at": bson.M{"$exists": false},
	}
	update = bson.M{
		"$addToSet": bson.M{"permission_ids": bson.M{"$each": permissionRequests}},
		"$set":      bson.M{"updated_at": time.Now(), "updated_by": updatorObjectId},
	}

	err = roleCollection.UpdateOne(filter, update)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":     http.StatusOK,
			"timestamps": time.Now(),
			"message":    "Thêm quyền thành công",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": "Không tìm thấy role để cập nhật hoặc đã bị xóa",
		})
	default:
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi từ hệ thống",
			"error":   err.Error(),
		})
	}
}

func RemovePermissionFromRole(c *gin.Context) {
	var (
		roleId             string
		roleObjectId       primitive.ObjectID
		err                error
		roleCollection     = &collections.Role{}
		permissionRequests []primitive.ObjectID
		updatorObjectId    primitive.ObjectID
		ok                 bool
		filter             bson.M
		update             bson.M
	)

	//Lấy ID
	roleId = c.Param("id")
	roleObjectId, err = primitive.ObjectIDFromHex(roleId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Lỗi: ID của role không hợp lệ"})
		return
	}

	// Bind JSON
	if err = c.BindJSON(&permissionRequests); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": err.Error()})
		return
	}

	// Role tồn tại
	err = roleCollection.First(bson.M{"_id": roleObjectId})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Không tìm thấy role"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": err.Error()})
		}
		return
	}

	// Lấy ID người update
	updatorObjectId, ok = utils.GetAccountID(c)
	if !ok {
		return
	}

	filter = bson.M{
		"_id":        roleObjectId,
		"deleted_at": bson.M{"$exists": false},
	}
	update = bson.M{
		"$pull": bson.M{"permission_ids": bson.M{"$in": permissionRequests}},
		"$set":  bson.M{"updated_at": time.Now(), "updated_by": updatorObjectId},
	}

	//Update
	err = roleCollection.UpdateOne(filter, update)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":     http.StatusOK,
			"timestamps": time.Now(),
			"message":    "Xóa quyền khỏi role thành công",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": "Không tìm thấy role để cập nhật (có thể đã bị xóa)",
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi từ hệ thống",
			"error":   err.Error(),
		})
	}
}

func GetPermissionFromRole(c *gin.Context) {
	var (
		roleId               string
		roleObjectId         primitive.ObjectID
		err                  error
		roleCollection       = &collections.Role{}
		permissionCollection = &collections.Permission{}
		permissions          collections.Permissions
	)

	// Lấy ID
	roleId = c.Param("id")
	roleObjectId, err = primitive.ObjectIDFromHex(roleId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Lỗi: ID của role không hợp lệ"})
		return
	}

	// Role tồn tại
	err = roleCollection.First(utils.GetFilter(bson.M{"_id": roleObjectId}))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "Không tìm thấy role"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi do hệ thống", "error": err.Error()})
		}
		return
	}

	permissions, err = permissionCollection.Find(utils.GetFilter(bson.M{
		"_id": bson.M{"$in": roleCollection.PermissionIds},
	}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi từ hệ thống",
			"error":   "Lỗi khi tìm kiếm permissions: " + err.Error(),
		})
		return
	}

	if len(permissions) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": "Không tìm thấy quyền nào (permissions) trong role này",
		})
		return
	}

	permissionsRespone := []bson.M{}
	for _, per := range permissions {
		permissionsRespone = append(permissionsRespone, utils.PrettyJSON(per.ParseEntry()))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     http.StatusOK,
		"timestamps": time.Now(),
		"message":    "Thành công.",
		"data":       permissionsRespone,
	})
}

func SoftDeleteRole(c *gin.Context) {
	var (
		roleCollection  = &collections.Role{}
		roleId          string
		roleObjectId    primitive.ObjectID
		err             error
		filterCheck     bson.M
		updatorObjectId primitive.ObjectID
		ok              bool
		filterUpdate    bson.M
		update          bson.M
	)

	// Lấy ID
	roleId = c.Param("id")
	roleObjectId, err = primitive.ObjectIDFromHex(roleId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Lỗi: ID của role không hợp lệ!", "error": err.Error()})
		return
	}

	// Role tồn tại và chưa bị xóa
	filterCheck = bson.M{
		"_id":        roleObjectId,
		"deleted_at": bson.M{"$exists": false},
	}
	err = roleCollection.First(filterCheck)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "Không tìm thấy role hoặc role này đã bị xóa!", "error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi từ hệ thống!", "error": err.Error()})
		}
		return
	}

	// Lấy ID người update
	updatorObjectId, ok = utils.GetAccountID(c)
	if !ok {
		return
	}

	filterUpdate = bson.M{"_id": roleObjectId}
	update = bson.M{
		"$set": bson.M{
			"status":     "deleted",
			"deleted_at": time.Now(),
			"deleted_by": updatorObjectId,
			"updated_at": time.Now(),
			"updated_by": updatorObjectId,
		},
	}

	// Update
	err = roleCollection.UpdateOne(filterUpdate, update)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":  http.StatusOK,
			"message": "Xóa mềm role thành công.",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": "Không tìm thấy role để xóa!",
		})
	default:
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   err.Error(),
		})
	}
}

func RestoreRole(c *gin.Context) {
	var (
		roleCollection  = &collections.Role{}
		roleId          string
		roleObjectId    primitive.ObjectID
		err             error
		filterCheck     bson.M
		updatorObjectId primitive.ObjectID
		ok              bool
		filterUpdate    bson.M
		update          bson.M
	)

	// Lấy ID
	roleId = c.Param("id")
	roleObjectId, err = primitive.ObjectIDFromHex(roleId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Lỗi: ID của role không hợp lệ", "error": err.Error()})
		return
	}

	// Role tồn tại và đã bị xóa
	filterCheck = bson.M{
		"_id":        roleObjectId,
		"deleted_at": bson.M{"$exists": true},
	}
	err = roleCollection.First(filterCheck)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "Không tìm thấy role hoặc role này không bị xóa!", "error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi từ hệ thống!", "error": err.Error()})
		}
		return
	}

	// Lấy ID người update
	updatorObjectId, ok = utils.GetAccountID(c)
	if !ok {
		return
	}

	filterUpdate = bson.M{"_id": roleObjectId}
	update = bson.M{
		"$unset": bson.M{"deleted_at": "", "deleted_by": ""},
		"$set":   bson.M{"status": "active", "updated_at": time.Now(), "updated_by": updatorObjectId},
	}

	// Update
	err = roleCollection.UpdateOne(filterUpdate, update)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":     http.StatusOK,
			"timestamps": time.Now(),
			"message":    "Khôi phục role thành công.",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": "Không tìm thấy role để khôi phục!",
		})
	default:
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   err.Error(),
		})
	}
}

func CreateRole(c *gin.Context) {
	var (
		roleCollection  = &collections.Role{}
		input           CreateRoleInput
		err             error
		filterCheck     bson.M
		accountObjectId primitive.ObjectID
		ok              bool
		status          string
		permissionIds   []primitive.ObjectID
		newRole         collections.Role
	)

	// Bind JSON
	if err = c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Lỗi khi bind dữ liệu!", "error": err.Error()})
		return
	}

	// Tên role tồn tại
	filterCheck = bson.M{
		"name":       input.Name,
		"deleted_at": bson.M{"$exists": false},
	}
	err = roleCollection.First(filterCheck)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"status": http.StatusConflict, "message": "Tên role này đã tồn tại!"})
		return
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi từ hệ thống!", "error": err.Error()})
		return
	}

	// Lấy ID người tạo
	accountObjectId, ok = utils.GetAccountID(c)
	if !ok {
		return
	}

	// Chuẩn bị dữ liệu
	status = input.Status
	if status == "" {
		status = "active"
	}
	permissionIds = input.PermissionIds
	if permissionIds == nil {
		permissionIds = []primitive.ObjectID{}
	}

	newRole = collections.Role{
		Name:          input.Name,
		Status:        status,
		PermissionIds: permissionIds,
		CreatedAt:     time.Now(),
		CreatedBy:     accountObjectId,
		UpdatedAt:     time.Now(),
		UpdatedBy:     accountObjectId,
	}

	// Create
	err = newRole.Create()
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, gin.H{
			"status":     http.StatusCreated,
			"timestamps": time.Now(),
			"message":    "Tạo role mới thành công.",
			"entry":      newRole,
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi hệ thống!",
			"error":   err.Error(),
		})
	}
}
