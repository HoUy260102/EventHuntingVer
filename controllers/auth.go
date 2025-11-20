package controllers

import (
	"EventHunting/collections"
	"EventHunting/configs"
	"EventHunting/consts"
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
	"github.com/redis/go-redis/v9"
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
		handleLoginFailure(c, redisClient, loginRequest.Email, loginFailedPrefix)
		return
	}

	//Set lại cài đặt nếu người dùng đăng nhập đúng
	if existedAccount.IsLocked == true {
		if !existedAccount.LockUtil.IsZero() && existedAccount.LockUtil.Before(time.Now()) && existedAccount.LockReason == consts.LockReasonLoginFail {
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
		if existedAccount.LockReason != consts.LockReasonLoginFail {
			c.JSON(http.StatusBadRequest, bson.M{
				"status":  http.StatusBadRequest,
				"message": existedAccount.LockMessage,
			})
			return
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

func handleLoginFailure(c *gin.Context, redisClient *redis.Client, email string, loginFailedPrefix string) {
	var (
		accountCollection = &collections.Account{}
	)
	// Tăng biến đếm trong Redis
	count, err := redisClient.Incr(c.Request.Context(), loginFailedPrefix).Result()
	if err != nil {
		log.Println("Redis INCR error:", err)
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	// Set TTL chỉ khi key mới được tạo
	if count == 1 {
		err := redisClient.Expire(c.Request.Context(), loginFailedPrefix, loginFailedTime).Err()
		if err != nil {
			log.Println("Redis EXPIRE error:", err)
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
			return
		}
	}

	// Kiểm tra số lần login fail
	if count > int64(maxLoginFailedCount) {
		lockTime := time.Now().Add(loginFailedTime)
		lockMessage := "Đăng nhập quá nhiều lần vui lòng thử lại sau, " + lockTime.Format("2006-01-02 15:04:05")

		// Cập nhật DB khóa tài khoản
		err := accountCollection.Update(bson.M{
			"email": email,
		}, bson.M{
			"$set": bson.M{
				"is_locked":    true,
				"lock_at":      time.Now(),
				"lock_util":    lockTime,
				"lock_message": lockMessage,
				"lock_reason":  consts.LockReasonLoginFail,
			},
		})

		if err != nil {
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
			return
		}

		// Trả về lỗi bị khóa
		utils.ResponseError(c, http.StatusBadRequest, lockMessage, nil)
		return
	}

	// Trả về lỗi sai mật khẩu thông thường (chưa bị khóa)
	utils.ResponseError(c, http.StatusBadRequest, "Tài khoản đăng nhập hoặc mật khẩu không chính xác!", nil)
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
			utils.ResponseError(c, http.StatusUnauthorized, "Token này đã được thu hồi!", nil)
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
		"deleted_at": bson.M{
			"$exists": false,
		},
	})

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			//Đăng ký tài khoản mới
			return
		}
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
}

func SignUpForUserAccount(c *gin.Context) {
	// Bind và Validate
	var (
		signUpRequest dto.CreateUserRequest
		accountEntry  = collections.Account{}
		RoleEntry     = collections.Role{}
		err           error
	)
	if err := c.ShouldBindJSON(&signUpRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Dữ liệu không hợp lệ: " + err.Error(),
		})
		return
	}

	if errs := utils.ValidateCreateUser(signUpRequest); len(errs) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": errs,
		})
		return
	}

	// Kiểm tra sự tồn tại của Email (Logic rõ ràng hơn)
	err = accountEntry.First(bson.M{
		"email": signUpRequest.Email,
	})

	if err == nil {
		// Tìm thấy tài khoản (err == nil)
		if accountEntry.IsVerified {
			c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Email này đã tồn tại"})
			return
		}
		// Nếu tìm thấy nhưng chưa xác thực
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Email này đã được đăng ký nhưng chưa xác nhận"})
		return
	}

	if !errors.Is(err, mongo.ErrNoDocuments) {
		// Đây là một lỗi CSDL thực sự (vd: mất kết nối), không phải "không tìm thấy"
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi máy chủ khi kiểm tra email: " + err.Error()})
		return
	}

	// Lấy vai trò "User"
	err = RoleEntry.First(bson.M{"name": "User"})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi cấu hình hệ thống."})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi máy chủ khi tìm vai trò: " + err.Error()})
		}
		return
	}

	// Băm mật khẩu
	hashPassword, err := utils.HashPassword(signUpRequest.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Không thể tạo mật khẩu: " + err.Error()})
		return
	}

	// Chuẩn bị dữ liệu tài khoản
	signUpId := primitive.NewObjectID()

	verifySignUpToken, _, err := utils.GenerateToken(signUpId.Hex(), signUpRequest.Email, []string{"User"}, configs.GetJWTVerifyExp(), "verify") // 900s = 15 phút
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Không thể tạo token xác thực: " + err.Error()})
		return
	}

	signUpAccount := collections.Account{
		ID:                signUpId,
		Name:              signUpRequest.Name,
		Email:             signUpRequest.Email,
		Password:          hashPassword,
		RoleId:            RoleEntry.Id,
		CreatedAt:         time.Now(),
		CreatedBy:         signUpId,
		IsVerified:        false,
		VerifySignUpToken: verifySignUpToken,
	}

	// Gán các trường tùy chọn
	if signUpRequest.Phone != nil {
		signUpAccount.Phone = *signUpRequest.Phone
	}

	if signUpRequest.UserInfo != nil {
		signUpAccount.UserInfo = &collections.User{}
		if signUpRequest.UserInfo.Dob != nil {
			signUpAccount.UserInfo.Dob = *signUpRequest.UserInfo.Dob
		}
		if signUpRequest.UserInfo.IsMale != nil {
			signUpAccount.UserInfo.IsMale = *signUpRequest.UserInfo.IsMale
		}
	}

	go func() {
		content := fmt.Sprintf(`
			<h2>Chào mừng bạn đến với Event App 🎉</h2>
			<p>Cảm ơn bạn đã đăng ký tài khoản. Vui lòng xác minh email của bạn bằng cách nhấn vào nút bên dưới:</p>
			<p>
				<a href="http://localhost:8080/api/v1/auth/signup/confirm?token=%s"
					style="background-color:#4CAF50;color:white;padding:10px 20px;text-decoration:none;border-radius:6px;">
					Xác minh tài khoản
				</a>
			</p>
			<p>Nếu bạn không yêu cầu tạo tài khoản, hãy bỏ qua email này.</p>
			<p>Trân trọng,<br>Đội ngũ Event App</p>
		`, verifySignUpToken)

		const maxRetries = 3
		var sendErr error
		emailService := utils.NewEmailService()
		for attempt := 1; attempt <= maxRetries; attempt++ {
			sendErr = emailService.SendEmail(utils.EmailPayload{
				To:       []string{signUpRequest.Email},
				HTMLBody: content,
				Subject:  "Xác thực đăng ký tài khoản",
			})

			if sendErr == nil {
				log.Printf("Gửi email xác thực thành công đến %s", signUpRequest.Email)
				break
			}

			log.Printf("Lần gửi thứ %d thất bại: %v", attempt, sendErr)
			time.Sleep(2 * time.Second) // chờ 2s rồi thử lại
		}

		if sendErr != nil {
			log.Printf("Gửi email xác thực thất bại sau %d lần: %v", maxRetries, sendErr)
		}
	}()

	// Chèn vào CSDL
	err = signUpAccount.Create()
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Email này đã tồn tại."})
			return
		}
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi máy chủ khi tạo tài khoản: " + err.Error(),
		})
		return
	}

	// Trả về thành công
	c.JSON(http.StatusOK, gin.H{
		"status":  http.StatusOK,
		"message": "Đăng ký thành công. Hãy kiểm tra email để xác nhận tài khoản.",
		"data": bson.M{
			"verify_sign_up_token": verifySignUpToken,
		},
	})
}

