package controllers

import (
	"EventHunting/collections"
	"EventHunting/configs"
	"EventHunting/dto"
	"EventHunting/utils"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	loginFailedTime     = 5 * time.Minute
	maxLoginFailedCount = 5
)

func Login(c *gin.Context, redisClient *redis.Client) {
	var (
		loginRequest dto.LoginRequest
		ctx, cancel  = context.WithTimeout(context.Background(), 10*time.Second)
	)
	defer cancel()

	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	if err := dto.ValidateLoginRequest(loginRequest); len(err) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": err,
		})
		return
	}

	// Khởi tạo collection
	accountCollection := &collections.Account{}
	existedAccount := &collections.Account{}
	checkExisted := existedAccount.First(bson.M{
		"email": loginRequest.Email,
	})

	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  http.StatusBadRequest,
				"message": "Tài khoản đăng nhập hoặc mật khẩu không chính xác",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": checkExisted.Error(),
		})
		return
	}
	if existedAccount.Provider == "google" && existedAccount.Password == "" {
		c.JSON(http.StatusBadRequest, bson.M{
			"status":  http.StatusBadRequest,
			"message": "Hãy đặt lại mật khẩu trước khi đăng nhập",
		})
		return
	}

	if existedAccount.IsActive == false {
		c.JSON(http.StatusForbidden, gin.H{
			"status":   http.StatusForbidden,
			"message.": "Tài khoản của bạn đã bị vô hiệu hóa. Vui lòng liên hệ quản trị viên.",
		})
		return
	}

	if existedAccount.IsLocked == true {
		if existedAccount.LockUtil.IsZero() {
			c.JSON(http.StatusForbidden, gin.H{
				"status":  http.StatusForbidden,
				"message": "Tài khoản đã bị khóa vĩnh viễn do: " + existedAccount.LockMessage,
			})
			return
		}
		if existedAccount.LockUtil.After(time.Now()) {
			c.JSON(http.StatusForbidden, gin.H{
				"status":  http.StatusForbidden,
				"message": "Tài khoản bị khóa tạm thời do: " + existedAccount.LockMessage,
			})
			return
		}
	}

	//Kiểm tra mật khẩu, và set redis cho tài khoản đăng nhập sai
	loginFailedPrefix := fmt.Sprintf("login_failed:%s:%s", c.ClientIP(), loginRequest.Email)
	if !utils.CheckPassword(existedAccount.Password, loginRequest.Password) {
		// Sử dụng redisClient được truyền vào
		count, err := redisClient.Incr(ctx, loginFailedPrefix).Result()
		if err != nil {
			log.Println("Redis INCR error:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
			return
		}

		// Set TTL chỉ khi key mới được tạo
		if count == 1 {
			err := redisClient.Expire(ctx, loginFailedPrefix, loginFailedTime).Err()
			if err != nil {
				log.Println("Redis EXPIRE error:", err)
			}
		}

		// Kiểm tra số lần login fail
		if count > int64(maxLoginFailedCount) {
			err := accountCollection.Update(bson.M{
				"email": loginRequest.Email,
			}, bson.M{
				"$set": bson.M{
					"is_locked":    true,
					"lock_at":      time.Now(),
					"lock_util":    time.Now().Add(loginFailedTime),
					"lock_message": "Đăng nhập quá nhiều lần vui lòng thử lại sau, " + time.Now().Add(loginFailedTime).Format("2006-01-02 15:04:05"),
				},
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  http.StatusInternalServerError,
					"message": err.Error(),
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  http.StatusBadRequest,
				"message": "Đăng nhập quá nhiều lần vui lòng thử lại sau" + time.Now().Add(loginFailedTime).Format("2006-01-02 15:04:05"),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Tài khoản đăng nhập hoặc mật khẩu không chính xác",
		})
		return
	}

	//Set lại cài đặt nếu người dùng đăng nhập đúng
	if existedAccount.IsLocked == true {
		if !existedAccount.LockUtil.IsZero() && existedAccount.LockUtil.Before(time.Now()) {
			err := accountCollection.Update(bson.M{
				"email": loginRequest.Email,
			}, bson.M{
				"$set": bson.M{"is_locked": false},
				"$unset": bson.M{
					"lock_at":      "",
					"lock_util":    "",
					"lock_message": "",
				},
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, bson.M{
					"status":  http.StatusInternalServerError,
					"message": err.Error(),
				})
				return
			}
		}
	}
	//Xóa key login fail trong redis nếu đăng nhập thành công
	redisClient.Del(ctx, loginFailedPrefix)

	//Sinh access token
	roles, _ := GetRolesFromAccount(*existedAccount)

	accessToken, accessTokenClaims, err := utils.GenerateToken(existedAccount.ID.Hex(), loginRequest.Email, roles, configs.GetJWTAccessExp(), "access")
	if err != nil {
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}
	//Sinh refresh token
	refreshToken, refreshTokenClaims, err := utils.GenerateToken(existedAccount.ID.Hex(), loginRequest.Email, roles, configs.GetJWTRefreshExp(), "refresh")
	if err != nil {
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	// Khởi tạo session collection
	sessionCollection := &collections.Session{}
	// Sử dụng 'Upsert' và 'collections.Session'
	sessionRes, _ := sessionCollection.FindOneAndUpdate(collections.Session{
		IsRevoked:     false,
		RefreshToken:  refreshToken,
		TrustedDevice: true,
		DeviceId:      c.GetHeader("Device-Id"),
		UserId:        existedAccount.ID,
		CreatedAt:     refreshTokenClaims.RegisteredClaims.IssuedAt.Time,
		ExpiresAt:     refreshTokenClaims.ExpiresAt.Time,
		ApprovedToken: "",
	})

	c.JSON(http.StatusOK, bson.M{
		"status":    int(http.StatusOK),
		"message":   "Login account successfully",
		"timestamp": time.Now(),
		"data": bson.M{
			"session_id":               sessionRes.Id,
			"access_token":             accessToken,
			"refresh_token":            refreshToken,
			"access_token_expired_at":  accessTokenClaims.ExpiresAt.Time,
			"refresh_token_expired_at": refreshTokenClaims.ExpiresAt.Time,
		},
	})
}

func GetRolesFromAccount(account collections.Account) ([]string, error) {
	var roles []string
	roleEntry := &collections.Role{} // Khởi tạo collection

	// Lấy main role
	if account.RoleId != primitive.NilObjectID {
		baseFilter := bson.M{
			"_id": account.RoleId,
			"deleted_at": bson.M{
				"$exists": false,
			},
		}
		err := roleEntry.First(baseFilter)
		if err != nil {
			if !errors.Is(err, mongo.ErrNoDocuments) {
				return nil, err
			}
		} else {
			roles = append(roles, roleEntry.Name)
		}
	}

	// Lấy sub role
	if account.SubroleId != primitive.NilObjectID {
		baseFilter := bson.M{
			"_id": account.RoleId,
			"deleted_at": bson.M{
				"$exists": false,
			},
		}
		err := roleEntry.First(baseFilter)
		if err != nil {
			if !errors.Is(err, mongo.ErrNoDocuments) {
				return nil, err
			}
		} else {
			if roleEntry.Name != "" && (len(roles) == 0 || roles[0] != roleEntry.Name) {
				roles = append(roles, roleEntry.Name)
			}
		}
	}

	return roles, nil
}
