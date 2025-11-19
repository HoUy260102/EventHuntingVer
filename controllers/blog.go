package controllers

import (
	"EventHunting/collections"
	"EventHunting/consts"
	"EventHunting/database"
	"EventHunting/dto"
	"EventHunting/utils"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CreateBlog(c *gin.Context) {
	var (
		mediaEntry = &collections.Media{}
		mediaIDs   = []primitive.ObjectID{}
		req        dto.CreateBlogRequest
		err        error
	)
	// Bind và Validate JSON input
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Lỗi bind dữ liệu!",
			"error":   err.Error(),
		})
		return
	}

	if err := utils.ValidateCreateBlogRequest(req); len(err) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Dữ liệu đầu vào không hợp lệ!",
			"error":   err,
		})
		return
	}

	// Lấy ID của user
	createdByID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	newBlogId := primitive.NewObjectID()
	// Map DTO
	newBlog := collections.Blog{
		ID:            newBlogId,
		Title:         req.Title,
		Content:       req.Content,
		ContentHtml:   req.ContentHtml,
		IsEdit:        false,
		IsLockComment: false,
		CreatedAt:     time.Now(),
		CreatedBy:     createdByID,
		UpdatedAt:     time.Now(),
		UpdatedBy:     createdByID,
		View:          0,
	}

	if req.TagIds != nil {
		newBlog.TagIds = *req.TagIds
	}

	if req.ThumbnailID != nil {
		err = mediaEntry.First(nil, bson.M{
			"_id": req.ThumbnailID,
		})
		switch {
		case err == nil:
		case errors.Is(err, mongo.ErrNoDocuments):
			utils.ResponseError(c, http.StatusBadRequest, "", fmt.Errorf("Không tìm thấy thumbnail: %v", err.Error()))
			return
		default:
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
			return
		}
		if mediaEntry.Type != consts.MEDIA_IMAGE {
			utils.ResponseError(c, http.StatusBadRequest, "", "Thumbnail phải là ảnh")
			return
		}
		newBlog.ThumbnailUrl = mediaEntry.Url
		newBlog.ThumbnailID = *req.ThumbnailID
		mediaIDs = append(mediaIDs, *req.ThumbnailID)
	}

	//Logic kiểm tra ảnh có tồn tại
	if req.MediaIDs != nil {
		if len(*req.MediaIDs) > 0 {
			existedMediaIDMap := make(map[primitive.ObjectID]struct{})
			validMediaFilter := bson.M{
				"_id": bson.M{
					"$in": req.MediaIDs,
				},
			}
			medias, err := mediaEntry.Find(nil, validMediaFilter)
			for _, media := range medias {
				existedMediaIDMap[media.ID] = struct{}{}
			}
			if err != nil {
				utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
				return
			}
			invalidMeida := []string{}
			for _, mediaID := range *req.MediaIDs {
				if _, ok := existedMediaIDMap[mediaID]; !ok {
					invalidMeida = append(invalidMeida, mediaID.Hex())
				}
			}
			if len(medias) != len(*req.MediaIDs) {
				utils.ResponseError(c, http.StatusBadRequest, "", fmt.Errorf("MediaIDs [%s] không hợp lệ", strings.Join(invalidMeida, ", ")).Error())
				return
			}
		}
		mediaIDs = append(mediaIDs, *req.MediaIDs...)
	}

	db := database.GetDB()
	session, err := db.Client().StartSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi khi bắt đầu session!",
			Error:   err.Error(),
		})
		return
	}
	defer session.EndSession(c)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = mongo.WithSession(ctx, session, func(sessCtx mongo.SessionContext) error {
		_, err := sessCtx.WithTransaction(sessCtx, func(sessCtx mongo.SessionContext) (interface{}, error) {
			// 1. Update media nếu có
			if len(mediaIDs) > 0 {
				mediaFilter := bson.M{"_id": bson.M{"$in": mediaIDs}, "status": "PENDING"}
				mediaUpdate := bson.M{"$set": bson.M{"status": consts.MediaStatusSuccess}}
				const maxRetry = 3
				for i := 0; i < maxRetry; i++ {
					if err := mediaEntry.UpdateMany(sessCtx, mediaFilter, mediaUpdate); err != nil {
						if !mongo.IsTimeout(err) && !mongo.IsNetworkError(err) {
							return nil, err
						}
						time.Sleep(time.Duration(100*(1<<i)) * time.Millisecond)
					} else {
						break
					}
				}
				if req.MediaIDs != nil && len(*req.MediaIDs) > 0 {
					newBlog.MediaIDs = *req.MediaIDs
				}
			}

			// 2. Tạo blog mới
			if err := newBlog.Create(sessCtx); err != nil {
				return nil, err
			}
			return nil, nil
		})
		return err
	})

	// Response cuối cùng dùng switch
	switch {
	case err == nil:
		_ = newBlog.Preload(nil, "AccountFirst", "MediaFirst", "TagFirst")
		c.JSON(http.StatusCreated, dto.ApiResponse{
			Status:  http.StatusCreated,
			Message: "Blog đã được tạo.",
			Data:    newBlog.ParseEntry(),
		})
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi hệ thống!",
			Error:   err.Error(),
		})
	}
}

