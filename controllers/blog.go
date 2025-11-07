package controllers

import (
	"EventHunting/collections"
	"EventHunting/consts"
	"EventHunting/dto"
	"EventHunting/utils"
	"errors"
	"log"
	"math"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// import (
//
//	"EventHunting/collections"
//	"EventHunting/dto"
//	"EventHunting/utils"
//	"context"
//	"errors"
//	"log"
//	"math"
//	"net/http"
//	"slices"
//	"strconv"
//	"time"
//
//	"github.com/cloudinary/cloudinary-go/v2"
//	"github.com/gin-gonic/gin"
//	"go.mongodb.org/mongo-driver/bson"
//	"go.mongodb.org/mongo-driver/bson/primitive"
//	"go.mongodb.org/mongo-driver/mongo"
//	"go.mongodb.org/mongo-driver/mongo/options"
//
// )
func CreateBlog(c *gin.Context) {
	var req dto.CreateBlogRequest

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

	// Map DTO
	newBlog := collections.Blog{
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

	if req.ThumbnailLink != nil {
		newBlog.ThumbnailLink = *req.ThumbnailLink
	}

	if req.ThumbnailPublicId != nil {
		newBlog.ThumbnailPublicId = *req.ThumbnailPublicId
	}

	if req.Medias != nil && len(*req.Medias) > 0 {
		newMedias := []collections.Media{}
		blogMedias := []struct {
			Type   consts.MediaFormat `bson:"type" json:"type"`
			Url    string             `bson:"url" json:"url"`
			Status consts.MediaStatus `bson:"status" json:"status"`
		}{}

		for _, reqMedia := range *req.Medias {
			blogMedias = append(blogMedias, struct {
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
					CollectionName: "blogs",
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

				log.Printf("CreateBlog (Media): lỗi tạm thời (lần %d): %v. Đang thử lại...", i+1, mediaErr)

				//Đỡ phải chạy sleep
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
		newBlog.Medias = blogMedias
	}

	// Gọi Collection để lưu vào DB
	err := newBlog.Create()
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, dto.ApiResponse{
			Status:  http.StatusCreated,
			Message: "Blog đã được tạo.",
			Data:    newBlog,
		})
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi do hệ thống!",
			Error:   err.Error(),
		})
	}
}

//	func UpdateBlog(c *gin.Context, db *mongo.Database, cld *cloudinary.Cloudinary) {
//		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
//		defer cancel()
//
//		// Khởi tạo collection
//		blogCollection := &collections.Blog{}
//
//		// LẤY VÀ KIỂM TRA BLOG ID
//		blodId := c.Param("id")
//		blodObjectId, err := primitive.ObjectIDFromHex(blodId)
//		if err != nil {
//			c.JSON(http.StatusBadRequest, gin.H{
//				"status":  http.StatusBadRequest,
//				"message": "ID của blog không hợp lệ",
//			})
//			return
//		}
//
//		// BIND VÀ VALIDATE REQUEST BODY
//		var req dto.UpdateBlogRequest
//		if err := c.ShouldBindJSON(&req); err != nil {
//			c.JSON(http.StatusBadRequest, gin.H{
//				"status":  http.StatusBadRequest,
//				"message": err.Error(),
//			})
//			return
//		}
//
//		// Giả sử bạn có hàm validate này
//		if err := utils.ValidateUpdateBlogRequest(req); len(err) > 0 {
//			c.JSON(http.StatusBadRequest, gin.H{
//				"status":  http.StatusBadRequest,
//				"message": err,
//			})
//			return
//		}
//
//		// LẤY THÔNG TIN USER TỪ CONTEXT
//		userIdInterface, exists := c.Get("account_id")
//		if !exists {
//			c.JSON(http.StatusUnauthorized, gin.H{
//				"status":  http.StatusUnauthorized,
//				"message": "Account chưa được xác thực (không tìm thấy account_id)",
//			})
//			return
//		}
//		userIdStr, ok := userIdInterface.(string)
//		if !ok {
//			c.JSON(http.StatusInternalServerError, gin.H{
//				"status":  http.StatusInternalServerError,
//				"message": "Lỗi định dạng dữ liệu account_id trong context",
//			})
//			return
//		}
//		updatedByID, err := primitive.ObjectIDFromHex(userIdStr)
//		if err != nil {
//			c.JSON(http.StatusUnauthorized, gin.H{
//				"status":  http.StatusUnauthorized,
//				"message": "Lỗi format id của user",
//			})
//			return
//		}
//		rolesInterface, exists := c.Get("roles")
//		if !exists {
//			c.JSON(http.StatusUnauthorized, gin.H{
//				"status":  http.StatusUnauthorized,
//				"message": "Account chưa được xác thực (không tìm thấy roles)",
//			})
//			return
//		}
//		roles, ok := rolesInterface.([]string)
//		if !ok {
//			c.JSON(http.StatusInternalServerError, gin.H{
//				"status":  http.StatusInternalServerError,
//				"message": "Lỗi định dạng dữ liệu roles trong context",
//			})
//			return
//		}
//
//		// LẤY BLOG HIỆN TẠI VÀ KIỂM TRA QUYỀN
//		existedBlog, checkExisted := blogCollection.FindById(db, ctx, blodObjectId)
//		if checkExisted != nil {
//			c.JSON(http.StatusBadRequest, gin.H{
//				"status":  http.StatusBadRequest,
//				"message": checkExisted.Error(),
//			})
//			return
//		}
//
//		// Kiểm tra quyền
//		if updatedByID != existedBlog.CreatedBy && !slices.Contains(roles, "Admin") {
//			c.JSON(http.StatusForbidden, gin.H{
//				"status":  http.StatusForbidden,
//				"message": "Bạn không có quyền chỉnh sửa blog này",
//			})
//			return
//		}
//
//		filter := bson.M{
//			"_id": blodObjectId,
//		}
//
//		updateDoc := bson.M{}
//		var imagesToDelete []string
//
//		// Thêm các trường cơ bản
//		if req.Title != nil {
//			updateDoc["title"] = *req.Title
//		}
//		if req.Content != nil {
//			updateDoc["content"] = *req.Content
//		}
//		if req.ThumbnailLink != nil {
//			updateDoc["thumbnail_link"] = *req.ThumbnailLink
//		}
//		if req.TagIds != nil {
//			updateDoc["tag_ids"] = *req.TagIds
//		}
//
//		// Logic Diff cho Thumbnail
//		if req.ThumbnailPublicId != nil {
//			updateDoc["thumbnail_public_id"] = *req.ThumbnailPublicId
//			if *req.ThumbnailPublicId != existedBlog.ThumbnailPublicId && existedBlog.ThumbnailPublicId != "" {
//				imagesToDelete = append(imagesToDelete, existedBlog.ThumbnailPublicId)
//			}
//		}
//
//		// Logic Diff cho ảnh nội dung
//		if req.PublicImgIds != nil {
//			updateDoc["public_img_ids"] = *req.PublicImgIds
//			newImageSet := make(map[string]bool)
//			for _, newId := range *req.PublicImgIds {
//				newImageSet[newId] = true
//			}
//			if existedBlog.PublicImgIds != nil {
//				for _, oldId := range existedBlog.PublicImgIds {
//					if _, found := newImageSet[oldId]; !found {
//						imagesToDelete = append(imagesToDelete, oldId)
//					}
//				}
//			}
//		}
//
//		// Thêm các trường audit
//		updateDoc["updated_at"] = time.Now()
//		updateDoc["updated_by"] = updatedByID
//
//		update := bson.M{
//			"$set": updateDoc,
//		}
//
//		// CẬP NHẬT DATABASE
//		_, err = blogCollection.Update(db, ctx, filter, update)
//		if err != nil {
//			if errors.Is(err, mongo.ErrNoDocuments) {
//				c.JSON(http.StatusNotFound, gin.H{
//					"status":  http.StatusNotFound,
//					"message": "Không tìm thấy blog để update (có thể đã bị xóa)",
//				})
//				return
//			}
//			c.JSON(http.StatusInternalServerError, gin.H{
//				"status":  http.StatusInternalServerError,
//				"message": "Lỗi khi cập nhật blog: " + err.Error(),
//			})
//			return
//		}
//
//		// DỌN DẸP ẢNH
//		if len(imagesToDelete) > 0 {
//			log.Printf("Bắt đầu xóa %d file cũ trên Cloudinary cho blog: %s", len(imagesToDelete), blodObjectId.Hex())
//			for _, publicId := range imagesToDelete {
//				err = utils.DeleteFileCloudinary(cld, publicId)
//				if err != nil {
//					log.Printf("LỖI dọn dẹp file: Không thể xóa file %s trên Cloudinary: %v", publicId, err)
//				}
//			}
//		}
//
//		// TRẢ VỀ THÀNH CÔNG
//		c.JSON(http.StatusOK, gin.H{
//			"status":  http.StatusOK,
//			"message": "Update blog thành công",
//		})
//	}

func SoftDeleteBlog(c *gin.Context) {
	// 1. Lấy BlogID từ URL param
	var (
		err          error
		blogEntry    = &collections.Blog{}
		commentEntry = &collections.Comment{}
		mediaEntry   = &collections.Media{}
	)
	blogIDStr := c.Param("id")
	blogID, err := primitive.ObjectIDFromHex(blogIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Blog ID không hợp lệ",
			"error":   err.Error(),
		})
		return
	}

	// Lấy UserID (người thực hiện xóa)
	deletorID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}
	// Lấy role từ người delete
	roles, existed := c.Get("roles")
	if !existed {
		c.JSON(http.StatusUnauthorized, dto.ApiResponse{
			Status:  http.StatusUnauthorized,
			Message: "Không tìm thấy roles trong context!",
		})
		return
	}
	rolesSlice := roles.([]string)
	//Kiểm tra xem blog đó có tồn tại không
	var (
		blogFilter = bson.M{
			"_id": blogID,
			"deleted_at": bson.M{
				"$exists": false,
			},
		}
	)
	err = blogEntry.First(blogFilter)
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
	//Kiểm tra deleter có phải là chủ blog hay là admin
	if blogEntry.CreatedBy != deletorID && !slices.Contains(rolesSlice, "Admin") {
		c.JSON(http.StatusForbidden, dto.ApiResponse{
			Status:  http.StatusForbidden,
			Message: "Bạn không có quyền truy cập vào tài nguyên này!",
		})
		return
	}
	//Lấy danh sách các comment cần soft delete
	var (
		commentFilter = bson.M{
			"blog_id": blogID,
			"deleted_at": bson.M{
				"$exists": false,
			},
		}
	)
	comments, err := commentEntry.Find(commentFilter, options.Find().SetProjection(bson.M{"_id": 1}))
	commentIDs := make([]primitive.ObjectID, 0)
	for _, comment := range comments {
		commentIDs = append(commentIDs, comment.ID)
	}

	//Soft delete các comment của blog
	if len(commentIDs) > 0 {
		updateCommentFilter := bson.M{
			"_id": bson.M{
				"$in": commentIDs,
			},
			"deleted_at": bson.M{
				"$exists": false,
			},
		}
		updateComment := bson.M{
			"$set": bson.M{
				"deleted_at": time.Now(),
				"deleted_by": deletorID,
				"updated_by": deletorID,
				"updated_at": time.Now(),
			},
		}
		_, err = commentEntry.UpdateMany(updateCommentFilter, updateComment)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ApiResponse{
				Status:  http.StatusInternalServerError,
				Message: "Lỗi do hệ thống!",
				Error:   err.Error(),
			})
			return
		}
	}

	//Soft delete các media của blog và các media của comment
	documentIDs := append(commentIDs, blogID)
	if len(documentIDs) > 0 {
		updateMediaFilter := bson.M{
			"document_id": bson.M{
				"$in": documentIDs,
			},
			"deleted_at": bson.M{
				"$exists": false,
			},
		}
		updateMedias := bson.M{
			"$set": bson.M{
				"deleted_at": time.Now(),
				"status":     "inactive",
			},
		}
		err = mediaEntry.UpdateMany(updateMediaFilter, updateMedias)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ApiResponse{
				Status:  http.StatusInternalServerError,
				Message: "Lỗi do hệ thống!",
				Error:   err.Error(),
			})
			return
		}
	}

	//Soft delete blog
	var (
		blogUpdate = bson.M{
			"$set": bson.M{
				"deleted_at": time.Now(),
				"deleted_by": deletorID,
				"updated_by": deletorID,
				"updated_at": time.Now(),
			},
		}
	)
	err = blogEntry.Update(blogFilter, blogUpdate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi từ hệ thống!",
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusNoContent, dto.ApiResponse{
		Status:  http.StatusNoContent,
		Message: "Xóa mềm thành công.",
	})
}

