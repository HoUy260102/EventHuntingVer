package controllers

import (
	"EventHunting/collections"
	"EventHunting/consts"
	"EventHunting/utils"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func UploadMedia(c *gin.Context) {
	var (
		err        error
		cld        = utils.GetCloudinary()
		maxRetries = 3
		mediaErr   error
	)

	coll := c.PostForm("coll_type")
	if coll == "" {
		utils.ResponseError(c, http.StatusBadRequest, "", "Thiếu coll!")
		return
	}

	collConvert, err := strconv.Atoi(coll)
	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "Lỗi chuyển string sang int!", err.Error())
		return
	}

	collMapping, err := utils.GetCollectionMappingByID("consts/collection_name.csv", collConvert)

	if err != nil {
		switch {
		case errors.Is(err, consts.ErrInvalidID) || errors.Is(err, consts.ErrCollectionEmpty):
			utils.ResponseError(c, http.StatusBadRequest, "", err.Error())
			return
		case errors.Is(err, consts.ErrFileNotFound) || errors.Is(err, consts.ErrParseFailed):
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
			return
		default:
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
			return
		}
	}

	// Lấy tất cả file gửi lên
	form, err := c.MultipartForm()
	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "", "Không có form data!")
		return
	}

	files := form.File["file"]
	if len(files) == 0 {
		utils.ResponseError(c, http.StatusBadRequest, "", "Chưa có file upload!")
		return
	}

	if len(files) > 1 {
		utils.ResponseError(c, http.StatusBadRequest, "", "Chưa có file upload!")
		return
	}

	fileHeader := files[0]

	err = utils.ChechValidFile(fileHeader)
	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "", err.Error())
	}

	err = utils.CheckValidMiMe(fileHeader)
	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "", err.Error())
	}

	// Mở stream file
	file, err := fileHeader.Open()
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}
	defer file.Close()

	// Xác định loại file
	format, fileType := utils.DetectFileType(fileHeader)

	// Upload trực tiếp theo chunk
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	//Sinh file path
	mediaID := primitive.NewObjectID()
	ext := filepath.Ext(files[0].Filename)
	baseName := strings.TrimSuffix(files[0].Filename, ext)
	fileName := fmt.Sprintf("%s_%s", baseName, mediaID.Hex())
	params := uploader.UploadParams{
		PublicID: fileName,
		Folder:   fmt.Sprintf("upload/%s", collMapping.Name),
		Format:   format,
	}

	res, err := cld.Upload.Upload(ctx, file, params)
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	newMedia := collections.Media{
		ID:             mediaID,
		Status:         consts.MediaStatusPending,
		CreatedAt:      time.Now(),
		Type:           fileType,
		UrlId:          res.PublicID,
		Url:            res.SecureURL,
		Extension:      "." + res.Format,
		CollectionName: collMapping.Name,
	}

	//Lưu media vào db
	for i := 0; i < maxRetries; i++ {
		mediaErr = newMedia.Create(nil)
		if mediaErr == nil {
			break
		}
		log.Printf("Lưu Media vào DB thất bại (lần %d): %v. Thử lại...", i+1, mediaErr)
		time.Sleep(time.Duration(i+1) * time.Second) // backoff đơn giản
	}

	if mediaErr != nil {
		// Xoá file vừa upload lên Cloudinary nếu lưu DB thất bại
		cld.Upload.Destroy(context.Background(), uploader.DestroyParams{
			PublicID: res.PublicID,
		})

		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống khi lưu dữ liệu!", mediaErr.Error())
		return
	}

	// Trả kết quả
	utils.ResponseSuccess(c, http.StatusOK, "", newMedia, nil)
}