func UpdateBlog(c *gin.Context) {
	var (
		err        error
		blogEntry  = &collections.Blog{}
		mediaEntry = &collections.Media{}
		maxRetry   = 3
	)

	// LẤY VÀ KIỂM TRA BLOG ID
	blogIDStr := c.Param("id")
	blogID, err := primitive.ObjectIDFromHex(blogIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Blog ID không hợp lệ!",
			Error:   err.Error(),
		})
		return
	}

	// BIND VÀ VALIDATE REQUEST BODY
	var req dto.UpdateBlogRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	if validateErrs := utils.ValidateUpdateBlogRequest(req); len(validateErrs) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": validateErrs,
		})
		return
	}

	// LẤY THÔNG TIN USER TỪ CONTEXT
	updatorID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	//Lấy roles từ context
	roles, err := utils.GetRoles(c)
	if err != nil {
		return
	}

	// Kiểm tra blog hiện tại có tồn tại không
	var (
		blogFilter = bson.M{
			"_id": blogID,
			"deleted_at": bson.M{
				"$exists": false,
			},
		}
	)
	err = blogEntry.First(nil, blogFilter)
	switch {
	case err == nil:
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, dto.ApiResponse{
			Status:  http.StatusNotFound,
			Message: "Không tìm thấy blog hoặc blog đã bị xóa!",
			Error:   err.Error(),
		})
		return
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi từ hệ thống!",
			Error:   err.Error(),
		})
		return
	}

	// Kiểm tra quyền xem thử account đó có phải chính chủ hay có quyền admin
	if !utils.CanModifyResource(blogEntry.CreatedBy, updatorID, roles) {
		utils.ResponseError(c, http.StatusForbidden, "Bạn không có quyền chỉnh sửa blog này!", nil)
		return
	}

	//Tiến hành update blog
	updateDoc := bson.M{}

	// Thêm các trường cơ bản
	if req.Title != nil {
		updateDoc["title"] = *req.Title
	}
	if req.Content != nil {
		updateDoc["content"] = *req.Content
	}
	if req.ContentHtml != nil {
		updateDoc["content_html"] = *req.ContentHtml
	}

	if req.TagIds != nil {
		updateDoc["tag_ids"] = *req.TagIds
	}

	// Logic Diff cho Thumbnail
	mediaIdsToUpdate := []primitive.ObjectID{}
	mediaIdsToDelete := []primitive.ObjectID{}
	if req.ThumbnailID != nil {
		newThumbnailID := *req.ThumbnailID
		oldThumbnailID := blogEntry.ThumbnailID
		if newThumbnailID.IsZero() {
			if !oldThumbnailID.IsZero() {
				mediaIdsToDelete = append(mediaIdsToDelete, oldThumbnailID)
			}
			updateDoc["thumbnail_url"] = ""
			updateDoc["thumbnail_id"] = primitive.NilObjectID
		} else if newThumbnailID != oldThumbnailID {
			err = mediaEntry.First(nil, bson.M{
				"_id": req.ThumbnailID,
			})
			switch {
			case err == nil:
			case errors.Is(err, mongo.ErrNoDocuments):
				utils.ResponseError(c, http.StatusBadRequest, "", fmt.Errorf("Không tìm thấy thumbnail: %v", err.Error()))
				return
			default:
				utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
				return
			}
			if mediaEntry.Type != consts.MEDIA_IMAGE {
				utils.ResponseError(c, http.StatusBadRequest, "", "Thumbnail phải là ảnh")
				return
			}
			updateDoc["thumbnail_url"] = mediaEntry.Url
			updateDoc["thumbnail_id"] = *req.ThumbnailID
			mediaIdsToUpdate = append(mediaIdsToUpdate, newThumbnailID)
			if !oldThumbnailID.IsZero() {
				mediaIdsToDelete = append(mediaIdsToDelete, oldThumbnailID)
			}
		}
	}
	// Logic Diff cho ảnh nội dung
	if req.MediaIDs != nil {
		oldMediaIDs := make(map[primitive.ObjectID]bool)
		newMediaIDs := make(map[primitive.ObjectID]bool)
		for _, m := range blogEntry.MediaIDs {
			oldMediaIDs[m] = true
		}
		for _, m := range *req.MediaIDs {
			newMediaIDs[m] = true
			if !oldMediaIDs[m] {
				mediaIdsToUpdate = append(mediaIdsToUpdate, m)
			}
		}
		for oldID := range oldMediaIDs {
			if !newMediaIDs[oldID] {
				mediaIdsToDelete = append(mediaIdsToDelete, oldID)
			}
		}
		updateDoc["media_ids"] = *req.MediaIDs
	}

	// Audit
	updateDoc["updated_at"] = time.Now()
	updateDoc["updated_by"] = updatorID
	updateDoc["is_edit"] = true

	db := database.GetDB()
	session, err := db.Client().StartSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi khi bắt đầu session",
			Error:   err.Error(),
		})
		return
	}
	defer session.EndSession(c)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Transaction
	err = mongo.WithSession(ctx, session, func(sessCtx mongo.SessionContext) error {
		_, err := sessCtx.WithTransaction(sessCtx, func(sessCtx mongo.SessionContext) (interface{}, error) {
			// Update media mới
			if len(mediaIdsToUpdate) > 0 {
				mediaFilter := bson.M{"_id": bson.M{"$in": mediaIdsToUpdate}, "status": "PENDING"}
				mediaUpdate := bson.M{"$set": bson.M{"status": "SUCCESS"}}
				for i := 0; i < maxRetry; i++ {
					if err := mediaEntry.UpdateMany(sessCtx, mediaFilter, mediaUpdate); err != nil {
						if !mongo.IsTimeout(err) && !mongo.IsNetworkError(err) {
							return nil, err
						}
						time.Sleep(time.Duration(100*(1<<i)) * time.Millisecond)
					} else {
						break
					}
				}
			}

			// Vô hiệu hóa media cũ
			if len(mediaIdsToDelete) > 0 {
				mediaFilter := bson.M{"_id": bson.M{"$in": mediaIdsToDelete}}
				mediaUpdate := bson.M{"$set": bson.M{"status": "DELETED"}}
				for i := 0; i < maxRetry; i++ {
					if err := mediaEntry.UpdateMany(sessCtx, mediaFilter, mediaUpdate); err != nil {
						if !mongo.IsTimeout(err) && !mongo.IsNetworkError(err) {
							return nil, err
						}
						time.Sleep(time.Duration(100*(1<<i)) * time.Millisecond)
					} else {
						break
					}
				}
			}

			// Update blog chính
			if err := blogEntry.Update(sessCtx, blogFilter, bson.M{"$set": updateDoc}); err != nil {
				return nil, err
			}
			return nil, nil
		})
		return err
	})

	switch {
	case err == nil:
		c.JSON(http.StatusOK, dto.ApiResponse{
			Status:  http.StatusOK,
			Message: "Cập nhật blog thành công",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, dto.ApiResponse{
			Status:  http.StatusNotFound,
			Message: "Không tìm thấy blog để update",
			Error:   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi hệ thống!",
			Error:   err.Error(),
		})
	}
}