func ConfirmSignUp(c *gin.Context) {
	var (
		verifySignUpToken string
		accountEntry      = &collections.Account{}
		roleEntry         = &collections.Role{}
		err               error
	)
	verifySignUpToken = c.Query("token")
	verifyTokenClaims, err := utils.ExtractCustomClaims(verifySignUpToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	if verifyTokenClaims.Type != "verify" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Token này không phải type verify",
		})
		return
	}

	err = accountEntry.First(bson.M{
		"verify_sign_up_token": verifySignUpToken,
	})

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  http.StatusBadRequest,
				"message": "Không tìm thấy token hoặc token này đã được dùng",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	if accountEntry.IsVerified == true {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Tài khoản này đã xác thực",
		})
		return
	}

	filter := bson.M{"_id": accountEntry.ID}
	update := bson.M{
		"$set": bson.M{
			"is_verified": true,
			"updated_at":  time.Now(),
			"updated_by":  accountEntry.ID,
			"verified_at": time.Now(),
		},
		"$unset": bson.M{
			"verify_sign_up_token": "",
		},
	}
	roleId := accountEntry.RoleId
	_ = roleEntry.First(bson.M{
		"_id": roleId,
	})

	setMap, ok := update["$set"].(bson.M)
	if !ok {
		setMap = bson.M{}
		update["$set"] = setMap
	}

	if roleEntry.Name == "User" {
		setMap["is_active"] = true
	}

	err = accountEntry.Update(filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  http.StatusOK,
		"message": "Đăng ký tài khoản thành công",
	})
}

