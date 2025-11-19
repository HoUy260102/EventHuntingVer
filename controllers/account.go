package controllers

import (
	"EventHunting/collections"
	"EventHunting/consts"
	"EventHunting/dto"
	"EventHunting/utils"
	"errors"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var RoleDataGroups = map[string]string{
	"User":      "user_info",
	"Organizer": "organizer_info",
}

func CreateAccount(c *gin.Context) {
	var (
		roleEntry = &collections.Role{}
	)

	// Bind dữ liệu từ body request
	var createData dto.CreateAccount
	if err := c.ShouldBindJSON(&createData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Dữ liệu gửi lên không hợp lệ: " + err.Error(),
		})
		return
	}

	// Validate dữ liệu đã bind
	validationErrors := dto.ValidateCreateAccountRequest(createData)
	if len(validationErrors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": validationErrors,
		})
		return
	}

	newAccount := collections.Account{
		Email: createData.Email,
	}

	checkExisted := newAccount.First(bson.M{
		"email": newAccount.Email,
	})

	if checkExisted == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Email này đã tồn tại",
		})
		return
	}

	checkExisted = roleEntry.First(bson.M{
		"_id": createData.RoleId,
	})

	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  http.StatusBadRequest,
				"message": "Không tìm thấy role!",
			})
			return
		}
	}

	allowedGroup, ownsDataGroup := RoleDataGroups[roleEntry.Name]
	if createData.UserInfo != nil && (!ownsDataGroup || allowedGroup != "user_info") {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Vai trò '" + roleEntry.Name + "' không được phép cập nhật 'user_info'",
		})
		return
	}

	if createData.OrganizerInfo != nil && (!ownsDataGroup || allowedGroup != "organizer_info") {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Vai trò '" + roleEntry.Name + "' không được phép cập nhật 'organizer_info'",
		})
		return
	}

	// Lấy thông tin người tạo từ Token
	authHeader := c.GetHeader("Authorization")
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Token không thấy!",
		})
		return
	}
	tokenClaims, err := utils.ExtractCustomClaims(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  http.StatusUnauthorized,
			"message": "Token không hợp lệ: " + err.Error(),
		})
		return
	}

	creatorObjectId, err := primitive.ObjectIDFromHex(tokenClaims.RegisteredClaims.Subject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi xử lý ID người tạo: " + err.Error(),
		})
		return
	}

	// Băm mật khẩu
	hashedPassword, err := utils.HashPassword(createData.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi khi xử lý mật khẩu",
		})
		return
	}

	// Chuyển đổi DTO sang Model
	now := time.Now()
	newAccount.Name = createData.Name
	newAccount.Password = hashedPassword
	newAccount.RoleId = createData.RoleId
	newAccount.Phone = createData.Phone
	newAccount.IsVerified = true
	newAccount.IsActive = true
	newAccount.CreatedAt = now
	newAccount.CreatedBy = creatorObjectId
	newAccount.UpdatedAt = now
	newAccount.UpdatedBy = creatorObjectId

	// Xử lý các trường tùy chọn
	if createData.Address != nil {
		newAccount.Address = *createData.Address
	}

	if createData.UserInfo != nil {
		var userInfo collections.User
		if createData.UserInfo.Dob != nil {
			userInfo.Dob = *createData.UserInfo.Dob
		}
		if createData.UserInfo.IsMale != nil {
			userInfo.IsMale = *createData.UserInfo.IsMale
		}
		newAccount.UserInfo = &userInfo
	}

	if createData.OrganizerInfo != nil {
		var orgInfo collections.Organizer
		if createData.OrganizerInfo.Decription != nil {
			orgInfo.Decription = *createData.OrganizerInfo.Decription
		}
		if createData.OrganizerInfo.WebsiteUrl != nil {
			orgInfo.WebsiteUrl = *createData.OrganizerInfo.WebsiteUrl
		}
		if createData.OrganizerInfo.ContactName != nil {
			orgInfo.ContactName = *createData.OrganizerInfo.ContactName
		}
		newAccount.OrganizerInfo = &orgInfo
	}

	// Gọi Collection để tạo mới
	err = newAccount.Create()
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, gin.H{
			"status":  http.StatusCreated,
			"message": "Tạo tài khoản thành công",
			"entry":   utils.PrettyJSON(newAccount.ParseEntry()),
		})
	case mongo.IsDuplicateKeyError(err):
		c.JSON(http.StatusConflict, gin.H{
			"status":  http.StatusConflict,
			"message": "Tài khoản đã tồn tại (lỗi duplicate key)",
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống",
			"error":   "Lỗi khi tạo tài khoản: " + err.Error(),
		})
	}
}

