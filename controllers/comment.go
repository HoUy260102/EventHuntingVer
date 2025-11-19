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
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CreateComment(c *gin.Context) {
	var (
		blogEntry     = &collections.Blog{}
		eventEntry    = &collections.Event{}
		err           error
		commentEntry  = &collections.Comment{}
		mediaEntry    = &collections.Media{}
		maxRetry      = 3
		invalidErrors []string
	)

	// LẤY ID NGƯỜI TẠO
	creatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	// BIND DỮ LIỆU TỪ BODY
	var req dto.CommentCreateReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Dữ liệu đầu vào không hợp lệ!",
			Error:   err.Error(),
		})
		return
	}

	//Valide dữ liệu đầu vào
	if invalidErrors = utils.ValidateCommentCreate(req); len(invalidErrors) > 0 {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Lỗi nhập sai dữ liệu!",
			Error:   invalidErrors,
		})
		return
	}

	if !req.DocumentID.IsZero() {
		filterBlog := bson.M{
			"_id":        req.DocumentID,
			"deleted_at": bson.M{"$exists": false},
		}
		if req.Category == consts.COMMENT_TYPE_BLOG {
			err = blogEntry.First(nil, filterBlog)
		}
		if req.Category == consts.COMMENT_TYPE_EVENT {
			err = eventEntry.First(nil, filterBlog)
		}
		switch {
		case err == nil:
			if blogEntry.IsLockComment {
				utils.ResponseError(c, http.StatusBadRequest, "", "Đang được khóa bình luận!")
				return
			}
		case errors.Is(err, mongo.ErrNoDocuments):
			c.JSON(http.StatusNotFound, dto.ApiResponse{
				Status:  http.StatusNotFound,
				Message: "Document không tồn tại hoặc đã bị xóa!",
				Error:   err.Error(),
			})
			return
		default:
			c.JSON(http.StatusInternalServerError, dto.ApiResponse{
				Status:  http.StatusInternalServerError,
				Message: "Lỗi hệ thống!",
				Error:   err.Error(),
			})
			return
		}
	}

	//Kiểm tra comment parent id có tồn tại không
	if req.ParentID.IsZero() {
		req.ParentID = primitive.NilObjectID
	} else {
		filter := bson.M{
			"_id":        req.ParentID,
			"deleted_at": bson.M{"$exists": false},
		}
		switch err := commentEntry.First(filter); {
		case err == nil:
		case errors.Is(err, mongo.ErrNoDocuments):
			c.JSON(http.StatusNotFound, dto.ApiResponse{
				Status:  http.StatusNotFound,
				Message: "Comment cha không tồn tại hoặc đã bị xóa!",
				Error:   err.Error(),
			})
			return
		default:
			c.JSON(http.StatusInternalServerError, dto.ApiResponse{
				Status:  http.StatusInternalServerError,
				Message: "Lỗi hệ thống khi kiểm tra comment cha!",
				Error:   err.Error(),
			})
			return
		}
	}

	// Tạo comment mới
	now := time.Now()
	newComment := collections.Comment{
		ParentID:    req.ParentID,
		Content:     req.Content,
		ContentHTML: req.ContentHTML,
		DocumentID:  req.DocumentID,
		IsEdit:      false,
		CreatedAt:   now,
		CreatedBy:   creatorObjectId,
		UpdatedAt:   now,
		UpdatedBy:   creatorObjectId,
		Category:    req.Category,
		ReplyCount:  0,
	}

	//Kiểm tra các media có phải là media hợp le
	mediaIDs := []primitive.ObjectID{}
	if len(req.MediaIds) > 0 {
		existedMediaIDMap := make(map[primitive.ObjectID]struct{})
		validMediaFilter := bson.M{
			"_id": bson.M{
				"$in": req.MediaIds,
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
		for _, mediaID := range req.MediaIds {
			if _, ok := existedMediaIDMap[mediaID]; !ok {
				invalidMeida = append(invalidMeida, mediaID.Hex())
			}
		}
		if len(medias) != len(req.MediaIds) {
			utils.ResponseError(c, http.StatusBadRequest, "", fmt.Errorf("MediaIDs [%s] không hợp lệ", strings.Join(invalidMeida, ", ")))
			return
		}
	}

	if len(mediaIDs) > 0 {
		newComment.MediaIds = mediaIDs
	}

	// Transaction
	db := database.GetDB()
	session, err := db.Client().StartSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi khi bắt đầu transaction!",
			Error:   err.Error(),
		})
		return
	}
	defer session.EndSession(c)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Transaction giống DeleteComment
	err = mongo.WithSession(ctx, session, func(sessCtx mongo.SessionContext) error {
		_, err := sessCtx.WithTransaction(sessCtx, func(sessCtx mongo.SessionContext) (interface{}, error) {
			// 1. Update media
			if len(req.MediaIds) > 0 {
				mediaFilter := bson.M{
					"_id":    bson.M{"$in": req.MediaIds},
					"status": "PENDING",
				}
				mediaUpdate := bson.M{"$set": bson.M{"status": consts.MediaStatusSuccess}}

				const baseBackoff = 100 * time.Millisecond
				for i := 0; i < maxRetry; i++ {
					mediaErr := mediaEntry.UpdateMany(sessCtx, mediaFilter, mediaUpdate)
					if mediaErr == nil {
						break
					}
					if !mongo.IsTimeout(mediaErr) && !mongo.IsNetworkError(mediaErr) {
						return nil, mediaErr
					}
					time.Sleep(baseBackoff * time.Duration(1<<i))
				}
			}

			// 2. Create comment
			if err := newComment.Create(sessCtx); err != nil {
				return nil, err
			}

			// 3. Update reply_count comment cha
			if !newComment.ParentID.IsZero() {
				commentParentEntry := &collections.Comment{}
				filterParent := bson.M{"_id": newComment.ParentID}
				updateParent := bson.M{"$inc": bson.M{"reply_count": 1}}
				for i := 0; i < maxRetry; i++ {
					retryErr := commentParentEntry.Update(sessCtx, filterParent, updateParent)
					if retryErr == nil {
						break
					}
				}
			}

			return nil, nil
		})
		return err
	})

	// XỬ LÝ KẾT QUẢ TRẢ VỀ
	switch {
	case err == nil:
		err = newComment.Preload(nil, "AccountFirst", "MediaFirst")
		c.JSON(http.StatusCreated, dto.ApiResponse{
			Status:  http.StatusCreated,
			Message: "Tạo bình luận thành công.",
			Data:    utils.PrettyJSON(newComment.ParseEntry()),
		})
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi do hệ thống khi tạo comment với transaction!",
			Error:   err.Error(),
		})
	}
}

