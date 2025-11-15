package middlewares

import (
	"EventHunting/collections"
	"EventHunting/database"
	"EventHunting/utils"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	unAvailableToken = []string{"verify", "approved"}
)

func AuthorizeJWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			redisClient = database.GetRedisClient().Client
		)
		authHeader := c.GetHeader("Authorization")
		authHeader = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if authHeader == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  http.StatusBadRequest,
				"message": "Token không thấy!",
			})
			c.Abort()
			return
		}
		key := fmt.Sprintf("blacklist:accesstoken:%s", authHeader)
		result, err := redisClient.Exists(c.Request.Context(), key).Result()
		if err != nil {
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống redis blacklist!", err.Error())
			c.Abort()
			return
		}
		if result != 0 {
			utils.ResponseError(c, http.StatusUnauthorized, "Token hiện tại không dùng được!", nil)
			c.Abort()
			return
		}
		token, err := utils.ValidateToken(authHeader)
		tokenClaims, _ := utils.ExtractCustomClaims(token.Raw)
		if token.Valid {
			if slices.Contains(unAvailableToken, tokenClaims.Type) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"status":  http.StatusUnauthorized,
					"message": "Không có quyền truy cập",
				})
				c.Abort()
				return
			}
			c.Set("roles", tokenClaims.Roles)
			c.Set("account_id", tokenClaims.RegisteredClaims.Subject)
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  http.StatusUnauthorized,
				"message": err.Error(),
			})
			c.Abort()
			return
		}
	}
}

func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		authHeader = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if authHeader == "" {
			c.Next()
			return
		}
		token, err := utils.ValidateToken(authHeader)
		tokenClaims, _ := utils.ExtractCustomClaims(token.Raw)
		if token.Valid {
			if slices.Contains(unAvailableToken, tokenClaims.Type) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"status":  http.StatusUnauthorized,
					"message": "Không có quyền truy cập",
				})
				c.Abort()
				return
			}
			c.Set("roles", tokenClaims.Roles)
			c.Set("account_id", tokenClaims.RegisteredClaims.Subject)
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  http.StatusUnauthorized,
				"message": err.Error(),
			})
			c.Abort()
			return
		}
	}
}

func RBACMiddleware(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			permissionEntry = collections.Permission{}
			roleEntry       = collections.Role{}
		)

		roles, exists := c.Get("roles")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  http.StatusUnauthorized,
				"message": "Không có role",
			})
			c.Abort()
			return
		}

		checkExisted := permissionEntry.First(bson.M{
			"name": permission,
		})

		if checkExisted != nil {
			if errors.Is(checkExisted, mongo.ErrNoDocuments) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"status":  http.StatusUnauthorized,
					"message": "Không tìm thấy permission",
				})
				c.Abort()
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  http.StatusInternalServerError,
				"message": checkExisted.Error(),
			})
			c.Abort()
			return
		}

		if permissionEntry.Disable == true || permissionEntry.Active != "active" {
			c.JSON(http.StatusForbidden, bson.M{
				"status":  http.StatusForbidden,
				"message": "Tài nguyên này hiện tại đang khóa",
			})
			c.Abort()
			return
		}

		permissionId := permissionEntry.ID.Hex()

		for _, role := range roles.([]string) {
			baseFilter := bson.M{
				"name": role,
				"deleted_at": bson.M{
					"$exists": false,
				},
			}
			err := roleEntry.First(baseFilter)
			if err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					continue
				}
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  http.StatusInternalServerError,
					"message": "Lỗi do hệ thống",
					"error":   err.Error(),
				})
				c.Abort()
				return
			}
			permissionIds := []string{}
			for _, perId := range roleEntry.PermissionIds {
				permissionIds = append(permissionIds, perId.Hex())
			}

			if slices.Contains(permissionIds, permissionId) {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, bson.M{
			"status":  http.StatusForbidden,
			"message": "Không được truy cập vào tài nguyên này",
		})
		c.Abort()
	}
}