func SoftDeleteBlog(c *gin.Context) {
	// Lấy BlogID từ URL param
	var (
		err          error
		blogEntry    = &collections.Blog{}
		commentEntry = &collections.Comment{}
	)

	blogIDStr := c.Param("id")
	blogID, err := primitive.ObjectIDFromHex(blogIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Blog ID không hợp lệ!",
			Error:   err.Error(),
		})
		return
	}

	// Lấy UserID (người thực hiện xóa)
	deletorID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	// Lấy role từ người delete
	roles, err := utils.GetRoles(c)
	if err != nil {
		return
	}

	// Kiểm tra xem blog đó có tồn tại không
	blogFilter := bson.M{"_id": blogID, "deleted_at": bson.M{"$exists": false}}
	err = blogEntry.First(nil, blogFilter)
	switch {
	case err == nil:
		// Blog tồn tại, không làm gì cả
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, dto.ApiResponse{
			Status:  http.StatusNotFound,
			Message: "Không tìm thấy blog hoặc blog đã bị xóa!",
			Error:   err.Error(),
		})
		return
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi từ hệ thống!",
			Error:   err.Error(),
		})
		return
	}

	// Kiểm tra quyền xóa
	if !utils.CanModifyResource(blogEntry.CreatedBy, deletorID, roles) {
		utils.ResponseError(c, http.StatusForbidden, "Bạn không có quyền chỉnh sửa blog này!", nil)
		return
	}

	// Lấy danh sách comment chưa xóa của blog
	commentFilter := bson.M{"blog_id": blogID, "deleted_at": bson.M{"$exists": false}}
	comments, err := commentEntry.Find(commentFilter, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi khi lấy comment",
			Error:   err.Error(),
		})
		return
	}
	commentIDs := make([]primitive.ObjectID, 0, len(comments))
	for _, comment := range comments {
		commentIDs = append(commentIDs, comment.ID)
	}

	// Bắt đầu transaction MongoDB
	db := database.GetDB()
	session, err := db.Client().StartSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi khi khởi tạo session",
			Error:   err.Error(),
		})
		return
	}
	defer session.EndSession(c)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = mongo.WithSession(ctx, session, func(sessCtx mongo.SessionContext) error {
		_, err := sessCtx.WithTransaction(sessCtx, func(sessCtx mongo.SessionContext) (interface{}, error) {
			now := time.Now()

			// Soft delete comment
			if len(commentIDs) > 0 {
				updateCommentFilter := bson.M{"_id": bson.M{"$in": commentIDs}, "deleted_at": bson.M{"$exists": false}}
				updateComment := bson.M{"$set": bson.M{
					"deleted_at": now,
					"deleted_by": deletorID,
					"updated_at": now,
					"updated_by": deletorID,
				}}
				if _, err := commentEntry.UpdateMany(sessCtx, updateCommentFilter, updateComment); err != nil {
					return nil, err
				}
			}

			// Soft delete blog
			blogUpdate := bson.M{"$set": bson.M{
				"deleted_at": now,
				"deleted_by": deletorID,
				"updated_at": now,
				"updated_by": deletorID,
			}}
			if err := blogEntry.Update(sessCtx, blogFilter, blogUpdate); err != nil {
				return nil, err
			}

			return nil, nil
		})
		return err
	})

	switch {
	case err == nil:
		c.JSON(http.StatusOK, dto.ApiResponse{
			Status:  http.StatusOK,
			Message: "Xóa mềm blog và comment thành công.",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, dto.ApiResponse{
			Status:  http.StatusNotFound,
			Message: "Không tìm thấy blog để xóa (có thể đã bị xóa)!",
			Error:   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi hệ thống!",
			Error:   err.Error(),
		})
	}
}