func UpdateComment(c *gin.Context) {
	var (
		commentEntry *collections.Comment = &collections.Comment{}
		mediaEntry   *collections.Media   = &collections.Media{}
		maxRetry                          = 3
	)

	// LẤY ID NGƯỜI CẬP NHẬT
	updatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	// Lấy roles từ context
	roles, err := utils.GetRoles(c)
	if err != nil {
		return
	}

	// LẤY ID COMMENT TỪ URL
	commentId := c.Param("id")
	commentObjectId, err := primitive.ObjectIDFromHex(commentId)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Id bình luận không hợp lệ!",
		})
		return
	}

	// BIND DỮ LIỆU TỪ BODY
	var req dto.CommentUpdateReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Dữ liệu đầu vào không hợp lệ!",
			Error:   err.Error(),
		})
		return
	}

	//Validate dữ liệu đầu vào
	if invalidErrors := utils.ValidateCommentUpdate(req); len(invalidErrors) > 0 {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Lỗi nhập sai dữ liệu!",
			Error:   invalidErrors,
		})
		return
	}

	// KIỂM TRA COMMENT TỒN TẠI
	filterComment := bson.M{
		"_id":        commentObjectId,
		"deleted_at": bson.M{"$exists": false},
	}

	err = commentEntry.First(filterComment)
	switch {
	case err == nil:
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, dto.ApiResponse{
			Status:  http.StatusNotFound,
			Message: "Bình luận không tồn tại hoặc đã bị xóa!",
			Error:   err.Error(),
		})
		return
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi do hệ thống khi kiểm tra bình luận!",
			Error:   err.Error(),
		})
		return
	}

	// KIỂM TRA QUYỀN SỞ HỮU
	if !utils.CanModifyResource(commentEntry.CreatedBy, updatorObjectId, roles) {
		utils.ResponseError(c, http.StatusForbidden, "Bạn không có quyền chỉnh sửa bình luận này!", nil)
		return
	}

	setData := bson.M{
		"is_edit":    true,
		"updated_at": time.Now(),
		"updated_by": updatorObjectId,
	}

	if req.Content != nil {
		setData["content"] = *req.Content
	}

	if req.ContentHTML != nil {
		setData["content_html"] = *req.ContentHTML
	}

	oldMediaIDs := make(map[primitive.ObjectID]bool)
	newMediaIDs := make(map[primitive.ObjectID]bool)
	mediaIdsToUpdate := []primitive.ObjectID{}
	mediaIdsToDelete := []primitive.ObjectID{}

	if req.MediaIds != nil {
		for _, m := range commentEntry.MediaIds {
			oldMediaIDs[m] = true
		}
		for _, m := range *req.MediaIds {
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
		setData["media_ids"] = *req.MediaIds
	}

	updateData := bson.M{"$set": setData}

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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err = mongo.WithSession(ctx, session, func(sessCtx mongo.SessionContext) error {
		_, err := sessCtx.WithTransaction(sessCtx, func(sessCtx mongo.SessionContext) (interface{}, error) {
			// 1. Cập nhật media mới
			if len(mediaIdsToUpdate) > 0 {
				mediaFilter := bson.M{
					"_id":    bson.M{"$in": mediaIdsToUpdate},
					"status": "PENDING",
				}
				mediaUpdate := bson.M{"$set": bson.M{"status": consts.MediaStatusSuccess}}
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

			// 2. Vô hiệu hóa media cũ
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

			// 3. Update comment chính
			if err := commentEntry.Update(sessCtx, filterComment, updateData); err != nil {
				return nil, err
			}

			return nil, nil
		})
		return err
	})

	// Response cuối cùng dùng switch
	switch {
	case err == nil:
		c.JSON(http.StatusOK, dto.ApiResponse{
			Status:  http.StatusOK,
			Message: "Cập nhật bình luận thành công.",
		})
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, dto.ApiResponse{
			Status:  http.StatusNotFound,
			Message: "Không tìm thấy bình luận để cập nhật (có thể đã bị xóa)!",
			Error:   err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi hệ thống khi cập nhật bình luận!",
			Error:   err.Error(),
		})
	}
}