func UpdateAccount(c *gin.Context) {
	var (
		accountEntry = &collections.Account{}
		roleEntry    = &collections.Role{}
	)
	// LẤY ID TÀI KHOẢN CẦN UPDATE
	accountId := c.Param("id")
	accountObjectId, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Id tài khoản không hợp lệ"})
		return
	}

	// LẤY ID NGƯỜI CẬP NHẬT
	updatorObjectId, _ := utils.GetAccountID(c)

	// BIND VÀ VALIDATE REQUEST BODY
	var req dto.UpdateAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Dữ liệu gửi lên không hợp lệ: " + err.Error(),
		})
		return
	}

	// Validate request
	validationErrors := dto.ValidateUpdateAccountReq(req)
	if len(validationErrors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": validationErrors,
		})
		return
	}

	// KIỂM TRA TÀI KHOẢN TỒN TẠI
	checkExisted := accountEntry.First(bson.M{"_id": accountObjectId})
	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "Tài khoản không tồn tại"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": checkExisted.Error()})
		return
	}

	// KIỂM TRA QUYỀN (PHẢI LÀ ADMIN HOẶC CHÍNH CHỦ)
	rolesValue, exists := c.Get("roles")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Không tìm thấy roles trong context"})
		return
	}

	updatorRoles, ok := rolesValue.([]string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Roles trong context không đúng định dạng (cần []string)"})
		return
	}
	if updatorObjectId.Hex() != accountId {
		// Nếu không phải chính chủ, kiểm tra xem có phải Admin không
		if !slices.Contains(updatorRoles, "Admin") { // Giả sử role Admin
			c.JSON(http.StatusForbidden, gin.H{
				"status":  http.StatusForbidden,
				"message": "Bạn không có quyền cập nhật tài khoản này",
			})
			return
		}
	}

	// KIỂM TRA LOGIC VAI TRÒ
	err = roleEntry.First(bson.M{"_id": accountEntry.RoleId})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Dữ liệu vai trò của tài khoản không hợp lệ"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi hệ thống", "error": err.Error()})
		return
	}

	allowedGroup, ownsDataGroup := RoleDataGroups[roleEntry.Name]
	if req.UserInfo != nil && (!ownsDataGroup || allowedGroup != "user_info") {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Vai trò '" + roleEntry.Name + "' không được phép cập nhật 'user_info'",
		})
		return
	}

	if req.OrganizerInfo != nil && (!ownsDataGroup || allowedGroup != "organizer_info") {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Vai trò '" + roleEntry.Name + "' không được phép cập nhật 'organizer_info'",
		})
		return
	}

	// Chỉ $set các trường được cung cấp
	setData := bson.M{
		"updated_at": time.Now(),
		"updated_by": updatorObjectId,
	}

	if req.Name != nil {
		setData["name"] = *req.Name
	}
	if req.Phone != nil {
		setData["phone"] = *req.Phone
	}
	if req.Address != nil {
		setData["address"] = *req.Address
	}

	// Cập nhật trường con của UserInfo
	if req.UserInfo != nil {
		if req.UserInfo.Dob != nil {
			setData["user_info.dob"] = *req.UserInfo.Dob
		}
		if req.UserInfo.IsMale != nil {
			setData["user_info.is_male"] = *req.UserInfo.IsMale
		}
	}

	// Cập nhật trường con của OrganizerInfo
	if req.OrganizerInfo != nil {
		if req.OrganizerInfo.Decription != nil {
			setData["organizer_info.decription"] = *req.OrganizerInfo.Decription
		}
		if req.OrganizerInfo.WebsiteUrl != nil {
			// Validation của bạn đã xử lý logic cho phép "" (để xóa)
			setData["organizer_info.website_url"] = *req.OrganizerInfo.WebsiteUrl
		}
		if req.OrganizerInfo.ContactName != nil {
			setData["organizer_info.contact_name"] = *req.OrganizerInfo.ContactName
		}
	}

	updateQuery := bson.M{"$set": setData}

	err = accountEntry.Update(bson.M{"_id": accountObjectId}, updateQuery)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "Không tìm thấy tài khoản để cập nhật"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi khi cập nhật tài khoản: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  http.StatusOK,
		"message": "Cập nhật tài khoản thành công",
	})
}

