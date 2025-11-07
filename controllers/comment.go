package controllers

import (
	"EventHunting/collections"
	"EventHunting/consts"
	"EventHunting/dto"
	"EventHunting/utils"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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
//	if req.BlogID.IsZero() && req.Category == consts.COMMENT_TYPE_BLOG {
//		c.JSON(http.StatusBadRequest, gin.H{
//			"status":  http.StatusBadRequest,
//			"message": "BlogID là bắt buộc",
//		})
//		return
//	}
//
//	// KIỂM TRA BÀI VIẾT (BLOG)
//	if !req.BlogID.IsZero() {
//		filterBlog := bson.M{
//			"_id":        req.BlogID,
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
//			mediaErr = mediaEntry.CreateMany(newMedias, opts)
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
//		BlogID:      req.BlogID,
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
//			_, err = commentEntry.UpdateMany(filterDescendants, updateDescendants)
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
//	res, err := commentEntry.UpdateMany(filterDelete, updateDelete)
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
//	res, err := commentEntry.UpdateMany(filterRestore, updateRestore)
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
//		mediaErr = mediaEntry.CreateMany(medias, opts)
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
//		mediaErr = mediaEntry.UpdateMany(filter, update)
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

func CreateComment(c *gin.Context) {
	var (
		blogEntry     = &collections.Blog{}
		commentEntry  = &collections.Comment{}
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
	if invalidErrors = dto.ValidateCommentCreate(req); len(invalidErrors) > 0 {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Lỗi nhập sai dữ liệu!",
			Error:   invalidErrors,
		})
		return
	}

	// KIỂM TRA BÀI VIẾT (BLOG)
	if !req.BlogID.IsZero() {
		filterBlog := bson.M{
			"_id":        req.BlogID,
			"deleted_at": bson.M{"$exists": false},
		}
		checkBlogExisted := blogEntry.First(filterBlog)
		if checkBlogExisted != nil {
			if errors.Is(checkBlogExisted, mongo.ErrNoDocuments) {
				c.JSON(http.StatusNotFound, dto.ApiResponse{
					Status:  http.StatusNotFound,
					Message: "Bài viết không tồn tại hoặc đã bị xóa!",
					Error:   checkBlogExisted.Error(),
				})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ApiResponse{
				Status:  http.StatusInternalServerError,
				Message: "Lỗi do hệ thống khi kiểm tra bài viết!",
				Error:   checkBlogExisted.Error(),
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
		checkParentExisted := commentEntry.First(filter)
		if checkParentExisted != nil {
			if errors.Is(checkParentExisted, mongo.ErrNoDocuments) {
				c.JSON(http.StatusNotFound, dto.ApiResponse{
					Status:  http.StatusNotFound,
					Message: "Comment cha không tồn tại hoặc đã bị xóa!",
					Error:   checkParentExisted.Error(),
				})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ApiResponse{
				Status:  http.StatusInternalServerError,
				Message: "Lỗi do hệ thống!",
				Error:   checkParentExisted.Error(),
			})
			return
		}
	}

	// CHUẨN BỊ DỮ LIỆU MEDIAS
	newCommentID := primitive.NewObjectID()
	newMedias := []collections.Media{}
	commentMedias := []struct {
		Type   consts.MediaFormat `bson:"type" json:"type"`
		Url    string             `bson:"url" json:"url"`
		Status consts.MediaStatus `bson:"status" json:"status"`
	}{}

	for _, reqMedia := range req.Medias {
		commentMedias = append(commentMedias, struct {
			Type   consts.MediaFormat `bson:"type" json:"type"`
			Url    string             `bson:"url" json:"url"`
			Status consts.MediaStatus `bson:"status" json:"status"`
		}{
			Type:   reqMedia.Type,
			Url:    reqMedia.Url,
			Status: reqMedia.Status,
		})
		if reqMedia.Status == consts.MediaStatusSuccess {
			newMedia := collections.Media{
				ID:             primitive.NewObjectID(),
				Url:            reqMedia.Url,
				PublicUrlId:    reqMedia.PublicUrlId,
				Type:           reqMedia.Type,
				Status:         "active",
				CollectionName: "comments",
				DocumentId:     newCommentID,
			}
			newMedias = append(newMedias, newMedia)
		}
	}

	// LƯU MEDIAS
	if len(newMedias) > 0 {
		var (
			mediaEntry = &collections.Media{}
			mediaErr   error
		)

		const (
			maxRetries  = 3
			baseBackoff = 100 * time.Millisecond
		)

		// Vòng lặp Retry
		for i := 0; i < maxRetries; i++ {
			opts := options.InsertMany().SetOrdered(false)
			mediaErr = mediaEntry.CreateMany(newMedias, opts)

			if mediaErr == nil {
				break
			}

			if !mongo.IsTimeout(mediaErr) && !mongo.IsNetworkError(mediaErr) {
				break
			}

			log.Printf("CreateComment (Media): lỗi tạm thời (lần %d): %v. Đang thử lại...", i+1, mediaErr)

			if i == maxRetries-1 {
				break
			}

			backoff := baseBackoff * time.Duration(1<<i)
			time.Sleep(backoff)
		}

		// Kiểm tra lỗi cuối cùng sau khi thoát vòng lặp
		if mediaErr != nil {
			c.JSON(http.StatusInternalServerError, dto.ApiResponse{
				Status:  http.StatusInternalServerError,
				Message: "Lỗi do hệ thống khi lưu media sau nhiều lần thử!",
				Error:   mediaErr.Error(),
			})
			return
		}
	}

	// CHUẨN BỊ DỮ LIỆU COMMENT CHÍNH
	now := time.Now()
	newComment := collections.Comment{
		ID:          newCommentID,
		ParentID:    req.ParentID,
		Content:     req.Content,
		ContentHTML: req.ContentHTML,
		BlogID:      req.BlogID,
		Medias:      commentMedias,
		IsEdit:      false,
		CreatedAt:   now,
		CreatedBy:   creatorObjectId,
		UpdatedAt:   now,
		UpdatedBy:   creatorObjectId,
		Category:    req.Category,

		ReplyCount: 0,
	}

	// LƯU COMMENT CHÍNH
	err := newComment.Create()

	// XỬ LÝ KẾT QUẢ TRẢ VỀ
	switch {
	case err == nil:
		//Cập nhật reply count cho comment cha
		if !newComment.ParentID.IsZero() {
			go func(parentID primitive.ObjectID) {
				commentParentEntry := &collections.Comment{}
				filterParent := bson.M{"_id": parentID}
				updateParent := bson.M{"$inc": bson.M{"reply_count": 1}}
				for i := 0; i < maxRetry; i++ {
					retryErr := commentParentEntry.Update(filterParent, updateParent)
					if retryErr == nil {
						break
					}
				}
			}(newComment.ParentID)
		}

		comments := collections.Comments{}
		err = newComment.Preload(comments, "AccountFirst")

		c.JSON(http.StatusCreated, dto.ApiResponse{
			Status:  http.StatusCreated,
			Message: "tạo bình luận thành công.",
			Data:    utils.PrettyJSON(newComment.ParseEntry()),
		})
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi do hệ thống!",
			Error:   err.Error(),
		})
	}
}

func UpdateComment(c *gin.Context) {
	var (
		commentEntry *collections.Comment = &collections.Comment{}
		mediaEntry   *collections.Media   = &collections.Media{}
	)

	// LẤY ID NGƯỜI CẬP NHẬT
	updatorObjectId, ok := utils.GetAccountID(c)
	if !ok {
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
	if invalidErrors := dto.ValidateCommentUpdate(req); len(invalidErrors) > 0 {
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

	checkExisted := commentEntry.First(filterComment)
	if checkExisted != nil {
		if errors.Is(checkExisted, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, dto.ApiResponse{
				Status:  http.StatusNotFound,
				Message: "Bình luận không tồn tại hoặc đã bị xóa!",
				Error:   checkExisted.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi do hệ thống khi kiểm tra bình luận!",
			Error:   checkExisted.Error(),
		})
		return
	}

	// KIỂM TRA QUYỀN SỞ HỮU
	if commentEntry.CreatedBy != updatorObjectId {
		c.JSON(http.StatusForbidden, dto.ApiResponse{
			Status:  http.StatusForbidden,
			Message: "Bạn không có quyền chỉnh sửa bình luận này!",
		})
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

	if req.Medias != nil {
		oldMediaUrls := make(map[string]bool)
		for _, m := range commentEntry.Medias {
			oldMediaUrls[m.Url] = true
		}

		newMediaUrls := make(map[string]bool)
		newMediasToInsert := []collections.Media{}
		finalCommentMedias := []struct {
			Type   consts.MediaFormat `bson:"type" json:"type"`
			Url    string             `bson:"url" json:"url"`
			Status consts.MediaStatus `bson:"status" json:"status"`
		}{}
		urlsToDeactivate := []string{}

		for _, reqMedia := range *req.Medias {
			newMediaUrls[reqMedia.Url] = true

			finalCommentMedias = append(finalCommentMedias, struct {
				Type   consts.MediaFormat `bson:"type" json:"type"`
				Url    string             `bson:"url" json:"url"`
				Status consts.MediaStatus `bson:"status" json:"status"`
			}{
				Type:   reqMedia.Type,
				Url:    reqMedia.Url,
				Status: reqMedia.Status,
			})

			// Nếu URL này là mới VÀ thành công
			if !oldMediaUrls[reqMedia.Url] && reqMedia.Status == consts.MediaStatusSuccess {
				newMediasToInsert = append(newMediasToInsert, collections.Media{
					ID:             primitive.NewObjectID(),
					Url:            reqMedia.Url,
					PublicUrlId:    reqMedia.PublicUrlId,
					Type:           reqMedia.Type,
					Status:         "active",
					CollectionName: "comments",
					DocumentId:     commentObjectId,
				})
			}
		}

		// Lặp qua media CŨ để tìm cái bị xóa
		for oldUrl := range oldMediaUrls {
			if !newMediaUrls[oldUrl] {
				urlsToDeactivate = append(urlsToDeactivate, oldUrl)
			}
		}

		// Cập nhật mảng media trong $set
		setData["medias"] = finalCommentMedias

		// LƯU MEDIA MỚI
		if len(newMediasToInsert) > 0 {
			if err := createMediasWithRetry(mediaEntry, newMediasToInsert); err != nil {
				c.JSON(http.StatusInternalServerError, dto.ApiResponse{
					Status:  http.StatusInternalServerError,
					Message: "Lỗi hệ thống khi lưu media mới!",
					Error:   err.Error(),
				})
				return
			}
		}

		// VÔ HIỆU HÓA MEDIA CŨ
		if len(urlsToDeactivate) > 0 {
			if err := deactivateMediasWithRetry(mediaEntry, urlsToDeactivate); err != nil {
				c.JSON(http.StatusInternalServerError, dto.ApiResponse{
					Status:  http.StatusInternalServerError,
					Message: "Lỗi hệ thống khi vô hiệu hóa media cũ!",
					Error:   err.Error(),
				})
				return
			}
		}
	}

	// CHUẨN BỊ DỮ LIỆU UPDATE COMMENT CHÍNH
	updateData := bson.M{"$set": setData}

	// LƯU COMMENT CHÍNH
	err = commentEntry.Update(filterComment, updateData)
	// XỬ LÝ KẾT QUẢ TRẢ VỀ
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
			Message: "Lỗi do hệ thống khi cập nhật bình luận!",
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

	filterFind := bson.M{
		"_id":        commentID,
		"deleted_at": bson.M{"$exists": false},
	}
	// Lấy comment và gán vào commentEntry
	if err := commentEntry.First(filterFind); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, dto.ApiResponse{
				Status:  http.StatusNotFound,
				Message: "Bình luận không tồn tại hoặc đã bị xóa!",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi hệ thống khi tìm bình luận!",
			Error:   err.Error(),
		})
		return
	}

	if commentEntry.CreatedBy != deleterObjectId {
		c.JSON(http.StatusForbidden, dto.ApiResponse{
			Status:  http.StatusForbidden,
			Message: "Bạn không có quyền xóa bình luận này!",
		})
		return
	}

	parentIDToUpdate := commentEntry.ParentID
	//parentMediaUrls := commentEntry.Medias

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
	res, err := commentEntry.UpdateMany(filterDelete, updateDelete)

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
					retryErr := commentParentEntry.Update(filterParent, updateParent)
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

		// Chạy ngầm TẤT CẢ logic dọn dẹp media
		go func(deletedCommentID primitive.ObjectID) {

			log.Printf("Chạy ngầm dọn dẹp media cho comment %s...", deletedCommentID.Hex())
			allCommentIDsToDelete := []primitive.ObjectID{deletedCommentID} // Bắt đầu với ID cha

			// Tìm và lấy ID từ tất cả comment con
			childCommentEntry := &collections.Comment{}
			filterFindChildren := bson.M{
				"parent_id":  deletedCommentID,
				"deleted_at": bson.M{"$exists": true},
			}
			findOptions := options.Find().SetProjection(bson.M{"_id": 1})

			childComments, err := childCommentEntry.Find(filterFindChildren, findOptions)
			if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
				log.Printf("Lỗi (chạy ngầm) khi tìm con bị xóa: %v", err)
				return
			}

			if len(childComments) > 0 {
				for _, child := range childComments {
					allCommentIDsToDelete = append(allCommentIDsToDelete, child.ID)
				}
			}

			log.Printf("Bắt đầu vô hiệu hóa media cho %d comments (chạy ngầm)...", len(allCommentIDsToDelete))
			mediaEntry := &collections.Media{}

			filter := bson.M{
				"document_id":     bson.M{"$in": allCommentIDsToDelete},
				"collection_name": "comments",
				"deleted_at":      bson.M{"$exists": false},
			}
			update := bson.M{"$set": bson.M{"status": "inactive", "deleted_at": time.Now()}}

			for i := 0; i < maxRetry; i++ {
				err = mediaEntry.UpdateMany(filter, update)
				if err == nil {
					log.Printf("Vô hiệu hóa media (chạy ngầm) thành công.")
					break
				}
				log.Printf("Lỗi khi vô hiệu hóa media (lần %d): %v", i+1, err)
				time.Sleep(100 * time.Millisecond)
			}

		}(commentID)
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

	filterFind := bson.M{
		"_id":        commentID,
		"deleted_at": bson.M{"$exists": true},
	}
	// Gán dữ liệu tìm được vào commentEntry
	if err := commentEntry.First(filterFind); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, dto.ApiResponse{
				Status:  http.StatusNotFound,
				Message: "Bình luận không tồn tại hoặc chưa bị xóa!",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi hệ thống khi tìm bình luận!",
			Error:   err.Error(),
		})
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
		"$unset": bson.M{
			"deleted_at": "",
			"deleted_by": "",
		},
	}

	// Thực thi lệnh khôi phục hàng loạt
	res, err := commentEntry.UpdateMany(filterRestore, updateRestore)

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
				filterParent := bson.M{"_id": parentID}
				updateParent := bson.M{"$inc": bson.M{"reply_count": 1}} // Tăng 1

				for i := 0; i < maxRetry; i++ {
					retryErr := commentParentEntry.Update(filterParent, updateParent)
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

		go func(restoredCommentID primitive.ObjectID) {

			log.Printf("Chạy ngầm kích hoạt media cho comment %s...", restoredCommentID.Hex())

			allCommentIDsToRestore := []primitive.ObjectID{restoredCommentID}

			// Tìm và lấy ID từ tất cả comment con (vừa được khôi phục)
			childCommentEntry := &collections.Comment{}
			filterFindChildren := bson.M{
				"parent_id":  restoredCommentID,
				"deleted_at": bson.M{"$exists": false},
			}
			findOptions := options.Find().SetProjection(bson.M{"_id": 1})

			childComments, err := childCommentEntry.Find(filterFindChildren, findOptions)
			if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
				log.Printf("Lỗi (chạy ngầm) khi tìm con được khôi phục: %v", err)
				return
			}

			if len(childComments) > 0 {
				for _, child := range childComments {
					allCommentIDsToRestore = append(allCommentIDsToRestore, child.ID)
				}
			}

			log.Printf("Bắt đầu khôi phục media cho %d comments (chạy ngầm)...", len(allCommentIDsToRestore))
			mediaEntry := &collections.Media{}

			filter := bson.M{
				"document_id":     bson.M{"$in": allCommentIDsToRestore},
				"collection_name": "comments",
				"deleted_at":      bson.M{"$exists": true},
			}
			update := bson.M{
				"$set": bson.M{
					"status": "active",
				},
				"$unset": bson.M{
					"deleted_at": "",
				}}

			for i := 0; i < maxRetry; i++ {
				err = mediaEntry.UpdateMany(filter, update)
				if err == nil {
					log.Printf("Khôi phục media (chạy ngầm) thành công.")
					break
				}
				log.Printf("Lỗi khi khôi phục media (lần %d): %v", i+1, err)
				time.Sleep(100 * time.Millisecond)
			}

		}(commentID)
	}
}

