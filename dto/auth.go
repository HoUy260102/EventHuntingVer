package dto

import (
	"regexp"
	"strings"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func ValidateLoginRequest(req LoginRequest) []string {
	var errs []string

	// Kiểm tra Email
	req.Email = strings.TrimSpace(req.Email) // Xóa khoảng trắng
	if req.Email == "" {
		errs = append(errs, "Trường email không được trống")
	} else if !emailRegex.MatchString(req.Email) {
		errs = append(errs, "Trường email không đúng định dạng")
	}

	// Kiểm tra Password
	req.Password = strings.TrimSpace(req.Password)
	if req.Password == "" {
		errs = append(errs, "Trường mật khẩu không được trống")
	} else if len(req.Password) < 6 {
		errs = append(errs, "Mật khẩu phải có ít nhất 6 ký tự")
	}

	// Trả về kết quả
	if len(errs) > 0 {
		return errs
	}

	return nil
}