func ResendConfirmSignUp(c *gin.Context) {
	var resendConfirmSignUp struct {
		Email string `json:"email"`
	}
	var (
		roles        []string
		accountEntry = &collections.Account{}
		roleEntry    = &collections.Role{}
	)

	if err := c.ShouldBindJSON(&resendConfirmSignUp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	checkExisted := accountEntry.First(bson.M{
		"email": resendConfirmSignUp.Email,
	})

	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  http.StatusBadRequest,
				"message": "Không tìm thấy tài khoản",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": checkExisted.Error(),
		})
		return
	}

	if accountEntry.IsVerified == true {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Tài khoản này đã được đăng ký",
		})
		return
	}

	checkExisted = roleEntry.First(bson.M{
		"_id": accountEntry.RoleId,
	})

	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  http.StatusBadRequest,
				"message": "Không tìm thấy role",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": checkExisted.Error(),
		})
		return
	}
	roles = append(roles, accountEntry.Name)

	verifySignUpToken, _, err := utils.GenerateToken(accountEntry.ID.Hex(), accountEntry.Email, roles, configs.GetJWTVerifyExp(), "verify")
	if err != nil {
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}
	go func() {
		content := fmt.Sprintf(`
			<h2>Chào mừng bạn đến với Event App 🎉</h2>
			<p>Cảm ơn bạn đã đăng ký tài khoản. Vui lòng xác minh email của bạn bằng cách nhấn vào nút bên dưới:</p>
			<p>
				<a href="http://localhost:8080/api/v1/auth/signup/confirm?token=%s"
					style="background-color:#4CAF50;color:white;padding:10px 20px;text-decoration:none;border-radius:6px;">
					Xác minh tài khoản
				</a>
			</p>
			<p>Nếu bạn không yêu cầu tạo tài khoản, hãy bỏ qua email này.</p>
			<p>Trân trọng,<br>Đội ngũ Event App</p>
		`, verifySignUpToken)

		const maxRetries = 3
		var sendErr error
		emailService := utils.NewEmailService()
		for attempt := 1; attempt <= maxRetries; attempt++ {
			sendErr = emailService.SendEmail(utils.EmailPayload{
				To:       []string{resendConfirmSignUp.Email},
				HTMLBody: content,
				Subject:  "Xác thực đăng ký tài khoản",
			})
			if sendErr == nil {
				log.Printf("Gửi email xác thực thành công đến %s", accountEntry.Email)
				break
			}

			log.Printf("Lần gửi thứ %d thất bại: %v", attempt, sendErr)
			time.Sleep(2 * time.Second) // chờ 2s rồi thử lại
		}

		if sendErr != nil {
			log.Printf("Gửi email xác thực thất bại sau %d lần: %v", maxRetries, sendErr)
		}
	}()

	err = accountEntry.Update(bson.M{
		"email": resendConfirmSignUp.Email,
	}, bson.M{
		"$set": bson.M{
			"verify_sign_up_token": verifySignUpToken,
		},
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, bson.M{
			"status":  http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  http.StatusOK,
		"message": "Thành công",
		"data": bson.M{
			"verify_sign_up_token": verifySignUpToken,
		},
	})
}
