package utils

import (
	"EventHunting/configs"
	"errors"

	"github.com/cloudinary/cloudinary-go/v2"
)

var (
	cld *cloudinary.Cloudinary
)

func InitCloudinary() error {
	cloudName := configs.GetCloudinaryName()
	apiKey := configs.GetCloudinaryKey()
	apiSecret := configs.GetCloudinarySecret()

	if cloudName == "" || apiKey == "" || apiSecret == "" {
		return errors.New("Lỗi: Các biến môi trường CLOUD_NAME, CLOUD_API_KEY, và CLOUD_API_SECRET là bắt buộc")
	}

	cldinary, err := cloudinary.NewFromParams(cloudName, apiKey, apiSecret)
	if err != nil {
		return err
	}
	cld = cldinary
	return nil
}

func GetCloudinary() *cloudinary.Cloudinary {
	return cld
}
