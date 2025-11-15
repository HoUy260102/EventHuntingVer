package utils

import (
	"EventHunting/consts"
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

// Account
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

	// Validate UserInfo (cấp lồng nhau)
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

	// Validate OrganizerInfo (cấp lồng nhau)
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

		// data.OrganizerInfo.Decription (*string) không có logic validate cụ thể
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
	// Validate UserInfo
	// Giả định: Nếu gửi UserInfo, thì Dob là bắt buộc
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
	// Giả định: Nếu gửi OrganizerInfo, thì ContactName là bắt buộc
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

	// Validate UserInfo
	if data.UserInfo != nil {
		if data.UserInfo.Dob != nil {
			if data.UserInfo.Dob.IsZero() {
				validationErrors = append(validationErrors, "ngày sinh (dob) không hợp lệ")
			} else if data.UserInfo.Dob.After(time.Now()) {
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

	// Validate UserInfo
	if data.OrganizerInfo != nil {
		if data.OrganizerInfo.WebsiteUrl != nil {
			_, err := url.ParseRequestURI(*data.OrganizerInfo.WebsiteUrl)
			if err != nil {
				validationErrors = append(validationErrors, "đường dẫn website (website_url) không hợp lệ")
			}
		}
		if data.OrganizerInfo.ContactName != nil {
			if strings.TrimSpace(*data.OrganizerInfo.ContactName) == "" {
				validationErrors = append(validationErrors, "Contact name không được rỗng")
			}
		}
	}

	return validationErrors
}

// Topic
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

// Tag
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

// Blog
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

	if data.ThumbnailID != nil {
		if (*data.ThumbnailID).IsZero() {
			errs = append(errs, "thumbnail id không được rỗng")
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

	if data.ThumbnailID != nil {
		if (*data.ThumbnailID).IsZero() {
			errs = append(errs, "thumbnail id không được để trống")
		}
	}

	if data.MediaIDs != nil {
		if len(*data.MediaIDs) == 0 {
			errs = append(errs, "Phải gửi có media")
		}
	}
	return errs
}

// Event
const (
	timeFormat = "15:04"
)

func ValidateEventCreateReq(e dto.EventCreateReq) []string {
	var errors []string

	if strings.TrimSpace(e.Name) == "" {
		errors = append(errors, "Tên sự kiện là bắt buộc.")
	}
	if strings.TrimSpace(e.EventInfo) == "" {
		errors = append(errors, "Thông tin sự kiện (text) là bắt buộc.")
	}
	if strings.TrimSpace(e.EventInfoHtml) == "" {
		errors = append(errors, "Thông tin sự kiện (HTML) là bắt buộc.")
	}

	if strings.TrimSpace(e.EventLocation.Name) == "" {
		errors = append(errors, "Tên địa điểm là bắt buộc.")
	}
	if strings.TrimSpace(e.EventLocation.Address) == "" {
		errors = append(errors, "Địa chỉ sự kiện là bắt buộc.")
	}

	if e.EventLocation.MapURL != "" {
		if _, err := url.ParseRequestURI(e.EventLocation.MapURL); err != nil {
			errors = append(errors, "URL bản đồ không hợp lệ.")
		}
	}

	if e.Price < 0 {
		errors = append(errors, "Giá vé không thể là số âm.")
	}

	if e.MaxParticipants != nil && *e.MaxParticipants <= 0 {
		errors = append(errors, "Số người tham gia tối đa phải lớn hơn 0 (nếu được cung cấp).")
	}

	if e.TopicIDs != nil && len(*e.TopicIDs) == 0 {
		errors = append(errors, "Danh sách chủ đề (TopicIDs) không được rỗng (nếu được cung cấp).")
	}

	var (
		startTimeParsed, endTimeParsed time.Time
		errStartTime, errEndTime       error
	)

	if e.EventTime.StartDate.IsZero() {
		errors = append(errors, "Ngày bắt đầu là bắt buộc.")
	}
	if e.EventTime.EndDate.IsZero() {
		errors = append(errors, "Ngày kết thúc là bắt buộc.")
	}

	if strings.TrimSpace(e.EventTime.StartTime) == "" {
		errors = append(errors, "Giờ bắt đầu là bắt buộc.")
	} else {
		startTimeParsed, errStartTime = time.Parse(timeFormat, e.EventTime.StartTime)
		if errStartTime != nil {
			errors = append(errors, fmt.Sprintf("Định dạng giờ bắt đầu không hợp lệ (phải là %s).", timeFormat))
		}
	}

	if strings.TrimSpace(e.EventTime.EndTime) == "" {
		errors = append(errors, "Giờ kết thúc là bắt buộc.")
	} else {
		endTimeParsed, errEndTime = time.Parse(timeFormat, e.EventTime.EndTime)
		if errEndTime != nil {
			errors = append(errors, fmt.Sprintf("Định dạng giờ kết thúc không hợp lệ (phải là %s).", timeFormat))
		}
	}

	if !e.EventTime.StartDate.IsZero() && !e.EventTime.EndDate.IsZero() && errStartTime == nil && errEndTime == nil {

		// Kết hợp Ngày và Giờ
		yS, mS, dS := e.EventTime.StartDate.Date()
		startDateTime := time.Date(yS, mS, dS, startTimeParsed.Hour(), startTimeParsed.Minute(), 0, 0, e.EventTime.StartDate.Location())

		yE, mE, dE := e.EventTime.EndDate.Date()
		endDateTime := time.Date(yE, mE, dE, endTimeParsed.Hour(), endTimeParsed.Minute(), 0, 0, e.EventTime.EndDate.Location())

		// Ngày/Giờ bắt đầu phải ở trong tương lai
		if !startDateTime.After(time.Now()) {
			errors = append(errors, "Thời gian bắt đầu (ngày và giờ) phải ở trong tương lai.")
		}

		// Ngày/Giờ kết thúc phải sau Ngày/Giờ bắt đầu
		if !endDateTime.After(startDateTime) {
			errors = append(errors, "Thời gian kết thúc (ngày và giờ) phải sau thời gian bắt đầu.")
		}
	}
	if e.ProvinceID.IsZero() {
		errors = append(errors, "Trường province_id không được để trống.")
	}
	if len(errors) > 0 {
		return errors
	}

	return nil
}

func ValidateEventUpdateReq(req dto.EventUpdateReq) []string {
	var errs []string

	if req.Name != nil && *req.Name == "" {
		errs = append(errs, "Tên sự kiện không được để trống")
	}

	if req.EventInfo != nil && *req.EventInfo == "" {
		errs = append(errs, "Thông tin sự kiện (text) không được để trống")
	}

	if req.EventInfoHtml != nil && *req.EventInfoHtml == "" {
		errs = append(errs, "Thông tin sự kiện (HTML) không được để trống")
	}

	if req.Price != nil && *req.Price < 0 {
		errs = append(errs, "Giá vé không thể là số âm")
	}

	if req.MaxParticipants != nil && *req.MaxParticipants <= 0 {
		errs = append(errs, "Số người tham gia tối đa phải là số dương")
	}

	if req.EventTime != nil {
		et := req.EventTime

		if et.StartDate != nil && et.EndDate != nil {
			if et.EndDate.Before(*et.StartDate) {
				errs = append(errs, "Ngày kết thúc không thể trước ngày bắt đầu")
			}
		}

		if et.StartTime != nil {
			if *et.StartTime == "" {
				errs = append(errs, "Thời gian bắt đầu không được để trống")
			} else if _, err := time.Parse(timeFormat, *et.StartTime); err != nil {
				errs = append(errs, "Thời gian bắt đầu không hợp lệ hoặc sai định dạng HH:MM")
			}
		}

		if et.EndTime != nil {
			if *et.EndTime == "" {
				errs = append(errs, "Thời gian kết thúc không được để trống")
			} else if _, err := time.Parse(timeFormat, *et.EndTime); err != nil {
				errs = append(errs, "Thời gian kết thúc không hợp lệ hoặc sai định dạng HH:MM")
			}
		}
	}

	if req.EventLocation != nil {
		el := req.EventLocation

		if el.Name != nil && *el.Name == "" {
			errs = append(errs, "Tên địa điểm không được để trống")
		}

		if el.Address != nil && *el.Address == "" {
			errs = append(errs, "Địa chỉ không được để trống")
		}

	}

	if req.ProvinceID != nil && (*req.ProvinceID).IsZero() {
		errs = append(errs, "Trường province_id không được để trống.")
	}

	return errs
}

// Comment
func ValidateCommentCreate(req dto.CommentCreateReq) []string {
	var errors []string

	// Category là bắt buộc
	if req.Category == "" {
		errors = append(errors, "Loại (category) bình luận là bắt buộc.")
	}

	// Nếu Category là Blog Event, thì DocumentID là bắt buộc
	if (req.Category == consts.COMMENT_TYPE_BLOG || req.Category == consts.COMMENT_TYPE_EVENT) && req.DocumentID.IsZero() {
		errors = append(errors, "DocumentID là bắt buộc khi bình luận cho blog.")
	}

	// Phải có Content hoặc Medias
	if strings.TrimSpace(req.Content) == "" && len(req.MediaIds) == 0 {
		errors = append(errors, "Bình luận phải có nội dung văn bản hoặc media.")
	}

	if req.DocumentID.IsZero() {
		errors = append(errors, "Bình luận phải có document_id")
	}
	return errors
}

func ValidateCommentUpdate(req dto.CommentUpdateReq) []string {
	var errors []string

	// Phải có *ít nhất một* trường được gửi lên để cập nhật
	if req.Content == nil && req.ContentHTML == nil && req.MediaIds == nil {
		errors = append(errors, "Phải cung cấp ít nhất một trường (content, content_html, hoặc medias) để cập nhật.")
		return errors
	}

	// Nếu gửi Content, nó không được là chuỗi rỗng
	if req.Content != nil && strings.TrimSpace(*req.Content) == "" {
		errors = append(errors, "Nội dung (content) không được là chuỗi rỗng.")
	}

	return errors
}

// Registration
func ValidateCreateRegistrationReq(req dto.CreateRegistrationEventRequest) []string {
	errs := []string{}
	if len(req.Tickets) == 0 {
		errs = append(errs, "tickets không để trống")
	}
	for _, ticket := range req.Tickets {
		if ticket.Quantity <= 0 {
			errs = append(errs, "quantity phải lớn hơn 0")
		}
	}
	return errs
}

// Ticket type
func ValidateCreateTicketType(req dto.CreateTicketType) []string {
	var errs []string

	// Kiểm tra Tên
	if strings.TrimSpace(req.Name) == "" {
		errs = append(errs, "Tên loại vé là bắt buộc")
	}

	// Kiểm tra Mô tả
	if req.Description != nil && len(*req.Description) == 0 {
		errs = append(errs, "Mô tả không được trống")
	}

	// Kiểm tra Giá (0 nếu là free)
	if req.Price < 0 {
		errs = append(errs, "Giá không được là số âm")
	}

	// Kiểm tra Số lượng (nil = unlimited là hợp lệ)
	if req.Quantity != nil && *req.Quantity <= 0 {
		errs = append(errs, "Số lượng (nếu được cung cấp) phải lớn hơn 0")
	}

	// Kiểm tra Trạng thái (Status)
	if !isValidTicketStatus(req.Status) {
		errs = append(errs, "Trạng thái không hợp lệ")
	}

	return errs
}

func ValidateUpdateTicketType(req dto.UpdateTicketType) []string {
	var errs []string

	// Kiểm tra Tên
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			errs = append(errs, "Tên loại vé không được để trống")
		}
	}

	// Kiểm tra Mô tả
	if req.Description != nil && strings.TrimSpace(*req.Description) == "" {
		errs = append(errs, "Mô tả không được rỗng")
	}

	// Kiểm tra Giá
	if req.Price != nil && *req.Price < 0 {
		errs = append(errs, "Giá không được là số âm")
	}

	//Kiểm tra Trạng thái
	if req.Status != nil && !isValidTicketStatus(*req.Status) {
		errs = append(errs, "Trạng thái không hợp lệ")
	}

	// Kiểm tra logic Số lượng
	if req.SetUnlimited != nil && *req.SetUnlimited && req.Quantity != nil {
		errs = append(errs, "Không thể đồng thời đặt số lượng và đặt 'không giới hạn'")
	}
	// Nếu set số lượng cụ thể, nó phải > 0
	if req.Quantity != nil && *req.Quantity <= 0 {
		errs = append(errs, "Số lượng (nếu được cung cấp) phải lớn hơn 0")
	}

	return errs
}

// Hàm trợ giúp để kiểm tra Status
func isValidTicketStatus(status consts.TicketTypeStatus) bool {
	switch status {
	case consts.TicketTypeInactive,
		consts.TicketTypeActive,
		consts.TicketTypeCanceled:
		return true
	default:
		return false
	}
}
func init() {
	// Custom validator cho số điện thoại VN
	_ = Validator.RegisterValidation("phoneVn", func(fl validator.FieldLevel) bool {
		phone := fl.Field().String()
		matched, _ := regexp.MatchString(PhoneRegex, phone)
		return matched
	})
}
