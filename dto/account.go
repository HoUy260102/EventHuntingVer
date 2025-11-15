package dto

import (
	"net/url"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	vietnamPhoneRegex = regexp.MustCompile(`^(0|\+84)(3|5|7|8|9)[0-9]{8}$`)
)

type UpdateAccountReq struct {
	Name          *string       `json:"name"`
	Phone         *string       `json:"phone"`
	Address       *string       `json:"address"`
	UserInfo      *UserDTO      `json:"user_info"`
	OrganizerInfo *OrganizerDTO `json:"organizer_info"`
}

// Validate cho updateAccountRequest
func ValidateUpdateAccountReq(data UpdateAccountReq) []string {
	var validationErrors []string

	// Validate Name
	if data.Name != nil {
		if strings.TrimSpace(*data.Name) == "" {
			validationErrors = append(validationErrors, "tên (name) không được để trống")
		}
	}

	// Validate Phone
	if data.Phone != nil {
		if !vietnamPhoneRegex.MatchString(*data.Phone) {
			validationErrors = append(validationErrors, "số điện thoại (phone) không đúng định dạng Việt Nam")
		}
	}

	// Validate Address
	if data.Address != nil {
		if strings.TrimSpace(*data.Address) == "" {
			validationErrors = append(validationErrors, "địa chỉ (address) không được để trống")
		}
	}

	// Validate UserInfo
	if data.UserInfo != nil {
		// Chỉ validate các trường con không-nil bên trong UserInfo

		if data.UserInfo.Dob != nil {
			if data.UserInfo.Dob.IsZero() {
				validationErrors = append(validationErrors, "ngày sinh (dob) không được để trống")
			} else if data.UserInfo.Dob.After(time.Now()) {
				validationErrors = append(validationErrors, "ngày sinh (dob) không được ở tương lai")
			}
		}

	}

	// Validate OrganizerInfo
	if data.OrganizerInfo != nil {
		// Chỉ validate các trường con không-nil bên trong OrganizerInfo

		if data.OrganizerInfo.ContactName != nil {
			if strings.TrimSpace(*data.OrganizerInfo.ContactName) == "" {
				validationErrors = append(validationErrors, "tên liên hệ (contact_name) của nhà tổ chức không được để trống")
			}
		}

		if data.OrganizerInfo.WebsiteUrl != nil {
			// Cho phép gửi lên string rỗng "" (để xóa url)
			// Nhưng nếu gửi lên string có nội dung, thì phải hợp lệ
			if *data.OrganizerInfo.WebsiteUrl != "" {
				_, err := url.ParseRequestURI(*data.OrganizerInfo.WebsiteUrl)
				if err != nil {
					validationErrors = append(validationErrors, "đường dẫn website (website_url) không hợp lệ")
				}
			}
		}

	}

	return validationErrors
}

type CreateAccount struct {
	Email    string             `json:"email"`
	Name     string             `json:"name"`
	Password string             `json:"password"`
	RoleId   primitive.ObjectID `json:"role_id"`
	Phone    string             `json:"phone"`

	Address       *string       `json:"address"`
	UserInfo      *UserDTO      `json:"user_info"`
	OrganizerInfo *OrganizerDTO `json:"organizer_info"`
}

type OrganizerDTO struct {
	Decription  *string `json:"decription,omitempty"`
	WebsiteUrl  *string `json:"website_url,omitempty"`
	ContactName *string `json:"contact_name,omitempty"`
}

type UserDTO struct {
	Dob    *time.Time `json:"dob,omitempty"`
	IsMale *bool      `json:"is_male,omitempty"`
}

// Validate cho CreateAccountRequest
func ValidateCreateAccountRequest(data CreateAccount) []string {
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

	// Validate Phone
	if !vietnamPhoneRegex.MatchString(data.Phone) {
		validationErrors = append(validationErrors, "số điện thoại (phone) không đúng định dạng Việt Nam")
	}
	// --- 2. Validate các trường TÙY CHỌN (Nếu có) ---

	//Validate địa chỉ
	if data.Address != nil {
		if strings.TrimSpace(*data.Address) != "" {
			validationErrors = append(validationErrors, "địa chỉ (address) không nên để trống")
		}
	}

	// Validate UserInfo
	if data.UserInfo != nil {
		if data.UserInfo.Dob != nil {
			if data.UserInfo.Dob.IsZero() {
				validationErrors = append(validationErrors, "ngày sinh (dob) không được để trống")
			}
			if data.UserInfo.Dob.After(time.Now()) {
				validationErrors = append(validationErrors, "ngày sinh (dob) không được ở tương lai")
			}
		}
	}

	// Validate OrganizerInfo
	if data.OrganizerInfo != nil {
		if data.OrganizerInfo.ContactName != nil && strings.TrimSpace(*data.OrganizerInfo.ContactName) == "" {
			validationErrors = append(validationErrors, "tên liên hệ (contact_name) là bắt buộc (khi cung cấp OrganizerInfo)")
		}

		if data.OrganizerInfo.WebsiteUrl != nil && *data.OrganizerInfo.WebsiteUrl != "" {
			_, err := url.ParseRequestURI(*data.OrganizerInfo.WebsiteUrl)
			if err != nil {
				validationErrors = append(validationErrors, "đường dẫn website (website_url) không hợp lệ")
			}
		}
	}

	return validationErrors
}

// LockAccountRequest
type LockAccountRequest struct {
	Message string     `json:"message" binding:"required"`
	Until   *time.Time `json:"until,omitempty"`
}

type CreateUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`

	Phone    *string  `json:"phone"`
	UserInfo *UserDTO `json:"user_info"`
}

type CreateOrganizerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`

	Phone         string        `json:"phone"`
	OrganizerInfo *OrganizerDTO `json:"organizer_info,omitempty"`
}
