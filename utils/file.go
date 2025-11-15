package utils

import (
	"EventHunting/consts"
	"EventHunting/dto"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

var validFile = map[string]bool{
	".heic": true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".bmp":  true,
	".webp": true,

	// video
	".mp4": true,
	".mov": true,
	".avi": true,
	".mkv": true,
	".flv": true,
	".wmv": true,
}

var validMiMe = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/bmp":  true,
	"image/webp": true,
	"image/heic": true,

	"video/mp4":        true,
	"video/quicktime":  true,
	"video/x-msvideo":  true,
	"video/x-matroska": true,
	"video/x-flv":      true,
	"video/x-ms-wmv":   true,
}

func ChechValidFile(fileHeader *multipart.FileHeader) error {
	fileName := fileHeader.Filename
	fileExt := filepath.Ext(fileName)
	if _, ok := validFile[fileExt]; !ok {
		return fmt.Errorf("%s không phải là định dạng file hợp lệ!", fileName)
	}
	return nil
}

func CheckValidMiMe(fileHeader *multipart.FileHeader) error {
	f, err := fileHeader.Open()

	if err != nil {
		return fmt.Errorf("Không thể mở được file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)

	mimeType := http.DetectContentType(buf[:n])
	if _, ok := validMiMe[mimeType]; !ok {
		return fmt.Errorf("%s không phải là định dạng file hợp lệ!", mimeType)
	}

	return nil
}

func DetectFileType(fileHeader *multipart.FileHeader) (string, consts.MediaFormat) {
	if fileHeader == nil {
		return "", consts.MEDIA_UNKNOWN
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", consts.MEDIA_UNKNOWN
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", consts.MEDIA_UNKNOWN
	}
	buffer = buffer[:n]

	contentType := http.DetectContentType(buffer)
	contentType = strings.ToLower(contentType)
	switch {
	case strings.HasPrefix(contentType, "image/"):
		format := strings.TrimPrefix(contentType, "image/")
		return format, consts.MEDIA_IMAGE
	case strings.HasPrefix(contentType, "video/"):
		format := strings.TrimPrefix(contentType, "video/")
		return format, consts.MEDIA_VIDEO
	default:
		return "", consts.MEDIA_UNKNOWN
	}
}

func GetCollectionMappingByID(filePath string, id int) (dto.CollectionMapping, error) {
	var (
		result    dto.CollectionMapping
		existedID = map[int]struct{}{}
		err       error
	)

	if id <= 0 {
		return result, fmt.Errorf("%w: id phải > 0", consts.ErrInvalidID)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return result, fmt.Errorf("không thể mở file: %w", consts.ErrFileNotFound)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return result, fmt.Errorf("không thể đọc file CSV: %w", consts.ErrParseFailed)
	}

	for i, row := range records {
		if i == 0 {
			continue
		}
		if len(row) < 2 {
			continue
		}
		idValue, err := strconv.Atoi(row[0])
		if err != nil {
			return result, err
		}
		if _, ok := existedID[idValue]; ok {
			return result, fmt.Errorf("Lỗi do file csv: %v", consts.ErrDuplicateID)
		}
		existedID[idValue] = struct{}{}
		if idValue == id {
			result = dto.CollectionMapping{
				ID:   idValue,
				Name: row[1],
			}
		}
	}

	if err == nil && !reflect.DeepEqual(result, dto.CollectionMapping{}) {
		return result, nil
	}

	return result, fmt.Errorf("%w: không tìm thấy ID %d trong file CSV", consts.ErrCollectionEmpty, id)
}

func DeleteFileCloudinary(cld *cloudinary.Cloudinary, urlId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_, err := cld.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID: urlId,
	})
	return err
}

func UploadFileCloudinary(cld *cloudinary.Cloudinary, file multipart.File, uploadFolder string, fileName string) (*uploader.UploadResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	uploadResult, err := cld.Upload.Upload(ctx, file, uploader.UploadParams{
		Folder:   uploadFolder,
		PublicID: fileName,
	})
	if err != nil {
		return nil, err
	}
	return uploadResult, nil
}