func RestoreBlog(c *gin.Context) {
	// Lấy BlogID từ URL param
	var (
		err          error
		blogEntry    = &collections.Blog{}
		commentEntry = &collections.Comment{}
		mediaEntry   = &collections.Media{}
	)
	blogIDStr := c.Param("id")
	blogID, err := primitive.ObjectIDFromHex(blogIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "Blog ID không hợp lệ",
			"error":   err.Error(),
		})
		return
	}

	// Lấy UserID
	restorerID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}
	// Lấy role
	roles, existed := c.Get("roles")
	if !existed {
		c.JSON(http.StatusUnauthorized, dto.ApiResponse{
			Status:  http.StatusUnauthorized,
			Message: "Không tìm thấy roles trong context!",
		})
		return
	}
	rolesSlice := roles.([]string)

	//Kiểm tra xem blog đó có tồn tại VÀ ĐÃ BỊ XÓA không
	var (
		blogFilter = bson.M{
			"_id": blogID,
			"deleted_at": bson.M{
				"$exists": true,
			},
		}
	)
	err = blogEntry.First(blogFilter)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, dto.ApiResponse{
				Status:  http.StatusNotFound,
				Message: "Không tìm thấy blog hoặc blog này chưa bị xóa!",
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

	//Kiểm tra quyền
	if blogEntry.CreatedBy != restorerID && !slices.Contains(rolesSlice, "Admin") {
		c.JSON(http.StatusForbidden, dto.ApiResponse{
			Status:  http.StatusForbidden,
			Message: "Bạn không có quyền truy cập vào tài nguyên này!",
		})
		return
	}

	//Lấy danh sách các comment cần khôi phục
	var (
		commentFilter = bson.M{
			"blog_id": blogID,
			"deleted_at": bson.M{
				"$exists": true,
			},
		}
	)
	comments, err := commentEntry.Find(commentFilter, options.Find().SetProjection(bson.M{"_id": 1}))
	commentIDs := make([]primitive.ObjectID, 0)
	for _, comment := range comments {
		commentIDs = append(commentIDs, comment.ID)
	}

	// KHÔI PHỤC CÁC COMMENT CỦA BLOG
	if len(commentIDs) > 0 {
		updateCommentFilter := bson.M{
			"_id": bson.M{
				"$in": commentIDs,
			},
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
		_, err = commentEntry.UpdateMany(updateCommentFilter, updateComment)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ApiResponse{
				Status:  http.StatusInternalServerError,
				Message: "Lỗi do hệ thống khi khôi phục comments!",
				Error:   err.Error(),
			})
			return
		}
	}

	// KHÔI PHỤC CÁC MEDIA
	documentIDs := append(commentIDs, blogID)
	if len(documentIDs) > 0 {
		updateMediaFilter := bson.M{
			"document_id": bson.M{
				"$in": documentIDs,
			},
			"deleted_at": bson.M{
				"$exists": true,
			},
		}
		updateMedias := bson.M{
			"$unset": bson.M{
				"deleted_at": "",
			},
			"$set": bson.M{
				"status": "active",
			},
		}
		err = mediaEntry.UpdateMany(updateMediaFilter, updateMedias)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ApiResponse{
				Status:  http.StatusInternalServerError,
				Message: "Lỗi do hệ thống khi khôi phục media!",
				Error:   err.Error(),
			})
			return
		}
	}

	// Khôi phục blog
	var (
		blogUpdate = bson.M{
			"$unset": bson.M{
				"deleted_at": "",
				"deleted_by": "",
			},
			"$set": bson.M{
				"updated_by": restorerID,
				"updated_at": time.Now(),
			},
		}
	)

	err = blogEntry.Update(blogFilter, blogUpdate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi từ hệ thống khi khôi phục blog!",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, dto.ApiResponse{
		Status:  http.StatusOK,
		Message: "Khôi phục thành công.",
	})
}