func RestoreBlog(c *gin.Context) {
	// Lấy BlogID từ URL param
	var (
		err          error
		blogEntry    = &collections.Blog{}
		commentEntry = &collections.Comment{}
	)
	blogIDStr := c.Param("id")
	blogID, err := primitive.ObjectIDFromHex(blogIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Blog ID không hợp lệ!",
			Error:   err.Error(),
		})
		return
	}

	// Lấy UserID
	restorerID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}
	// Lấy role
	roles, err := utils.GetRoles(c)
	if err != nil {
		return
	}

	//Kiểm tra xem blog đó có tồn tại VÀ ĐÃ BỊ XÓA không
	var (
		blogFilter = bson.M{
			"_id": blogID,
			"deleted_at": bson.M{
				"$exists": true,
			},
		}
	)

	err = blogEntry.First(nil, blogFilter)
	switch {
	case err == nil:
		// Blog tồn tại, không làm gì cả
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, dto.ApiResponse{
			Status:  http.StatusNotFound,
			Message: "Không tìm thấy blog hoặc blog đã bị xóa!",
			Error:   err.Error(),
		})
		return
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi từ hệ thống!",
			Error:   err.Error(),
		})
		return
	}

	//Kiểm tra quyền
	if !utils.CanModifyResource(blogEntry.CreatedBy, restorerID, roles) {
		utils.ResponseError(c, http.StatusForbidden, "Bạn không có quyền chỉnh sửa blog này!", nil)
		return
	}

	//Lấy danh sách các comment cần khôi phục
	db := database.GetDB()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := db.Client().StartSession()
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}
	defer session.EndSession(ctx)

	// Transaction logic
	callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// Khôi phục các comment bị xóa
		commentFilter := bson.M{
			"blog_id": blogID,
			"deleted_at": bson.M{
				"$exists": true,
			},
		}
		updateComment := bson.M{
			"$unset": bson.M{
				"deleted_at": "",
				"deleted_by": "",
			},
			"$set": bson.M{
				"updated_by": restorerID,
				"updated_at": time.Now(),
			},
		}

		if _, err := commentEntry.UpdateMany(sessCtx, commentFilter, updateComment); err != nil {
			return nil, fmt.Errorf("Lỗi khi khôi phục comment: %v", err)
		}

		// Khôi phục blog
		blogUpdate := bson.M{
			"$unset": bson.M{
				"deleted_at": "",
				"deleted_by": "",
			},
			"$set": bson.M{
				"updated_by": restorerID,
				"updated_at": time.Now(),
			},
		}

		if err := blogEntry.Update(sessCtx, blogFilter, blogUpdate); err != nil {
			return nil, fmt.Errorf("Lỗi khi khôi phục blog: %v", err)
		}

		return nil, nil
	}

	// Chạy transaction
	_, err = session.WithTransaction(ctx, callback)
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", fmt.Sprintf("Lỗi transaction: %v", err))
		return
	}
	utils.ResponseSuccess(c, http.StatusOK, "Khôi phục blog thành công!", nil, nil)
}