func UploadAvatar(c *gin.Context) {
	var (
		accountEntry = &collections.Account{}
	)
	// Lấy ID từ URL
	accountId := c.Param("id")
	accountObjectId, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Id tài khoản không hợp lệ",
		})
		return
	}

	checkExisted := accountEntry.First(bson.M{
		"_id":        accountObjectId,
		"deleted_at": bson.M{"$exists": false},
	})
	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  http.StatusBadRequest,
				"message": "Tài khoản không tồn tại",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống",
			"error":   checkExisted.Error(),
		})
		return
	}

	updatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	rolesValue, exists := c.Get("roles")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  http.StatusUnauthorized,
			"message": "Không tìm thấy 'roles' trong context",
		})
		return
	}
	roles, ok := rolesValue.([]string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "'roles' trong context không đúng định dạng",
		})
		return
	}

	if updatorObjectId.Hex() != accountId {
		if !slices.Contains(roles, "Admin") {
			c.JSON(http.StatusForbidden, gin.H{
				"status":  http.StatusForbidden,
				"message": "Bạn không có quyền cập nhật avatar cho tài khoản này",
			})
			return
		}
	}

	//Kiểm tra gửi ảnh
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi đọc form!",
			"error":   err.Error(),
		})
		return
	}
	files := form.File["avatar"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Bạn chưa gửi ảnh!",
		})
		return
	}
	if len(files) > 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Chỉ được gửi tối đa một file ảnh!",
		})
		return
	}
	//Check gửi ảnh phải đúng định dạng
	if err := utils.ChechValidFile(files[0]); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Lỗi file không đúng định dạng!",
			"error":   err.Error(),
		})
		return
	}

	if err := utils.CheckValidMiMe(files[0]); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	fileOpen, err := files[0].Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi hệ thống!",
			"error":   "Lỗi đọc file:" + err.Error(),
		})
		return
	}
	defer fileOpen.Close()

	//Kiểm tra xem avatar có tồn tại không để xóa.
	cld := utils.GetCloudinary()

	if accountEntry.AvatarUrl != "" {
		err = utils.DeleteFileCloudinary(cld, accountEntry.AvatarUrlId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  http.StatusInternalServerError,
				"message": "Lỗi xóa ảnh trên cloudinary!",
				"error":   err.Error(),
			})
			return
		}
	}

	//Update ảnh lên cloudinary
	u := uuid.New().String()
	ext := filepath.Ext(files[0].Filename)
	baseName := strings.TrimSuffix(files[0].Filename, ext)
	fileName := fmt.Sprintf("%s_%s%s", baseName, u, ext)

	uploadResult, err := utils.UploadFileCloudinary(cld, fileOpen, "upload/avatar", fileName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi upload ảnh trên cloudinary!",
			"error":   err.Error(),
		})
		return
	}

	//Update link ảnh vào db
	var (
		baseFilter = bson.M{
			"_id":        accountObjectId,
			"deleted_at": bson.M{"$exists": false},
		}
		baseUpdate = bson.M{
			"$set": bson.M{
				"avatar_url":    uploadResult.URL,
				"avatar_url_id": uploadResult.PublicID,
				"updated_at":    time.Now(),
				"updated_by":    updatorObjectId,
			},
		}
	)
	err = (accountEntry).Update(baseFilter, baseUpdate)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":     http.StatusOK,
			"message":    "Upload ảnh thành công.",
			"avatar_url": uploadResult.URL,
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Lỗi do không tìm thấy tài khoản update!",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   err.Error(),
		})
	}
}