func SoftDeleteComment(c *gin.Context) {
	var (
		commentEntry *collections.Comment = &collections.Comment{}
		maxRetry                          = 3
	)

	// Lấy ID bình luận từ URL
	commentIDParam := c.Param("id")
	commentID, err := primitive.ObjectIDFromHex(commentIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Comment ID không hợp lệ!",
		})
		return
	}

	// LẤY ID NGƯỜI XÓA
	deleterObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	roles, err := utils.GetRoles(c)
	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "", err.Error())
		return
	}

	filterFind := bson.M{
		"_id":        commentID,
		"deleted_at": bson.M{"$exists": false},
	}

	// Lấy comment
	switch err = commentEntry.First(filterFind); {
	case err == nil:
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, dto.ApiResponse{
			Status:  http.StatusNotFound,
			Message: "Bình luận không tồn tại hoặc đã bị xóa!",
		})
		return
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi hệ thống khi tìm bình luận!",
			Error:   err.Error(),
		})
		return
	}

	//Kiềm tra có quyền xóa comment này không
	if !utils.CanModifyResource(commentEntry.CreatedBy, deleterObjectId, roles) {
		utils.ResponseError(c, http.StatusForbidden, "Bạn không có quyền chỉnh sửa bình luận này!", nil)
		return
	}

	parentIDToUpdate := commentEntry.ParentID

	//THỰC HIỆN XÓA MỀM
	now := time.Now()
	filterDelete := bson.M{
		"$or": []bson.M{
			{"_id": commentID},
			{"parent_id": commentID},
		},
		"deleted_at": bson.M{"$exists": false},
	}
	updateDelete := bson.M{
		"$set": bson.M{
			"deleted_at": now,
			"deleted_by": deleterObjectId,
		},
	}

	// Thực thi lệnh xóa mềm hàng loạt
	res, err := commentEntry.UpdateMany(nil, filterDelete, updateDelete)

	switch {
	case err != nil:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi hệ thống khi xóa bình luận!",
			Error:   err.Error(),
		})
	case res == 0:
		c.JSON(http.StatusNotFound, dto.ApiResponse{
			Status:  http.StatusNotFound,
			Message: "Không tìm thấy bình luận nào để xóa!",
		})
	default:
		if !parentIDToUpdate.IsZero() {
			go func(parentID primitive.ObjectID) {
				commentParentEntry := &collections.Comment{}
				filterParent := bson.M{"_id": parentID}
				updateParent := bson.M{"$inc": bson.M{"reply_count": -1}}

				for i := 0; i < maxRetry; i++ {
					retryErr := commentParentEntry.Update(nil, filterParent, updateParent)
					if retryErr == nil {
						break
					}
					log.Printf("Lỗi khi cập nhật (giảm) reply_count (lần %d): %v", i+1, retryErr)
					time.Sleep(100 * time.Millisecond)
				}
			}(parentIDToUpdate)
		}

		// Trả về JSON cho người dùng *NGAY LẬP TỨC*
		c.JSON(http.StatusOK, dto.ApiResponse{
			Status:  http.StatusOK,
			Message: "Xóa mềm bình luận và các trả lời thành công.",
		})
	}
}

func RestoreComment(c *gin.Context) {
	var (
		commentEntry *collections.Comment = &collections.Comment{}
		maxRetry                          = 3
	)

	// Lấy ID bình luận từ URL
	commentIDParam := c.Param("id")
	commentID, err := primitive.ObjectIDFromHex(commentIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Comment ID không hợp lệ!",
		})
		return
	}

	//Lấy id người restore
	// LẤY ID NGƯỜI XÓA
	restorerObjectId, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	roles, err := utils.GetRoles(c)
	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "", err.Error())
		return
	}

	filterFind := bson.M{
		"_id":        commentID,
		"deleted_at": bson.M{"$exists": true},
	}
	// Gán dữ liệu tìm được vào commentEntry
	switch err = commentEntry.First(filterFind); {
	case err == nil:
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, dto.ApiResponse{
			Status:  http.StatusNotFound,
			Message: "Bình luận không tồn tại hoặc đã bị xóa!",
		})
		return
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi hệ thống khi tìm bình luận!",
			Error:   err.Error(),
		})
		return
	}

	//Kiểm tra quyền chỉnh sửa
	if !utils.CanModifyResource(commentEntry.CreatedBy, restorerObjectId, roles) {
		utils.ResponseError(c, http.StatusForbidden, "Bạn không có quyền chỉnh sửa bình luận này!", nil)
		return
	}

	parentIDToUpdate := commentEntry.ParentID

	filterRestore := bson.M{
		"$or": []bson.M{
			{"_id": commentID},
			{"parent_id": commentID},
		},
		"deleted_at": bson.M{"$exists": true},
	}
	updateRestore := bson.M{
		"$set": bson.M{
			"updated_at": time.Now(),
			"updated_by": restorerObjectId,
		},
		"$unset": bson.M{
			"deleted_at": "",
			"deleted_by": "",
		},
	}

	// Thực thi lệnh khôi phục hàng loạt
	res, err := commentEntry.UpdateMany(nil, filterRestore, updateRestore)

	switch {
	case err != nil:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi hệ thống khi khôi phục bình luận!",
			Error:   err.Error(),
		})
	case res == 0:
		c.JSON(http.StatusNotFound, dto.ApiResponse{
			Status:  http.StatusNotFound,
			Message: "Không tìm thấy bình luận nào đã bị xóa để khôi phục!",
		})
	default:

		if !parentIDToUpdate.IsZero() {
			go func(parentID primitive.ObjectID) {
				commentParentEntry := &collections.Comment{}
				filterParent := bson.M{"_id": parentID, "deleted_at": bson.M{
					"$exists": false,
				}}
				updateParent := bson.M{"$inc": bson.M{"reply_count": 1}} // Tăng 1

				for i := 0; i < maxRetry; i++ {
					retryErr := commentParentEntry.Update(nil, filterParent, updateParent)
					if retryErr == nil {
						break
					}
					log.Printf("Lỗi khi cập nhật (tăng) reply_count (lần %d): %v", i+1, retryErr)
					time.Sleep(100 * time.Millisecond)
				}
			}(parentIDToUpdate)
		}

		c.JSON(http.StatusOK, dto.ApiResponse{
			Status:  http.StatusOK,
			Message: "Khôi phục bình luận và các trả lời thành công.",
		})
	}
}

