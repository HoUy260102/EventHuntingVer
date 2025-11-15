package utils

import (
	"EventHunting/consts"
	"EventHunting/dto"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

var listMethod = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

func GetSuccessMessageByMethod(method string) string {
	if !slices.Contains(listMethod, method) {
		return ""
	}

	message := ""
	switch method {
	case http.MethodGet:
		message = consts.MsgGetSuccess
	case http.MethodPost:
		message = consts.MsgCreateSuccess
	case http.MethodPut, http.MethodPatch:
		message = consts.MsgUpdateSuccess
	case http.MethodDelete:
		message = consts.MsgDeleteSuccess
	}

	return message
}

func GetErrorMessageByMethod(method string) string {
	if !slices.Contains(listMethod, method) {
		return ""
	}

	message := ""
	switch method {
	case http.MethodGet:
		message = consts.MsgGetErr
	case http.MethodPost:
		message = consts.MsgCreateErr
	case http.MethodPut, http.MethodPatch:
		message = consts.MsgUpdateErr
	case http.MethodDelete:
		message = consts.MsgDeleteErr
	default:
		message = "Lỗi hệ thống!"
	}

	return message
}

func ResponseSuccess(c *gin.Context, status int, msg string, data interface{}, pagination *dto.Pagination) {
	if strings.TrimSpace(msg) == "" {
		msg = GetSuccessMessageByMethod(c.Request.Method)
	}

	c.JSON(http.StatusOK, dto.ApiResponse{
		Status:     status,
		Message:    msg,
		Data:       data,
		Pagination: pagination,
	})
}

func ResponseError(c *gin.Context, status int, msg string, err interface{}) {
	if strings.TrimSpace(msg) == "" {
		msg = GetErrorMessageByMethod(c.Request.Method)
	}
	c.JSON(status, dto.ApiResponse{
		Status:  status,
		Message: msg,
		Error:   err,
	})
}

//var MessageResponse = map[int]string{
//	//Thành công
//	//http.StatusOK:        GetMessageByMethod("GET"),
//	//http.StatusCreated:   GetMessageByMethod("GET"),
//	http.StatusAccepted:  "Yêu cầu đã được chấp nhận.",
//	http.StatusNoContent: "Xóa thành công, không có nội dung trả về.",
//
//	// Lỗi do client
//	http.StatusBadRequest:      "Yêu cầu không hợp lệ!",
//	http.StatusUnauthorized:    "Chưa được xác thực hoặc token không hợp lệ!",
//	http.StatusForbidden:       "Bạn không có quyền truy cập tài nguyên này!",
//	http.StatusNotFound:        "Không tìm thấy tài nguyên!",
//	http.StatusConflict:        "Dữ liệu bị trùng lặp hoặc xung đột!",
//	http.StatusTooManyRequests: "Quá nhiều yêu cầu. Vui lòng thử lại sau!",
//
//	// Lỗi do server
//	http.StatusInternalServerError: "Lỗi hệ thống!",
//}