func GetListBlogs(c *gin.Context) {
	var (
		blogEntry = &collections.Blog{}
	)
	queryMap := c.Request.URL.Query()
	// Lấy 'page'
	pagination := dto.GetPagination(c, "primary")
	skip := (pagination.Page - 1) * pagination.Length

	//Filter
	filter := bson.M{
		"deleted_at": bson.M{
			"$exists": false,
		},
	}
	dynamicFilter := utils.BuildBlogSearchFilter(queryMap)
	for key, value := range dynamicFilter {
		filter[key] = value
	}

	findOptions := options.Find()
	findOptions.SetLimit(int64(pagination.Length))
	findOptions.SetSkip(int64(skip))

	//Sort
	sorts := utils.BuildSortFilter(queryMap)
	findOptions.SetSort(sorts)

	//Tính toán trang
	totalDocs, err := blogEntry.CountDocuments(nil, filter)
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}
	pagination.TotalDocs = int(totalDocs)
	pagination.BuildPagination()

	//Lấy kết quả tìm kiếm
	results, err := blogEntry.Find(nil, filter, findOptions)
	switch {
	case err == nil && len(results) == 0:
		utils.ResponseError(c, http.StatusNotFound, "", nil)
	case err == nil:
		err = blogEntry.Preload(&results, "AccountFind", "TagFind", "CommentCountFind", "MediaFind")
		if err != nil {
			if err != mongo.ErrNoDocuments {
				utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
				return
			}
		}
		res := []bson.M{}
		for _, blog := range results {
			res = append(res, utils.PrettyJSON(blog.ParseEntry()))
		}
		utils.ResponseSuccess(c, http.StatusOK, "", res, &pagination)
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
	}
}

