package consts

import "errors"

var (
	ErrInvalidID         = errors.New("ID không hợp lệ")
	ErrFileNotFound      = errors.New("Không tìm thấy file CSV")
	ErrParseFailed       = errors.New("Lỗi đọc hoặc parse file CSV")
	ErrCollectionEmpty   = errors.New("Không tìm thấy collection tương ứng")
	ErrDuplicateID       = errors.New("Bị trùng id")
	ErrFatalDataNotFound = errors.New("dữ liệu không tồn tại trong DB -> drop job")
	ErrFatalInvalidData  = errors.New("dữ liệu sai định dạng logic -> drop job")

	ErrRegistrationNotFound = errors.New("không tìm thấy đăng ký hoặc chưa thanh toán")
	ErrEventNotFound        = errors.New("không tìm thấy sự kiện hoặc sự kiện không hoạt động")
	ErrAccountNotFound      = errors.New("không tìm thấy tài khoản đăng ký")
	ErrTicketAlreadySent    = errors.New("email vé đã được gửi trước đó")
	ErrTicketTypeFetch      = errors.New("lỗi khi lấy thông tin loại vé")
	ErrTicketProcessing     = errors.New("lỗi khi xử lý vé")
	ErrEmailBuild           = errors.New("lỗi khi tạo nội dung email")
)