func GetCommentReplies(c *gin.Context) {
	var (
		commentEntry = &collections.Comment{}
		lastId       primitive.ObjectID
		err          error
	)

	pagination := dto.GetPagination(c, "primary")
	if pagination.LastId == "" {
		lastId = primitive.NilObjectID
	} else {
		lastId, err = primitive.ObjectIDFromHex(pagination.LastId)
		if err != nil {
			utils.ResponseError(c, http.StatusBadRequest, "", nil)
			return
		}
	}

	parentIDParam := c.Param("id")
	parentID, err := primitive.ObjectIDFromHex(parentIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Parent Comment ID không hợp lệ!",
		})
		return
	}

	filter := bson.M{
		"parent_id":  parentID,
		"deleted_at": bson.M{"$exists": false},
	}

	if !lastId.IsZero() {
		filter["_id"] = bson.M{"$gt": lastId}
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"_id", 1}})
	findOptions.SetLimit(int64(pagination.Length) + 1)

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
		commentsRes = append(commentsRes, cmt.ParseEntry())
	}

	pagination.TotalDocs = len(commentsRes)

	if len(commentsRes) > pagination.Length {
		commentsRes = commentsRes[:pagination.Length]
	}

	if len(commentsRes) > 0 {
		pagination.LastId = comments[len(commentsRes)-1].ID.Hex()
	}

	pagination.BuildPagination()
	utils.ResponseSuccess(c, http.StatusOK, "", commentsRes, &pagination)
}

func createMediasWithRetry(mediaEntry *collections.Media, medias []collections.Media) error {
	var mediaErr error
	const (
		maxRetries  = 3
		baseBackoff = 100 * time.Millisecond
	)
	for i := 0; i < maxRetries; i++ {
		opts := options.InsertMany().SetOrdered(false)
		mediaErr = mediaEntry.CreateMany(nil, medias, opts)
		if mediaErr == nil {
			return nil
		}
		if !mongo.IsTimeout(mediaErr) && !mongo.IsNetworkError(mediaErr) {
			return mediaErr
		}
		log.Printf("createMediasWithRetry: lỗi tạm thời (lần %d): %v.", i+1, mediaErr)
		if i == maxRetries-1 {
			break
		}
		time.Sleep(baseBackoff * time.Duration(1<<i))
	}
	return mediaErr
}

func deactivateMediasWithRetry(mediaEntry *collections.Media, urls []string, collectionName string) error {
	var mediaErr error
	const (
		maxRetries  = 3
		baseBackoff = 100 * time.Millisecond
	)

	filter := bson.M{
		"url":             bson.M{"$in": urls},
		"collection_name": collectionName,
	}
	update := bson.M{"$set": bson.M{"status": "inactive", "deleted_at": time.Now()}}

	for i := 0; i < maxRetries; i++ {
		mediaErr = mediaEntry.UpdateMany(nil, filter, update)
		if mediaErr == nil {
			return nil
		}
		if !mongo.IsTimeout(mediaErr) && !mongo.IsNetworkError(mediaErr) {
			return mediaErr
		}
		log.Printf("deactivateMediasWithRetry: lỗi tạm thời (lần %d): %v.", i+1, mediaErr)
		if i == maxRetries-1 {
			break
		}
		time.Sleep(baseBackoff * time.Duration(1<<i))
	}
	return mediaErr
}