func GetBlog(c *gin.Context) {
	var (
		id       = c.Param("id")
		idHex, _ = primitive.ObjectIDFromHex(id)
		entry    collections.Blog
		filter   = bson.M{
			"_id": idHex,
			"deleted_at": bson.M{
				"$exists": false,
			},
		}
		err error
	)

	viewerID, exists := c.Get("account_id")
	viewerIDStr := c.ClientIP()
	if exists {
		viewerIDStr = viewerID.(string)
	}

	err = entry.First(nil, filter)
	switch err {
	case nil:
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			wg.Done()
			entry.IncrementBlogView(viewerIDStr)
		}()
		err = entry.Preload(nil, "AccountFirst", "TagFirst", "CommentCountFirst", "CommentFirst", "MediaFirst")
		if err != nil {
			if err != mongo.ErrNoDocuments {
				c.JSON(http.StatusInternalServerError, dto.ApiResponse{
					Status:  http.StatusInternalServerError,
					Message: "Lỗi do hệ thống!",
					Error:   err.Error(),
				})
				return
			}
		}
		wg.Wait()
		c.JSON(http.StatusOK, dto.ApiResponse{
			Status:  http.StatusOK,
			Message: "Lấy dữ liệu thành công!",
			Data:    utils.PrettyJSON(entry.ParseEntry()),
		})
	case mongo.ErrNoDocuments:
		c.JSON(http.StatusNotFound, dto.ApiResponse{
			Status:  http.StatusNotFound,
			Message: "Không tìm thấy dữ liệu!",
			Error:   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi do hệ thống!",
			Error:   err.Error(),
		})
	}
}

func GetCommentFromBlog(c *gin.Context) {
	var (
		commentEntry       = &collections.Comment{}
		limit        int64 = 10
	)

	blogIDParam := c.Param("id")
	blogID, err := primitive.ObjectIDFromHex(blogIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Blog ID không hợp lệ!",
		})
		return
	}

	lastIDParam := c.Query("last_id")
	var lastID primitive.ObjectID

	if lastIDParam != "" {
		lastID, err = primitive.ObjectIDFromHex(lastIDParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ApiResponse{
				Status:  http.StatusBadRequest,
				Message: "Last ID không hợp lệ!",
			})
			return
		}
	}

	filter := bson.M{
		"blog_id":    blogID,
		"parent_id":  bson.M{"$exists": false},
		"deleted_at": bson.M{"$exists": false},
	}

	if !lastID.IsZero() {
		filter["_id"] = bson.M{"$gt": lastID}
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"_id", 1}})
	findOptions.SetLimit(limit + 1)

	// Thực thi truy vấn
	comments, err := commentEntry.Find(filter, findOptions)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusOK, dto.ApiResponse{
				Status:  http.StatusOK,
				Message: "Không tìm thấy replies nào.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi hệ thống khi lấy replies!",
			Error:   err.Error(),
		})
		return
	}

	if len(comments) == 0 {
		utils.ResponseError(c, http.StatusNotFound, "Không tìm thấy comment của bài viết!", nil)
		return
	}

	//Xử lý preload
	err = commentEntry.Preload(comments, "AccountFind", "MediaFind")
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi do hệ thống!",
			Error:   err.Error(),
		})
		return
	}

	commentsRes := make([]bson.M, 0)
	for _, cmt := range comments {
		commentsRes = append(commentsRes, utils.PrettyJSON(cmt.ParseEntry()))
	}

	// Xử lý kết quả cho hasMore
	hasMore := false
	if len(commentsRes) > int(limit) {
		hasMore = true
		commentsRes = commentsRes[:limit]
	}

	var nextLastID primitive.ObjectID

	if len(commentsRes) > 0 {
		nextLastID = comments[len(commentsRes)-1].ID
	}

	// Trả về kết quả
	c.JSON(http.StatusOK, dto.ApiResponse{
		Status:  http.StatusOK,
		Message: "Lấy replies thành công.",
		Data:    commentsRes,
		Pagination: &dto.Pagination{
			HasMore: hasMore,
			LastId:  nextLastID.Hex(),
		},
	})
}

