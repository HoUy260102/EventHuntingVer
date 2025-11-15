package controllers

import (
	"EventHunting/collections"
	"EventHunting/configs"
	"EventHunting/database"
	"EventHunting/dto"
	"EventHunting/utils"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/markbates/goth/gothic"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	loginFailedTime     = 5 * time.Minute
	maxLoginFailedCount = 5
)

func Login(c *gin.Context) {
	var (
		loginRequest dto.LoginRequest
		ctx, cancel  = context.WithTimeout(context.Background(), 10*time.Second)
		redisClient  = database.GetRedisClient().Client
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

func RenewAccessToken(c *gin.Context) {
	var (
		req          dto.RenewAcessTokenRequest
		err          error
		sessionEntry = &collections.Session{}
	)

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	err = sessionEntry.First(nil, bson.M{
		"refresh_token": req.RefreshToken,
	})
	switch {
	case err == nil:
		if sessionEntry.IsRevoked {
			utils.ResponseError(c, http.StatusUnauthorized, "Token này đã được thu hồi!", err.Error())
			return
		}
	case err == mongo.ErrNoDocuments:
		utils.ResponseError(c, http.StatusUnauthorized, "Token không hợp lệ!", err.Error())
		return
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	refreshTokenClaim, err := utils.ExtractCustomClaims(req.RefreshToken)
	if err != nil {
		utils.ResponseError(c, http.StatusUnauthorized, "Token không hợp lệ!", err.Error())
		return
	}

	roles := refreshTokenClaim.Roles
	accessToken, _, err := utils.GenerateToken(refreshTokenClaim.RegisteredClaims.Subject, refreshTokenClaim.Email, roles, configs.GetJWTAccessExp(), "access")
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Làm mới access token thành công.", accessToken, nil)
}

func Logout(c *gin.Context) {
	var (
		err          error
		sessionEntry = &collections.Session{}
		redisClient  = database.GetRedisClient().Client
	)
	deviceId := c.GetHeader("Device-Id")
	authHeader := c.GetHeader("Authorization")
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		utils.ResponseError(c, http.StatusUnauthorized, "Không tìm thấy token!", nil)
		return
	}
	tokenClaims, err := utils.ExtractCustomClaims(token)
	if err != nil {
		utils.ResponseError(c, http.StatusUnauthorized, "Token không hợp lệ hoặc hết hạn!", err.Error())
		return
	}
	accountId, _ := primitive.ObjectIDFromHex(tokenClaims.RegisteredClaims.Subject)

	err = sessionEntry.Update(nil, bson.M{
		"user_id":   accountId,
		"device_id": deviceId,
	}, bson.M{
		"$set": bson.M{
			"is_revoked": true,
		},
	})

	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	duration := tokenClaims.RegisteredClaims.ExpiresAt.Unix() - time.Now().Unix() + 3600
	if duration > 0 {
		key := fmt.Sprintf("blacklist:accesstoken:%s", token)
		err := redisClient.Set(c.Request.Context(), key, "", time.Duration(duration)*time.Second).Err()
		if err != nil {
			log.Printf("Redis blacklist set error: %v", err)
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
			return
		}
	}
	utils.ResponseSuccess(c, http.StatusOK, "Logout thành công.", nil, nil)
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

func BeginGoogleAuth(c *gin.Context) {
	q := c.Request.URL.Query()
	q.Add("provider", "google")
	c.Request.URL.RawQuery = q.Encode()
	gothic.BeginAuthHandler(c.Writer, c.Request)
}

func OAuthCallback(c *gin.Context) {
	var (
		accountEntry = &collections.Account{}
		sessionEntry = &collections.Session{}
		err          error
	)
	q := c.Request.URL.Query()
	q.Add("provider", "google")
	c.Request.URL.RawQuery = q.Encode()

	//Gửi authorization code lên resource server để nhận token
	user, err := gothic.CompleteUserAuth(c.Writer, c.Request)
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	err = accountEntry.First(bson.M{
		"email": user.Email,
	})

	switch {
	case err == nil:
	case errors.Is(err, mongo.ErrNoDocuments):
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}
	roles, _ := GetRolesFromAccount(*accountEntry)
	accessToken, accessTokenClaims, err := utils.GenerateToken(accountEntry.ID.Hex(), user.Email, roles, configs.GetJWTAccessExp(), "access")
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}
	//Sinh refresh token
	refreshToken, refreshTokenClaims, err := utils.GenerateToken(accountEntry.ID.Hex(), user.Email, roles, configs.GetJWTRefreshExp(), "refresh")
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	sessionRes, err := sessionEntry.FindOneAndUpdate(collections.Session{
		IsRevoked:     false,
		RefreshToken:  refreshToken,
		TrustedDevice: true,
		DeviceId:      c.GetHeader("Device-Id"),
		UserId:        accountEntry.ID,
		CreatedAt:     refreshTokenClaims.RegisteredClaims.IssuedAt.Time,
		ExpiresAt:     refreshTokenClaims.ExpiresAt.Time,
		ApprovedToken: "",
	})

	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	c.JSON(http.StatusOK, bson.M{
		"status":    http.StatusOK,
		"message":   "Login thành công",
		"timestamp": time.Now(),
		"data": bson.M{
			"session_id":               sessionRes.Id,
			"access_token":             accessToken,
			"refresh_token":            refreshToken,
			"access_token_expired_at":  accessTokenClaims.ExpiresAt.Time,
			"refresh_token_expired_at": refreshTokenClaims.ExpiresAt.Time,
		},
	})

	//payload := fmt.Sprintf(`{
	//    "access_token": "%s",
	//    "refresh_token": "%s",
	//}`, accessToken, refreshToken)
	//html := fmt.Sprintf(`
	//<html>
	//  <body>
	//    <script>
	//      window.opener.postMessage(%s, "http://localhost:5173");
	//      window.close();
	//    </script>
	//  </body>
	//</html>
	//`, payload)
	//
	//c.Data(http.StatusOK, "text/html", []byte(html))
}

//func GetCurrentRole(account collections.Account, redisClient *redis.Client) (string, error) {
//	var (
//		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
//		roleEntry   = &collections.Role{}
//	)
//	defer cancel()
//	role, err := redisClient.Get(ctx, fmt.Sprintf("auth:account:%s:current_role", account.ID.Hex())).Result()
//	if err == redis.Nil {
//		_ = roleEntry.First(bson.M{
//			"_id": account.RoleId,
//		})
//		redisClient.Set(ctx, fmt.Sprintf("auth:account:%s:current_role", account.ID.Hex()), roleEntry.Name, 0)
//		return roleEntry.Name, nil
//	} else if err != nil {
//		return "", err
//	}
//	return role, nil
//}