func GetListBlogs(c *gin.Context) {
	var (
		blogEntry = &collections.Blog{}
	)
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

	filter := bson.M{
		"deleted_at": bson.M{
			"$exists": false,
		},
	}

	//Filter
	dynamicFilter := utils.BuildBlogSearchFilter(queryMap)
	for key, value := range dynamicFilter {
		filter[key] = value
	}

	findOptions := options.Find()
	findOptions.SetLimit(limit)
	findOptions.SetSkip(skip)

	//Sort
	sorts := utils.BuildSortFilter(queryMap)
	findOptions.SetSort(sorts)

	//Tính toán trang
	totalDocs, _ := blogEntry.CountDocuments(filter)
	totalPages := int64(math.Ceil(float64(totalDocs) / float64(limit)))

	//Lấy kết quả tìm kiếm
	results, err := blogEntry.Find(filter, findOptions)
	switch {
	case err == nil:
		err = blogEntry.Preload(&results, "AccountFind", "TagFind", "CommentCountFind")
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
		res := []bson.M{}
		for _, blog := range results {
			res = append(res, utils.PrettyJSON(blog.ParseEntry()))
		}
		c.JSON(http.StatusOK, dto.ApiResponse{
			Status:  http.StatusOK,
			Message: "Thành công.",
			Data:    res,
			Pagination: &dto.Pagination{
				PageNo:     int(page),
				PageSize:   int(limit),
				PageCount:  int(totalPages),
				TotalItems: int(totalDocs),
			},
		})
	case len(results) == 0:
		c.JSON(http.StatusNotFound, dto.ApiResponse{
			Status:  http.StatusNotFound,
			Message: "Không tìm thấy kết quả!",
		})
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi do hệ thống!",
			Error:   err.Error(),
		})
	}
}

func GetBlog(c *gin.Context) {
	var (
		id       = c.Param("id")
		idHex, _ = primitive.ObjectIDFromHex(id)
		entry    collections.Blog
		filter   = bson.M{
			"_id":        idHex,
			"deleted_at": nil,
		}
		err error
	)

	err = entry.First(filter)
	switch err {
	case nil:
		err = entry.Preload(nil, "AccountFirst", "TagFirst", "CommentCountFirst")
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
		PaginationLoadMore: &dto.PaginationLoadMore{
			HasMore:     hasMore,
			NextLastId:  nextLastID,
			TotalLoaded: len(commentsRes),
		},
	})
}