//func CreateComment(c *gin.Context) {
//	var (
//		blogEntry    = &collections.Blog{}
//		commentEntry = &collections.Comment{}
//	)
//
//	// LẤY ID NGƯỜI TẠO
//	creatorObjectId, ok := utils.GetAccountID(c)
//	if !ok {
//		return
//	}
//
//	// BIND DỮ LIỆU TỪ BODY
//	var req dto.CommentCreateReq
//	if err := c.BindJSON(&req); err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{
//			"status":  http.StatusBadRequest,
//			"message": "Dữ liệu đầu vào không hợp lệ",
//			"error":   err.Error(),
//		})
//		return
//	}
//
//	// KIỂM TRA DỮ LIỆU CƠ BẢN
//	if req.DocumentID.IsZero() && req.Category == consts.COMMENT_TYPE_BLOG {
//		c.JSON(http.StatusBadRequest, gin.H{
//			"status":  http.StatusBadRequest,
//			"message": "DocumentID là bắt buộc",
//		})
//		return
//	}
//
//	// KIỂM TRA BÀI VIẾT (BLOG)
//	if !req.DocumentID.IsZero() {
//		filterBlog := bson.M{
//			"_id":        req.DocumentID,
//			"deleted_at": bson.M{"$exists": false},
//		}
//		checkBlogExisted := blogEntry.First(filterBlog)
//		if checkBlogExisted != nil {
//			if errors.Is(checkBlogExisted, mongo.ErrNoDocuments) {
//				c.JSON(http.StatusNotFound, gin.H{
//					"status":  http.StatusNotFound,
//					"message": "Bài viết không tồn tại hoặc đã bị xóa!",
//					"error":   checkBlogExisted.Error(),
//				})
//				return
//			}
//			c.JSON(http.StatusInternalServerError, gin.H{
//				"status":  http.StatusInternalServerError,
//				"message": "Lỗi do hệ thống khi kiểm tra bài viết!",
//				"error":   checkBlogExisted.Error(),
//			})
//			return
//		}
//	}
//
//	var ancestorIDs []primitive.ObjectID
//
//	//Kiểm tra comment parent id có tồn tại không
//	if req.ParentID.IsZero() {
//		ancestorIDs = make([]primitive.ObjectID, 0)
//	} else {
//		filter := bson.M{
//			"_id":        req.ParentID,
//			"deleted_at": bson.M{"$exists": false},
//		}
//		checkParentExisted := commentEntry.First(filter)
//		if checkParentExisted != nil {
//			if errors.Is(checkParentExisted, mongo.ErrNoDocuments) {
//				c.JSON(http.StatusNotFound, gin.H{
//					"status":  http.StatusNotFound,
//					"message": "Comment cha không tồn tại hoặc đã bị xóa!",
//					"error":   checkParentExisted.Error(),
//				})
//				return
//			}
//			c.JSON(http.StatusInternalServerError, gin.H{
//				"status":  http.StatusInternalServerError,
//				"message": "Lỗi do hệ thống khi kiểm tra bài viết!",
//				"error":   checkParentExisted.Error(),
//			})
//			return
//		}
//		if commentEntry.AncestorIDs == nil {
//			commentEntry.AncestorIDs = make([]primitive.ObjectID, 0)
//		}
//		ancestorIDs = append(commentEntry.AncestorIDs, commentEntry.ID)
//	}
//
//	// CHUẨN BỊ DỮ LIỆU MEDIAS
//	newMedias := []collections.Media{}
//	commentMedias := []struct {
//		Type   consts.MediaFormat `bson:"type" json:"type"`
//		Url    string             `bson:"url" json:"url"`
//		Status consts.MediaStatus `bson:"status" json:"status"`
//	}{}
//
//	for _, reqMedia := range req.Medias {
//		commentMedias = append(commentMedias, struct {
//			Type   consts.MediaFormat `bson:"type" json:"type"`
//			Url    string             `bson:"url" json:"url"`
//			Status consts.MediaStatus `bson:"status" json:"status"`
//		}{
//			Type:   reqMedia.Type,
//			Url:    reqMedia.Url,
//			Status: reqMedia.Status,
//		})
//		if reqMedia.Status == consts.MediaStatusSuccess {
//			newMedia := collections.Media{
//				ID:             primitive.NewObjectID(),
//				Url:            reqMedia.Url,
//				PublicUrlId:    reqMedia.PublicUrlId,
//				Type:           reqMedia.Type,
//				Status:         "active",
//				CollectionName: "comments",
//			}
//			newMedias = append(newMedias, newMedia)
//		}
//	}
//
//	// LƯU MEDIAS
//	if len(newMedias) > 0 {
//		var (
//			mediaEntry = &collections.Media{}
//			mediaErr   error
//		)
//
//		const (
//			maxRetries  = 3
//			baseBackoff = 100 * time.Millisecond
//		)
//
//		// Vòng lặp Retry
//		for i := 0; i < maxRetries; i++ {
//			opts := options.InsertMany().SetOrdered(false)
//			mediaErr = mediaEntry.CreateMany(nil, newMedias, opts)
//
//			if mediaErr == nil {
//				break
//			}
//
//			if !mongo.IsTimeout(mediaErr) && !mongo.IsNetworkError(mediaErr) {
//				break
//			}
//
//			log.Printf("CreateComment (Media): lỗi tạm thời (lần %d): %v. Đang thử lại...", i+1, mediaErr)
//
//			if i == maxRetries-1 {
//				break
//			}
//
//			backoff := baseBackoff * time.Duration(1<<i)
//			time.Sleep(backoff)
//		}
//
//		// Kiểm tra lỗi cuối cùng sau khi thoát vòng lặp
//		if mediaErr != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{
//				"status":  http.StatusInternalServerError,
//				"message": "Lỗi do hệ thống khi lưu media sau nhiều lần thử!",
//				"error":   mediaErr.Error(),
//			})
//			return
//		}
//	}
//
//	// CHUẨN BỊ DỮ LIỆU COMMENT CHÍNH
//	now := time.Now()
//	newComment := collections.Comment{
//		ParentID:    req.ParentID,
//		Content:     req.Content,
//		ContentHTML: req.ContentHTML,
//		DocumentID:      req.DocumentID,
//		Medias:      commentMedias,
//		IsEdit:      false,
//		CreatedAt:   now,
//		CreatedBy:   creatorObjectId,
//		UpdatedAt:   now,
//		UpdatedBy:   creatorObjectId,
//		Category:    req.Category,
//
//		AncestorIDs:     ancestorIDs,
//		ReplyCount:      0,
//		DescendantCount: 0,
//	}
//
//	// LƯU COMMENT CHÍNH
//	err := newComment.Create()
//
//	// XỬ LÝ KẾT QUẢ TRẢ VỀ
//	switch {
//	case err == nil:
//		if len(newComment.AncestorIDs) > 0 {
//			filterDescendants := bson.M{"_id": bson.M{"$in": newComment.AncestorIDs}}
//			updateDescendants := bson.M{"$inc": bson.M{"descendant_count": 1}}
//
//			_, err = commentEntry.UpdateMany(nil, filterDescendants, updateDescendants)
//			if err != nil {
//				log.Printf("LỖI NỀN: Không thể cập nhật descendant_count: %v", err)
//			}
//		}
//		if !newComment.ParentID.IsZero() {
//			filterParent := bson.M{"_id": newComment.ParentID}
//			updateParent := bson.M{"$inc": bson.M{"reply_count": 1}}
//
//			err = commentEntry.Update(filterParent, updateParent)
//			if err != nil {
//				log.Printf("LỖI NỀN: Không thể cập nhật reply_count: %v", err)
//			}
//		}
//		comments := collections.Comments{}
//		err = newComment.Preload(comments, "AccountFirst")
//		c.JSON(http.StatusCreated, gin.H{
//			"status":  http.StatusCreated,
//			"message": "Tạo bình luận thành công.",
//			"data":    utils.PrettyJSON(newComment.ParseEntry()),
//		})
//	default:
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"status":  http.StatusInternalServerError,
//			"message": "Lỗi do hệ thống khi tạo bình luận!",
//			"error":   err.Error(),
//		})
//	}
//}
//
//func UpdateComment(c *gin.Context) {
//	var (
//		commentEntry *collections.Comment = &collections.Comment{}
//		mediaEntry   *collections.Media   = &collections.Media{}
//	)
//
//	// LẤY ID NGƯỜI CẬP NHẬT
//	updatorObjectId, ok := utils.GetAccountID(c)
//	if !ok {
//		return
//	}
//
//	// LẤY ID COMMENT TỪ URL
//	commentId := c.Param("id")
//	commentObjectId, err := primitive.ObjectIDFromHex(commentId)
//	if err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{
//			"status":  http.StatusBadRequest,
//			"message": "Id bình luận không hợp lệ",
//		})
//		return
//	}
//
//	// BIND DỮ LIỆU TỪ BODY
//	var req dto.CommentUpdateReq
//	if err := c.BindJSON(&req); err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{
//			"status":  http.StatusBadRequest,
//			"message": "Dữ liệu đầu vào không hợp lệ",
//			"error":   err.Error(),
//		})
//		return
//	}
//
//	// KIỂM TRA COMMENT TỒN TẠI
//	filterComment := bson.M{
//		"_id":        commentObjectId,
//		"deleted_at": bson.M{"$exists": false},
//	}
//	checkExisted := commentEntry.First(filterComment)
//	if checkExisted != nil {
//		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
//			c.JSON(http.StatusNotFound, gin.H{
//				"status":  http.StatusNotFound,
//				"message": "Bình luận không tồn tại hoặc đã bị xóa!",
//				"error":   checkExisted.Error(),
//			})
//			return
//		}
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"status":  http.StatusInternalServerError,
//			"message": "Lỗi do hệ thống khi kiểm tra bình luận!",
//			"error":   checkExisted.Error(),
//		})
//		return
//	}
//
//	// KIỂM TRA QUYỀN SỞ HỮU
//	if commentEntry.CreatedBy != updatorObjectId {
//		c.JSON(http.StatusForbidden, gin.H{
//			"status":  http.StatusForbidden,
//			"message": "Bạn không có quyền chỉnh sửa bình luận này",
//		})
//		return
//	}
//
//	setData := bson.M{
//		"is_edit":    true,
//		"updated_at": time.Now(),
//		"updated_by": updatorObjectId,
//	}
//
//	if req.Content != nil {
//		setData["content"] = *req.Content
//	}
//	if req.ContentHTML != nil {
//		setData["content_html"] = *req.ContentHTML
//	}
//
//	if req.Medias != nil {
//		oldMediaUrls := make(map[string]bool)
//		for _, m := range commentEntry.Medias {
//			oldMediaUrls[m.Url] = true
//		}
//
//		newMediaUrls := make(map[string]bool)
//		newMediasToInsert := []collections.Media{}
//		finalCommentMedias := []struct {
//			Type   consts.MediaFormat `bson:"type" json:"type"`
//			Url    string             `bson:"url" json:"url"`
//			Status consts.MediaStatus `bson:"status" json:"status"`
//		}{}
//		urlsToDeactivate := []string{}
//
//		for _, reqMedia := range *req.Medias {
//			newMediaUrls[reqMedia.Url] = true
//
//			finalCommentMedias = append(finalCommentMedias, struct {
//				Type   consts.MediaFormat `bson:"type" json:"type"`
//				Url    string             `bson:"url" json:"url"`
//				Status consts.MediaStatus `bson:"status" json:"status"`
//			}{
//				Type:   reqMedia.Type,
//				Url:    reqMedia.Url,
//				Status: reqMedia.Status,
//			})
//
//			// Nếu URL này là mới VÀ thành công
//			if !oldMediaUrls[reqMedia.Url] && reqMedia.Status == consts.MediaStatusSuccess {
//				newMediasToInsert = append(newMediasToInsert, collections.Media{
//					ID:             primitive.NewObjectID(),
//					Url:            reqMedia.Url,
//					PublicUrlId:    reqMedia.PublicUrlId,
//					Type:           reqMedia.Type,
//					Status:         "active",
//					CollectionName: "comments",
//				})
//			}
//		}
//
//		// Lặp qua media CŨ để tìm cái bị xóa
//		for oldUrl := range oldMediaUrls {
//			if !newMediaUrls[oldUrl] {
//				urlsToDeactivate = append(urlsToDeactivate, oldUrl)
//			}
//		}
//
//		// Cập nhật mảng media trong $set
//		setData["medias"] = finalCommentMedias
//
//		// LƯU MEDIA MỚI
//		if len(newMediasToInsert) > 0 {
//			if err := createMediasWithRetry(mediaEntry, newMediasToInsert); err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{
//					"status":  http.StatusInternalServerError,
//					"message": "Lỗi hệ thống khi lưu media mới!",
//					"error":   err.Error(),
//				})
//				return
//			}
//		}
//
//		// VÔ HIỆU HÓA MEDIA CŨ
//		if len(urlsToDeactivate) > 0 {
//			if err := deactivateMediasWithRetry(mediaEntry, urlsToDeactivate); err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{
//					"status":  http.StatusInternalServerError,
//					"message": "Lỗi hệ thống khi vô hiệu hóa media cũ!",
//					"error":   err.Error(),
//				})
//				return
//			}
//		}
//	}
//
//	// CHUẨN BỊ DỮ LIỆU UPDATE COMMENT CHÍNH
//	updateData := bson.M{"$set": setData}
//
//	// LƯU COMMENT CHÍNH
//	err = commentEntry.Update(filterComment, updateData)
//	// XỬ LÝ KẾT QUẢ TRẢ VỀ
//	switch {
//	case err == nil:
//		c.JSON(http.StatusOK, gin.H{
//			"status":  http.StatusOK,
//			"message": "Cập nhật bình luận thành công.",
//		})
//	case errors.Is(err, mongo.ErrNoDocuments):
//		c.JSON(http.StatusNotFound, gin.H{
//			"status":  http.StatusNotFound,
//			"message": "Không tìm thấy bình luận để cập nhật (có thể đã bị xóa).",
//			"error":   err.Error(),
//		})
//	default:
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"status":  http.StatusInternalServerError,
//			"message": "Lỗi do hệ thống khi cập nhật bình luận!",
//			"error":   err.Error(),
//		})
//	}
//}
//
//func SoftDeleteComment(c *gin.Context) {
//	var (
//		commentEntry = &collections.Comment{}
//	)
//	// Lấy ID bình luận từ URL
//	commentIDParam := c.Param("id")
//	commentID, err := primitive.ObjectIDFromHex(commentIDParam)
//	if err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{
//			"status":  http.StatusBadRequest,
//			"message": "Comment ID không hợp lệ",
//		})
//		return
//	}
//
//	// LẤY ID NGƯỜI XÓA
//	deleterObjectId, ok := utils.GetAccountID(c)
//	if !ok {
//		return
//	}
//
//	// 3. THỰC HIỆN XÓA MỀM
//	now := time.Now()
//	filterDelete := bson.M{
//		"$or": []bson.M{
//			{"_id": commentID},
//			{"ancestor_ids": commentID},
//		},
//		"deleted_at": bson.M{"$exists": false}, // Chỉ xóa những cái chưa bị xóa
//	}
//	updateDelete := bson.M{
//		"$set": bson.M{
//			"deleted_at": now,
//			"deleted_by": deleterObjectId,
//		},
//	}
//
//	// Thực thi lệnh xóa mềm hàng loạt
//	res, err := commentEntry.UpdateMany(nil, filterDelete, updateDelete)
//	switch {
//	case err == nil:
//		c.JSON(http.StatusOK, gin.H{
//			"status":  http.StatusOK,
//			"message": "Xóa mềm bình luận và các trả lời thành công.",
//		})
//	case res == 0:
//		c.JSON(http.StatusNotFound, gin.H{
//			"status":  http.StatusNotFound,
//			"message": "Không tìm thấy comment nào để xóa",
//		})
//	default:
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"status":  http.StatusInternalServerError,
//			"message": "Lỗi hệ thống khi xóa bình luận!",
//			"error":   err.Error(),
//		})
//	}
//}
//
//func RestoreComment(c *gin.Context) {
//	var (
//		commentEntry = &collections.Comment{}
//	)
//	// Lấy ID bình luận từ URL
//	commentIDParam := c.Param("id")
//	commentID, err := primitive.ObjectIDFromHex(commentIDParam)
//	if err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{
//			"status":  http.StatusBadRequest,
//			"message": "Comment ID không hợp lệ",
//		})
//		return
//	}
//
//	// THỰC HIỆN KHÔI PHỤC (Restore)
//	filterRestore := bson.M{
//		"$or": []bson.M{
//			{"_id": commentID},
//			{"ancestor_ids": commentID},
//		},
//		"deleted_at": bson.M{"$exists": true},
//	}
//	updateRestore := bson.M{
//		"$unset": bson.M{
//			"deleted_at": "",
//			"deleted_by": "",
//		},
//	}
//
//	// Thực thi lệnh khôi phục hàng loạt
//	res, err := commentEntry.UpdateMany(nil, filterRestore, updateRestore)
//
//	switch {
//	case err == nil:
//		c.JSON(http.StatusOK, gin.H{
//			"status":  http.StatusOK,
//			"message": "Khôi phục bình luận và các trả lời thành công.",
//		})
//	case res == 0:
//		c.JSON(http.StatusNotFound, gin.H{
//			"status":  http.StatusNotFound,
//			"message": "Không tìm thấy bình luận nào đã bị xóa để khôi phục.",
//		})
//	default:
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"status":  http.StatusInternalServerError,
//			"message": "Lỗi hệ thống khi khôi phục bình luận!",
//			"error":   err.Error(),
//		})
//	}
//}
//
//func GetCommentReplies(c *gin.Context) {
//	var (
//		commentEntry       = &collections.Comment{}
//		limit        int64 = 10
//	)
//
//	parentIDParam := c.Param("id")
//	parentID, err := primitive.ObjectIDFromHex(parentIDParam)
//	if err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{
//			"status":  http.StatusBadRequest,
//			"message": "Parent Comment ID không hợp lệ",
//		})
//		return
//	}
//
//	lastIDParam := c.Query("last_id")
//	var lastID primitive.ObjectID
//
//	if lastIDParam != "" {
//		lastID, err = primitive.ObjectIDFromHex(lastIDParam)
//		if err != nil {
//			c.JSON(http.StatusBadRequest, gin.H{
//				"status":  http.StatusBadRequest,
//				"message": "Last ID không hợp lệ",
//			})
//			return
//		}
//	}
//
//	filter := bson.M{
//		"parent_id":  parentID,
//		"deleted_at": bson.M{"$exists": false},
//	}
//
//	if !lastID.IsZero() {
//		filter["_id"] = bson.M{"$gt": lastID}
//	}
//
//	findOptions := options.Find()
//	findOptions.SetSort(bson.D{{"_id", 1}})
//	findOptions.SetLimit(limit + 1)
//
//	// Thực thi truy vấn
//	comments, err := commentEntry.Find(filter, findOptions)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"status":  http.StatusInternalServerError,
//			"message": "Lỗi hệ thống khi lấy replies",
//			"error":   err.Error(),
//		})
//		return
//	}
//
//	//Xử lý preload
//	err = commentEntry.Preload(comments, "AccountFind")
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{
//			"status":  http.StatusInternalServerError,
//			"message": "Lỗi do hệ thống!",
//			"error":   err.Error(),
//		})
//		return
//	}
//
//	commentsRes := make([]bson.M, 0)
//	for _, cmt := range comments {
//		commentsRes = append(commentsRes, cmt.ParseEntry())
//	}
//
//	// Xử lý kết quả cho hasMore
//	hasMore := false
//	if len(comments) > int(limit) {
//		hasMore = true
//		comments = comments[:limit]
//	}
//
//	// Lấy ID của item cuối cùng để làm "last_id" cho lần request tiếp theo
//	var nextLastID string
//	if len(comments) > 0 {
//		nextLastID = comments[len(comments)-1].ID.Hex()
//	}
//	fmt.Println("oke nha mày")
//	// Trả về kết quả
//	c.JSON(http.StatusOK, gin.H{
//		"status":  http.StatusOK,
//		"message": "Lấy replies thành công.",
//		"data":    commentsRes,
//		"pagination": gin.H{
//			"next_last_id": nextLastID,
//			"has_more":     hasMore,
//			"total_loaded": len(comments),
//		},
//	})
//}
//
//func createMediasWithRetry(mediaEntry *collections.Media, medias []collections.Media) error {
//	var mediaErr error
//	const (
//		maxRetries  = 3
//		baseBackoff = 100 * time.Millisecond
//	)
//	for i := 0; i < maxRetries; i++ {
//		opts := options.InsertMany().SetOrdered(false)
//		mediaErr = mediaEntry.CreateMany(nil, medias, opts)
//		if mediaErr == nil {
//			return nil
//		}
//		if !mongo.IsTimeout(mediaErr) && !mongo.IsNetworkError(mediaErr) {
//			return mediaErr
//		}
//		log.Printf("createMediasWithRetry: lỗi tạm thời (lần %d): %v.", i+1, mediaErr)
//		if i == maxRetries-1 {
//			break
//		}
//		time.Sleep(baseBackoff * time.Duration(1<<i))
//	}
//	return mediaErr
//}
//
//func deactivateMediasWithRetry(mediaEntry *collections.Media, urls []string) error {
//	var mediaErr error
//	const (
//		maxRetries  = 3
//		baseBackoff = 100 * time.Millisecond
//	)
//
//	filter := bson.M{
//		"url":             bson.M{"$in": urls},
//		"collection_name": "comments",
//	}
//	update := bson.M{"$set": bson.M{"status": "inactive"}}
//
//	for i := 0; i < maxRetries; i++ {
//		mediaErr = mediaEntry.UpdateMany(nil, filter, update)
//		if mediaErr == nil {
//			return nil
//		}
//		if !mongo.IsTimeout(mediaErr) && !mongo.IsNetworkError(mediaErr) {
//			return mediaErr
//		}
//		log.Printf("deactivateMediasWithRetry: lỗi tạm thời (lần %d): %v.", i+1, mediaErr)
//		if i == maxRetries-1 {
//			break
//		}
//		time.Sleep(baseBackoff * time.Duration(1<<i))
//	}
//	return mediaErr
//}