func FindAccounts(c *gin.Context) {
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

	// LẤY THÔNG SỐ TÌM KIẾM
	filter := bson.M{
		"deleted_at": nil,
	}

	dynamicFilter := utils.BuildAccountSearchFilter(queryMap)

	for key, value := range dynamicFilter {
		filter[key] = value
	}

	findOptions := options.Find()
	findOptions.SetLimit(limit)
	findOptions.SetSkip(skip)
	//Sort
	sortQuery := utils.BuildSortFilter(queryMap)
	findOptions.SetSort(sortQuery)

	// FIND
	accounts, err := (&collections.Account{}).Find(filter, findOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi hệ thống: Khi tìm kiếm account!",
			"error":   err.Error(),
		})
		return
	}

	// LẤY TỔNG SỐ
	totalDocs, err := (&collections.Account{}).CountDocuments(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi hệ thống: Lỗi khi đếm tài khoản!",
			"error":   err.Error(),
		})
		return
	}

	// Tính toán tổng số trang
	totalPages := int64(math.Ceil(float64(totalDocs) / float64(limit)))

	c.JSON(http.StatusOK, gin.H{
		"status":  http.StatusOK,
		"message": "Lấy danh sách tài khoản thành công.",
		"data":    accounts,
		"pagination": gin.H{
			"current_page": page,
			"per_page":     limit,
			"total_pages":  totalPages,
			"total_items":  totalDocs,
		},
	})
}

func FindAccountsByKeyset(c *gin.Context) {
	queryMap := c.Request.URL.Query()

	// LẤY CÁC THÔNG SỐ
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit < 1 {
		limit = 10
	}

	// Lấy 'last_id'
	lastIdStr := c.Query("last_id")

	//XÂY DỰNG FILTER
	filter := bson.M{
		"deleted_at": nil,
	}

	dynamicFilter := utils.BuildAccountSearchFilter(queryMap)

	for key, value := range dynamicFilter {
		filter[key] = value
	}

	// Nếu có 'last_id', thêm điều kiện (>)
	if lastIdStr != "" {
		lastObjectId, err := primitive.ObjectIDFromHex(lastIdStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  http.StatusBadRequest,
				"message": "Last ID không hợp lệ",
			})
			return
		}

		filter["_id"] = bson.M{"$gt": lastObjectId}
	}

	findOptions := options.Find().SetSort(bson.D{{"_id", 1}})

	//Lấy limit + 1 để kiểm tra hasMore
	findOptions.SetLimit(limit + 1)

	accounts, err := (&collections.Account{}).Find(filter, findOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi khi tìm kiếm tài khoản: " + err.Error(),
		})
		return
	}

	// XỬ LÝ KẾT QUẢ (N+1)

	hasMore := false
	if int64(len(accounts)) > limit {
		hasMore = true
		accounts = accounts[:limit]
	}

	// Lấy ID của bản ghi CUỐI CÙNG để làm "last_id" cho lần gọi tiếp theo
	var nextLastId string = ""
	if len(accounts) > 0 {
		nextLastId = accounts[len(accounts)-1].ID.Hex()
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  http.StatusOK,
		"message": "Lấy danh sách tài khoản thành công",
		"data":    accounts,
		"pagination": gin.H{
			"has_more":     hasMore,
			"next_last_id": nextLastId, // Client sẽ dùng ID này cho lần gọi sau
		},
	})
}