func LockComment(c *gin.Context) {
	var (
		err       error
		blogEntry = &collections.Blog{}
	)

	// LẤY VÀ KIỂM TRA BLOG ID
	blogIDStr := c.Param("id")
	blogID, err := primitive.ObjectIDFromHex(blogIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Blog ID không hợp lệ!",
			Error:   err.Error(),
		})
		return
	}

	// LẤY THÔNG TIN USER TỪ CONTEXT
	updatorID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	//Lấy roles từ context
	roles, err := utils.GetRoles(c)
	if err != nil {
		return
	}

	// Kiểm tra blog hiện tại có tồn tại không
	var (
		blogFilter = bson.M{
			"_id": blogID,
			"deleted_at": bson.M{
				"$exists": false,
			},
		}
	)

	err = blogEntry.First(nil, blogFilter)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, dto.ApiResponse{
				Status:  http.StatusNotFound,
				Message: "Không tìm thấy blog hoặc blog đã bị xóa!",
				Error:   err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi từ hệ thống!",
			Error:   err.Error(),
		})
		return
	}

	// Kiểm tra quyền xem thử account đó có phải chính chủ hay có quyền admin
	if !utils.CanModifyResource(blogEntry.CreatedBy, updatorID, roles) {
		utils.ResponseError(c, http.StatusForbidden, "Bạn không có quyền chỉnh sửa blog này!", nil)
		return
	}

	if blogEntry.IsLockComment {
		utils.ResponseError(c, http.StatusBadRequest, "", "Bài viết này hiện tại đã khóa bình luận!")
		return
	}

	err = blogEntry.Update(nil, blogFilter, bson.M{
		"$set": bson.M{
			"is_lock_comment": true,
			"updated_at":      time.Now(),
			"updated_by":      updatorID,
		},
	})

	if err != nil {
		if err != mongo.ErrNoDocuments {
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
			return
		}
	}
	utils.ResponseSuccess(c, http.StatusOK, "Khóa bình luận thành công.", nil, nil)
}

func UnLockComment(c *gin.Context) {
	var (
		err       error
		blogEntry = &collections.Blog{}
	)

	// LẤY VÀ KIỂM TRA BLOG ID
	blogIDStr := c.Param("id")
	blogID, err := primitive.ObjectIDFromHex(blogIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Blog ID không hợp lệ!",
			Error:   err.Error(),
		})
		return
	}

	// LẤY THÔNG TIN USER TỪ CONTEXT
	updatorID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	//Lấy roles từ context
	roles, err := utils.GetRoles(c)
	if err != nil {
		return
	}

	// Kiểm tra blog hiện tại có tồn tại không
	var (
		blogFilter = bson.M{
			"_id": blogID,
			"deleted_at": bson.M{
				"$exists": false,
			},
		}
	)

	err = blogEntry.First(nil, blogFilter)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, dto.ApiResponse{
				Status:  http.StatusNotFound,
				Message: "Không tìm thấy blog hoặc blog đã bị xóa!",
				Error:   err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi từ hệ thống!",
			Error:   err.Error(),
		})
		return
	}

	// Kiểm tra quyền xem thử account đó có phải chính chủ hay có quyền admin
	if !utils.CanModifyResource(blogEntry.CreatedBy, updatorID, roles) {
		utils.ResponseError(c, http.StatusForbidden, "Bạn không có quyền chỉnh sửa blog này!", nil)
		return
	}

	if !blogEntry.IsLockComment {
		utils.ResponseError(c, http.StatusBadRequest, "", "Bài viết này hiện tại vẫn chưa khóa bình luận!")
		return
	}

	err = blogEntry.Update(nil, blogFilter, bson.M{
		"$set": bson.M{
			"is_lock_comment": false,
			"updated_at":      time.Now(),
			"updated_by":      updatorID,
		},
	})

	if err != nil {
		if err != mongo.ErrNoDocuments {
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
			return
		}
	}

	utils.ResponseSuccess(c, http.StatusOK, "Mở bình luận thành công.", nil, nil)
}
