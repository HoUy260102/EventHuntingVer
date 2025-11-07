package utils

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

var validFile = map[string]bool{
	".heic": true, //định dạng ảnh của ios 11
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".bmp":  true,
	".webp": true,
}

var validMiMe = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/bmp":  true,
	"image/webp": true,
	"image/heic": true,
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
		Folder:   uploadFolder, // folder lưu trên Cloudinary
		PublicID: fileName,     // đặt tên file = tên gốc + UUID
	})
	if err != nil {
		return nil, err
	}
	return uploadResult, nil
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