func LockAccount(c *gin.Context) {
	updatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	// LẤY ID TÀI KHOẢN CẦN KHÓA TỪ URL
	accountId := c.Param("id")
	accountObjectId, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Id tài khoản không hợp lệ"})
		return
	}

	// KIỂM TRA TÀI KHOẢN TỒN TẠI
	accountEntry := &collections.Account{}
	checkExisted := accountEntry.First(bson.M{
		"_id":        accountObjectId,
		"deleted_at": bson.M{"$exists": false},
	})

	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "Tài khoản không tồn tại hoặc đã bị xóa!"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi do hệ thống!", "error": checkExisted.Error()})
		return
	}

	if accountEntry.IsLocked == true {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Tài khoản này đã bị khóa từ trước!"})
		return
	}

	// VALIDATE REQUEST BODY
	var req dto.LockAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Dữ liệu gửi lên không hợp lệ!",
			"error":   err.Error(),
		})
		return
	}

	// XỬ LÝ LOGIC KHÓA
	setData := bson.M{
		"is_locked":    true,
		"lock_at":      time.Now(),
		"lock_message": req.Message,
		"lock_reason":  consts.LockReasonAdminBan,
		"updated_at":   time.Now(),
		"updated_by":   updatorObjectId,
	}
	unsetData := bson.M{}

	if req.Until != nil {
		// KHÓA TẠM THỜI
		if req.Until.Before(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  http.StatusBadRequest,
				"message": "Ngày mở khóa phải ở trong tương lai!",
			})
			return
		}
		setData["lock_util"] = *req.Until
	} else {
		// KHÓA VĨNH VIỄN
		unsetData["lock_util"] = ""
	}

	updateData := bson.M{
		"$set": setData,
	}
	if len(unsetData) > 0 {
		updateData["$unset"] = unsetData
	}

	filter := bson.M{
		"_id":        accountObjectId,
		"deleted_at": bson.M{"$exists": false},
	}

	err = accountEntry.Update(filter, updateData)

	message := "Khóa tài khoản thành công."
	if req.Until == nil {
		message = "Khóa vĩnh viễn tài khoản thành công."
	}

	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":  http.StatusOK,
			"message": message,
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Lỗi do không tìm thấy tài khoản update!",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   err.Error(),
		})
	}
}

func UnlockAccount(c *gin.Context) {
	updatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	//LẤY ID TÀI KHOẢN CẦN MỞ KHÓA TỪ URL
	accountId := c.Param("id")
	accountObjectId, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Id tài khoản không hợp lệ!"})
		return
	}

	//KIỂM TRA TÀI KHOẢN TỒN TẠI
	accountEntry := &collections.Account{}
	checkExisted := accountEntry.First(bson.M{
		"_id":        accountObjectId,
		"deleted_at": bson.M{"$exists": false},
	})

	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "Tài khoản không tồn tại hoặc đã bị xóa!"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Lỗi do hệ thống!", "error": checkExisted.Error()})
		return
	}

	if accountEntry.IsLocked == false {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Tài khoản này hiện tại không bị khóa!"})
		return
	}

	//THỰC HIỆN UPDATE
	updateData := bson.M{
		"$set": bson.M{
			"is_locked":  false,
			"updated_at": time.Now(),
			"updated_by": updatorObjectId,
		},
		"$unset": bson.M{
			"lock_at":      "",
			"lock_util":    "",
			"lock_message": "",
			"lock_reason":  "",
		},
	}

	filter := bson.M{
		"_id":        accountObjectId,
		"deleted_at": bson.M{"$exists": false},
	}
	err = accountEntry.Update(filter, updateData)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":  http.StatusOK,
			"message": "Mở khóa tài khoản thành công.",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Lỗi do không tìm thấy tài khoản update!",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   err.Error(),
		})
	}
}