func GetCommentReplies(c *gin.Context) {
	var (
		commentEntry       = &collections.Comment{}
		limit        int64 = 10
	)

	parentIDParam := c.Param("id")
	parentID, err := primitive.ObjectIDFromHex(parentIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ApiResponse{
			Status:  http.StatusBadRequest,
			Message: "Parent Comment ID không hợp lệ!",
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
		"parent_id":  parentID,
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

	//Xử lý preload
	err = commentEntry.Preload(comments, "AccountFind")
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
		PaginationLoadMore: &dto.PaginationLoadMore{
			HasMore:     hasMore,
			NextLastId:  nextLastID,
			TotalLoaded: len(commentsRes),
		},
	})
}

func createMediasWithRetry(mediaEntry *collections.Media, medias []collections.Media) error {
	var mediaErr error
	const (
		maxRetries  = 3
		baseBackoff = 100 * time.Millisecond
	)
	for i := 0; i < maxRetries; i++ {
		opts := options.InsertMany().SetOrdered(false)
		mediaErr = mediaEntry.CreateMany(medias, opts)
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

func deactivateMediasWithRetry(mediaEntry *collections.Media, urls []string) error {
	var mediaErr error
	const (
		maxRetries  = 3
		baseBackoff = 100 * time.Millisecond
	)

	filter := bson.M{
		"url":             bson.M{"$in": urls},
		"collection_name": "comments",
	}
	update := bson.M{"$set": bson.M{"status": "inactive", "deleted_at": time.Now()}}

	for i := 0; i < maxRetries; i++ {
		mediaErr = mediaEntry.UpdateMany(filter, update)
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
