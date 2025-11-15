package consts

import "errors"

var (
	ErrInvalidID       = errors.New("ID không hợp lệ")
	ErrFileNotFound    = errors.New("Không tìm thấy file CSV")
	ErrParseFailed     = errors.New("Lỗi đọc hoặc parse file CSV")
	ErrCollectionEmpty = errors.New("Không tìm thấy collection tương ứng")
	ErrDuplicateID     = errors.New("Bị trùng id")
)