func GetAccount(c *gin.Context) {
	accountId := c.Param("id")
	accountObjectId, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Id tài khoản không hợp lệ!",
		})
		return
	}

	accountEntry := &collections.Account{}

	filter := bson.M{
		"_id":        accountObjectId,
		"deleted_at": bson.M{"$exists": false},
	}

	err = accountEntry.First(filter)

	switch {
	case err == nil:
		accountEntry.Password = ""
		c.JSON(http.StatusOK, gin.H{
			"status":  http.StatusOK,
			"message": "Lấy thông tin tài khoản thành công.",
			"data":    utils.PrettyJSON(accountEntry.ParseEntry()),
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": "Tài khoản không tồn tại hoặc đã bị xóa!",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   err.Error(),
		})
	}
}

func SoftDeleteAccount(c *gin.Context) {
	updatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	// LẤY ID TỪ URL VÀ KIỂM TRA
	accountId := c.Param("id")
	accountObjectId, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Id tài khoản không hợp lệ!",
		})
		return
	}

	// KIỂM TRA TÀI KHOẢN TỒN TẠI
	accountEntry := &collections.Account{}
	filter := bson.M{
		"_id":        accountObjectId,
		"deleted_at": bson.M{"$exists": false},
	}

	checkExisted := accountEntry.First(filter)
	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  http.StatusNotFound,
				"message": "Tài khoản không tồn tại hoặc đã bị xóa!",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   checkExisted.Error(),
		})
		return
	}

	// THỰC HIỆN SOFT DELETE
	updateData := bson.M{
		"$set": bson.M{
			"deleted_at":   time.Now(),
			"deleted_by":   updatorObjectId,
			"updated_at":   time.Now(),
			"updated_by":   updatorObjectId,
			"is_locked":    true,
			"lock_message": "Tài khoản đã bị quản trị viên xóa, Hãy liên hệ với admin",
		},
	}

	err = accountEntry.Update(filter, updateData)

	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":  http.StatusOK,
			"message": "Xóa (mềm) tài khoản thành công.",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Lỗi do không tìm thấy tài khoản để xóa",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống khi xóa",
			"error":   err.Error(),
		})
	}
}

func RestoreAccount(c *gin.Context) {
	updatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	//LẤY ID TỪ URL VÀ KIỂM TRA
	accountId := c.Param("id")
	accountObjectId, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Id tài khoản không hợp lệ",
		})
		return
	}

	// KIỂM TRA TÀI KHOẢN TỒN TẠI
	accountEntry := &collections.Account{}
	filter := bson.M{
		"_id":        accountObjectId,
		"deleted_at": bson.M{"$exists": true},
	}

	checkExisted := accountEntry.First(filter)
	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  http.StatusNotFound,
				"message": "Tài khoản không tồn tại hoặc chưa bị xóa!",
				"error":   checkExisted.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống!",
			"error":   checkExisted.Error(),
		})
		return
	}

	updateData := bson.M{
		"$set": bson.M{
			"is_locked":  false,
			"updated_at": time.Now(),
			"updated_by": updatorObjectId,
		},
		"$unset": bson.M{
			"deleted_at":   "",
			"deleted_by":   "",
			"lock_at":      "",
			"lock_util":    "",
			"lock_message": "",
		},
	}

	err = accountEntry.Update(filter, updateData)

	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"status":  http.StatusOK,
			"message": "Khôi phục tài khoản thành công",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Lỗi do không tìm thấy tài khoản để khôi phục!",
			"error":   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  http.StatusInternalServerError,
			"message": "Lỗi do hệ thống khi khôi phục!",
			"error":   err.Error(),
		})
	}
}
