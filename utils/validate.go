package utils

import (
	"EventHunting/dto"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

var (
	Validator         *validator.Validate = validator.New()
	PhoneRegex        string              = `^(0|\+84)(3|5|7|8|9)\d{8}$`
	vietnamPhoneRegex                     = regexp.MustCompile(`^(0|\+84)(3|5|7|8|9)[0-9]{8}$`)
)

func HandlerValidation(err error) string {
	errValidator := ""
	if err == nil {
		return errValidator
	}
	if errVa, ok := err.(validator.ValidationErrors); ok {
		for _, e := range errVa {
			switch e.Tag() {
			case "required":
				errValidator += fmt.Sprintf("%s không được trống, ", strings.ToLower(e.Field()))
			case "email":
				errValidator += fmt.Sprintf("%s không phải là một email hợp lệ, ", strings.ToLower(e.Field()))
			case "phoneVn":
				errValidator += fmt.Sprintf("%s phải theo định dạng số phone Việt Nam, ", strings.ToLower(e.Field()))
			}
		}
		errValidator = strings.TrimSuffix(errValidator, ", ")
	}
	return errValidator
}

func ValidateUpdate(data dto.UpdateAccountReq) []string {
	var validationErrors []string

	// Validate Name (cấp ngoài cùng)
	if data.Name != nil {
		if strings.TrimSpace(*data.Name) == "" {
			validationErrors = append(validationErrors, "tên (name) không được để trống")
		}
	}

	// Validate Phone (cấp ngoài cùng)
	if data.Phone != nil {
		if !vietnamPhoneRegex.MatchString(*data.Phone) {
			validationErrors = append(validationErrors, "số điện thoại (phone) không đúng định dạng Việt Nam")
		}
	}

	// Validate UserInfor (cấp lồng nhau)
	if data.UserInfor != nil {
		// Chỉ validate các trường con không-nil bên trong UserInfor

		if data.UserInfor.Dob != nil {
			if data.UserInfor.Dob.IsZero() {
				validationErrors = append(validationErrors, "ngày sinh (dob) không được để trống")
			} else if data.UserInfor.Dob.After(time.Now()) {
				validationErrors = append(validationErrors, "ngày sinh (dob) không được ở tương lai")
			}
		}

	}

	// Validate OrganizerInfor (cấp lồng nhau)
	if data.OrganizerInfor != nil {
		// Chỉ validate các trường con không-nil bên trong OrganizerInfor

		if data.OrganizerInfor.ContactName != nil {
			if strings.TrimSpace(*data.OrganizerInfor.ContactName) == "" {
				validationErrors = append(validationErrors, "tên liên hệ (contact_name) của nhà tổ chức không được để trống")
			}
		}

		if data.OrganizerInfor.WebsiteUrl != nil {
			// Cho phép gửi lên string rỗng "" (để xóa url)
			// Nhưng nếu gửi lên string có nội dung, thì phải hợp lệ
			if *data.OrganizerInfor.WebsiteUrl != "" {
				_, err := url.ParseRequestURI(*data.OrganizerInfor.WebsiteUrl)
				if err != nil {
					validationErrors = append(validationErrors, "đường dẫn website (website_url) không hợp lệ")
				}
			}
		}

		// data.OrganizerInfor.Decription (*string) không có logic validate cụ thể
	}

	return validationErrors
}

func ValidateCreate(data dto.CreateAccount) []string {
	var validationErrors []string

	// --- 1. Validate các trường BẮT BUỘC ---
	// Validate Email
	if strings.TrimSpace(data.Email) == "" {
		validationErrors = append(validationErrors, "email (email) là bắt buộc")
	}

	// Validate Name
	if strings.TrimSpace(data.Name) == "" {
		validationErrors = append(validationErrors, "tên (name) là bắt buộc")
	}

	// Validate Password (Thêm logic độ dài tối thiểu, ví dụ: 6 ký tự)
	if strings.TrimSpace(data.Password) == "" {
		validationErrors = append(validationErrors, "mật khẩu (password) là bắt buộc")
	}

	// Validate RoleId
	if data.RoleId.IsZero() { // IsZero() là cách kiểm tra ObjectID rỗng (NilObjectID)
		validationErrors = append(validationErrors, "vai trò (role_id) là bắt buộc")
	}

	// --- 2. Validate các trường TÙY CHỌN (Nếu có) ---

	// Validate Phone
	if !vietnamPhoneRegex.MatchString(data.Phone) {
		validationErrors = append(validationErrors, "số điện thoại (phone) không đúng định dạng Việt Nam")
	}
	// Validate UserInfor
	// Giả định: Nếu gửi UserInfor, thì Dob là bắt buộc
	if data.UserInfor != nil {
		if data.UserInfor.Dob != nil {
			if data.UserInfor.Dob.IsZero() {
				validationErrors = append(validationErrors, "ngày sinh (dob) không được để trống")
			}
			if data.UserInfor.Dob.After(time.Now()) {
				validationErrors = append(validationErrors, "ngày sinh (dob) không được ở tương lai")
			}
		}
	}

	// Validate OrganizerInfor
	// Giả định: Nếu gửi OrganizerInfor, thì ContactName là bắt buộc
	if data.OrganizerInfor != nil {
		if data.OrganizerInfor.ContactName != nil && strings.TrimSpace(*data.OrganizerInfor.ContactName) == "" {
			validationErrors = append(validationErrors, "tên liên hệ (contact_name) là bắt buộc (khi cung cấp OrganizerInfor)")
		}

		if data.OrganizerInfor.WebsiteUrl != nil && *data.OrganizerInfor.WebsiteUrl != "" {
			_, err := url.ParseRequestURI(*data.OrganizerInfor.WebsiteUrl)
			if err != nil {
				validationErrors = append(validationErrors, "đường dẫn website (website_url) không hợp lệ")
			}
		}
	}

	return validationErrors
}

func ValidateCreateUser(data dto.CreateUserRequest) []string {
	var validationErrors []string

	// 1. Validate các trường BẮT BUỘC

	// Validate Email
	data.Email = strings.TrimSpace(data.Email)
	if data.Email == "" {
		validationErrors = append(validationErrors, "email là bắt buộc")
	} else {
		_, err := mail.ParseAddress(data.Email)
		if err != nil {
			validationErrors = append(validationErrors, "email không đúng định dạng")
		}
	}

	// Validate Name
	data.Name = strings.TrimSpace(data.Name)
	if data.Name == "" {
		validationErrors = append(validationErrors, "tên (name) là bắt buộc")
	}

	// Validate Password
	if data.Password == "" {
		validationErrors = append(validationErrors, "mật khẩu (password) là bắt buộc")
	}

	// --- 2. Validate các trường TÙY CHỌN (Nếu có) ---

	// Validate Phone
	if data.Phone != nil {
		phone := strings.TrimSpace(*data.Phone)
		if phone != "" && !vietnamPhoneRegex.MatchString(phone) {
			validationErrors = append(validationErrors, "số điện thoại (phone) không đúng định dạng Việt Nam")
		}
	}

	// Validate UserInfor
	if data.UserInfor != nil {
		if data.UserInfor.Dob != nil {
			if data.UserInfor.Dob.IsZero() {
				validationErrors = append(validationErrors, "ngày sinh (dob) không hợp lệ")
			} else if data.UserInfor.Dob.After(time.Now()) {
				validationErrors = append(validationErrors, "ngày sinh (dob) không được ở tương lai")
			}
		}
	}

	return validationErrors
}

func ValidateCreateOrganizer(data dto.CreateOrganizerRequest) []string {
	var validationErrors []string

	// 1. Validate các trường BẮT BUỘC

	// Validate Email
	data.Email = strings.TrimSpace(data.Email)
	if data.Email == "" {
		validationErrors = append(validationErrors, "email là bắt buộc")
	} else {
		_, err := mail.ParseAddress(data.Email)
		if err != nil {
			validationErrors = append(validationErrors, "email không đúng định dạng")
		}
	}

	// Validate Name
	data.Name = strings.TrimSpace(data.Name)
	if data.Name == "" {
		validationErrors = append(validationErrors, "tên (name) là bắt buộc")
	}

	// Validate Password
	if data.Password == "" {
		validationErrors = append(validationErrors, "mật khẩu (password) là bắt buộc")
	}

	// --- 2. Validate các trường TÙY CHỌN (Nếu có) ---

	// Validate Phone
	phone := strings.TrimSpace(data.Phone)
	if phone != "" && !vietnamPhoneRegex.MatchString(phone) {
		validationErrors = append(validationErrors, "số điện thoại (phone) không đúng định dạng Việt Nam")
	}

	// Validate UserInfor
	if data.OrganizerInfor != nil {
		if data.OrganizerInfor.WebsiteUrl != nil {
			_, err := url.ParseRequestURI(*data.OrganizerInfor.WebsiteUrl)
			if err != nil {
				validationErrors = append(validationErrors, "đường dẫn website (website_url) không hợp lệ")
			}
		}
		if data.OrganizerInfor.ContactName != nil {
			if strings.TrimSpace(*data.OrganizerInfor.ContactName) == "" {
				validationErrors = append(validationErrors, "Contact name không được rỗng")
			}
		}
	}

	return validationErrors
}

func ValidateTopicCreate(data dto.CreateTopicRequest) []string {
	var validationErrors []string
	if strings.TrimSpace(data.Name) == "" {
		validationErrors = append(validationErrors, fmt.Sprint("trường tên (name) không được trống"))
	}

	//Nếu truyền vào slug thì phải kiểm tra slug đó không rỗng
	if data.Slug != nil {
		if strings.TrimSpace(*data.Slug) == "" {
			validationErrors = append(validationErrors, fmt.Sprint("trường slug không được trống"))
		}
	}

	//Nếu truyền vào description thì phải kiểm tra description không rỗng
	if data.Description != nil {
		if strings.TrimSpace(*data.Description) == "" {
			validationErrors = append(validationErrors, fmt.Sprint("trường mô tả (description) không được trống"))
		}
	}
	return validationErrors
}

func ValidateTopicUpdate(data dto.UpdateTopicRequest) []string {
	var validationErrors []string
	if data.Name != nil {
		if strings.TrimSpace(*data.Name) == "" {
			validationErrors = append(validationErrors, fmt.Sprint("trường tên (name) không được trống"))
		}
	}

	//Nếu truyền vào slug thì phải kiểm tra slug đó không rỗng
	if data.Slug != nil {
		if strings.TrimSpace(*data.Slug) == "" {
			validationErrors = append(validationErrors, fmt.Sprint("trường slug không được trống"))
		}
	}

	//Nếu truyền vào description thì phải kiểm tra description không rỗng
	if data.Description != nil {
		if strings.TrimSpace(*data.Description) == "" {
			validationErrors = append(validationErrors, fmt.Sprint("trường mô tả (description) không được trống"))
		}
	}
	return validationErrors
}

func ValidateTagCreate(data dto.CreateTagRequest) []string {
	var validationErrors []string
	if strings.TrimSpace(data.Name) == "" {
		validationErrors = append(validationErrors, fmt.Sprint("trường tên (name) không được trống"))
	}

	//Nếu truyền vào slug thì phải kiểm tra slug đó không rỗng
	if data.Slug != nil {
		if strings.TrimSpace(*data.Slug) == "" {
			validationErrors = append(validationErrors, fmt.Sprint("trường slug không được trống"))
		}
	}

	//Nếu truyền vào description thì phải kiểm tra description không rỗng
	if data.Description != nil {
		if strings.TrimSpace(*data.Description) == "" {
			validationErrors = append(validationErrors, fmt.Sprint("trường mô tả (description) không được trống"))
		}
	}
	return validationErrors
}

func ValidateTagUpdate(data dto.UpdateTagRequest) []string {
	var validationErrors []string
	if data.Name != nil {
		if strings.TrimSpace(*data.Name) == "" {
			validationErrors = append(validationErrors, fmt.Sprint("trường tên (name) không được trống"))
		}
	}

	//Nếu truyền vào slug thì phải kiểm tra slug đó không rỗng
	if data.Slug != nil {
		if strings.TrimSpace(*data.Slug) == "" {
			validationErrors = append(validationErrors, fmt.Sprint("trường slug không được trống"))
		}
	}

	//Nếu truyền vào description thì phải kiểm tra description không rỗng
	if data.Description != nil {
		if strings.TrimSpace(*data.Description) == "" {
			validationErrors = append(validationErrors, fmt.Sprint("trường mô tả (description) không được trống"))
		}
	}
	return validationErrors
}

func ValidateCreateBlogRequest(data dto.CreateBlogRequest) []string {
	var errs []string
	if strings.TrimSpace(data.Title) == "" {
		errs = append(errs, "title không được rỗng")
	}
	if strings.TrimSpace(data.Content) == "" {
		errs = append(errs, "content không được rỗng")
	}
	if strings.TrimSpace(data.Content) == "" {
		errs = append(errs, "content không được rỗng")
	}
	if data.ThumbnailLink != nil {
		_, err := url.ParseRequestURI(*data.ThumbnailLink)
		if err != nil {
			errs = append(errs, "Đường dẫn ảnh thu nhỏ không hợp lệ")
		}
	}

	if data.ThumbnailPublicId != nil {
		if strings.TrimSpace(*data.ThumbnailPublicId) == "" {
			errs = append(errs, "thumbnail public id không được rỗng")
		}
	}

	return errs
}

func ValidateUpdateBlogRequest(data dto.UpdateBlogRequest) []string {
	var errs []string

	if data.Title != nil {
		if strings.TrimSpace(*data.Title) == "" {
			errs = append(errs, "Tiêu đề không được để trống")
		}
	}

	if data.Content != nil {
		if strings.TrimSpace(*data.Content) == "" {
			errs = append(errs, "Nội dung không được để trống")
		}
	}

	if data.ThumbnailLink != nil {
		if strings.TrimSpace(*data.ThumbnailLink) == "" {
			errs = append(errs, "Đường dẫn ảnh thu nhỏ không được để trống")
		} else {
			_, err := url.ParseRequestURI(*data.ThumbnailLink)
			if err != nil {
				errs = append(errs, "Đường dẫn ảnh thu nhỏ không hợp lệ")
			}
		}
	}

	if data.ThumbnailPublicId != nil {
		if strings.TrimSpace(*data.ThumbnailPublicId) == "" {
			errs = append(errs, "thumbnail public id không được để trống")
		}
	}

	return errs
}

func init() {
	// Custom validator cho số điện thoại VN
	_ = Validator.RegisterValidation("phoneVn", func(fl validator.FieldLevel) bool {
		phone := fl.Field().String()
		matched, _ := regexp.MatchString(PhoneRegex, phone)
		return matched
	})
}
