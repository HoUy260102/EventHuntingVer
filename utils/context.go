package utils

import (
	"EventHunting/dto"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetAccountID(c *gin.Context) (primitive.ObjectID, bool) {
	updatorIDValue, exists := c.Get("account_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, dto.ApiResponse{
			Status:  http.StatusUnauthorized,
			Message: "Không tìm thấy account_id trong context!",
		})
		return primitive.NilObjectID, false
	}

	updatorIDStr, ok := updatorIDValue.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "account_id không hợp lệ (không phải string)!",
		})
		return primitive.NilObjectID, false
	}

	updatorObjectId, err := primitive.ObjectIDFromHex(updatorIDStr)
	if err != nil {

		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "account_id không đúng định dạng ObjectID!",
		})
		return primitive.NilObjectID, false
	}

	return updatorObjectId, true
}

func GetRoles(c *gin.Context) ([]string, error) {
	rolesInterface, ok := c.Get("roles")
	if !ok {
		return nil, errors.New("Không tìm thấy roles trong context")
	}

	roles, ok := rolesInterface.([]string)
	if !ok {
		return nil, errors.New("Kiểu dữ liệu roles không hợp lệ (phải là []string)")
	}

	return roles, nil
}

func GetClientIpAdrr(c *gin.Context) string {
	ipAdrr := c.ClientIP()
	if ipAdrr == "::1" {
		ipAdrr = "127.0.0.1"
	}
	return ipAdrr
}
