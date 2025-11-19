package controllers

import (
	"EventHunting/collections"
	"EventHunting/dto"
	"EventHunting/utils"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CreateTicketType(c *gin.Context) {
	var (
		req             dto.CreateTicketType
		eventEntry      = &collections.Event{}
		ticketTypeEntry = &collections.TicketType{}
		err             error
	)
	ctx := c.Request.Context()

	// Bind JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "Lỗi do bind dữ liệu", err.Error())
		return
	}

	// Validate định dạng
	if validateErrs := utils.ValidateCreateTicketType(req); len(validateErrs) > 0 {
		utils.ResponseError(c, http.StatusBadRequest, "Dữ liệu không hợp lệ", strings.Join(validateErrs, ", "))
		return
	}

	eventObjectID, _ := primitive.ObjectIDFromHex(req.EventID)

	// Lấy thông tin người tạo
	creatorID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	err = eventEntry.First(ctx, bson.M{"_id": eventObjectID, "deleted_at": bson.M{"$exists": false}})
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusBadRequest, "", "Sự kiện (Event ID) không tồn tại")
		return
	case err != nil:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống khi tìm sự kiện", err.Error())
		return
	}

	//Kiểm tra tên vé đã tồn tại trong sự kiện này chưa
	err = ticketTypeEntry.First(ctx, bson.M{
		"event_id":   req.EventID,
		"name":       req.Name,
		"deleted_at": bson.M{"$exists": false},
	})
	if err == nil {
		utils.ResponseError(c, http.StatusConflict, "", "Tên loại vé này đã tồn tại trong sự kiện")
		return
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống khi kiểm tra trùng tên vé", err.Error())
		return
	}

	// Tạo đối tượng
	now := time.Now()
	newTicketType := &collections.TicketType{
		ID:              primitive.NewObjectID(),
		Name:            req.Name,
		EventID:         eventObjectID,
		Price:           req.Price,
		Quantity:        req.Quantity, // nil nếu không cung cấp
		Status:          req.Status,
		RegisteredCount: 0,
		CreatedAt:       now,
		CreatedBy:       creatorID,
		UpdatedAt:       now,
		UpdatedBy:       creatorID,
	}

	if req.Description != nil {
		newTicketType.Description = *req.Description
	}
	// Lưu vào DB
	if err := newTicketType.Create(ctx); err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống", err.Error())
		return
	}

	//Trả về thành công
	utils.ResponseSuccess(c, http.StatusCreated, "", newTicketType, nil)
}

func UpdateTicketType(c *gin.Context) {
	var (
		req             dto.UpdateTicketType
		ticketTypeEntry = &collections.TicketType{}
		eventEntry      = &collections.Event{}
		err             error
	)
	ctx := c.Request.Context()

	ticketIDStr := c.Param("id")
	ticketID, err := primitive.ObjectIDFromHex(ticketIDStr)
	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "Ticket ID không hợp lệ", err.Error())
		return
	}

	// Bind JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "Lỗi do bind dữ liệu", err.Error())
		return
	}

	// Validate định dạng
	if validateErrs := utils.ValidateUpdateTicketType(req); len(validateErrs) > 0 {
		utils.ResponseError(c, http.StatusBadRequest, "Dữ liệu không hợp lệ", strings.Join(validateErrs, ", "))
		return
	}

	// Lấy thông tin người cập nhật và quyền
	updaterID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}
	roles, err := utils.GetRoles(c)
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi khi lấy quyền", err.Error())
		return
	}

	// Lấy loại vé hiện tại
	filter := bson.M{"_id": ticketID, "deleted_at": bson.M{"$exists": false}}
	err = ticketTypeEntry.First(ctx, filter)
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusNotFound, "", "Không tìm thấy loại vé")
		return
	case err != nil:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống khi tìm loại vé", err.Error())
		return
	}

	err = eventEntry.First(ctx, bson.M{"_id": ticketTypeEntry.EventID})
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusNotFound, "", "Không tìm thấy loại vé")
		return
	case err != nil:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống khi tìm loại vé", err.Error())
		return
	}

	// Kiểm tra quyền
	if !utils.CanModifyResource(eventEntry.CreatedBy, updaterID, roles) {
		utils.ResponseError(c, http.StatusForbidden, "", "Bạn không có quyền cập nhật loại vé này")
		return
	}

	// Xây dựng tài liệu cập nhật
	now := time.Now()
	updateFields := bson.M{
		"updated_at": now,
		"updated_by": updaterID,
	}
	unsetFields := bson.M{}

	if req.Name != nil {
		updateFields["name"] = *req.Name
	}
	if req.Description != nil {
		updateFields["description"] = *req.Description
	}
	if req.Price != nil {
		updateFields["price"] = *req.Price
	}
	if req.Status != nil {
		updateFields["status"] = *req.Status
	}

	// Logic xử lý Quantity
	if req.SetUnlimited != nil && *req.SetUnlimited {
		unsetFields["quantity"] = ""
	} else if req.Quantity != nil {
		updateFields["quantity"] = *req.Quantity
	}

	finalUpdateDoc := bson.M{"$set": updateFields}
	if len(unsetFields) > 0 {
		finalUpdateDoc["$unset"] = unsetFields
	}

	// Thực hiện cập nhật
	err = ticketTypeEntry.Update(ctx, filter, finalUpdateDoc)
	switch {
	case err == nil:
		utils.ResponseSuccess(c, http.StatusOK, "", nil, nil)
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống!", err.Error())
	}
}

func GetListTicketTypes(c *gin.Context) {
	var (
		ticketTypeEntry = &collections.TicketType{}
	)
	ctx := c.Request.Context()

	eventIDStr := c.Param("id")
	if eventIDStr == "" {
		utils.ResponseError(c, http.StatusBadRequest, "", "Bắt buộc phải cung cấp 'event_id' trong query")
		return
	}
	eventID, err := primitive.ObjectIDFromHex(eventIDStr)
	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "Event ID không hợp lệ", err.Error())
		return
	}

	filterSearch := bson.M{
		"event_id":   eventID,
		"deleted_at": bson.M{"$exists": false},
	}

	opts := options.Find().SetSort(bson.M{"created_at": 1})

	ticketTypes, err := ticketTypeEntry.Find(ctx, filterSearch, opts)

	switch {
	case err == nil:
		ticketTypesRes := []bson.M{}
		for _, ticketType := range ticketTypes {
			ticketTypesRes = append(ticketTypesRes, ticketType.ParseEntry())
		}
		utils.ResponseSuccess(c, http.StatusOK, "", ticketTypesRes, nil)
	case err != nil:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
	}
}

//func DeleteTicketType(c *gin.Context) {
//	var (
//		ticketTypeEntry = &collections.TicketType{}
//		eventEntry      = &collections.Event{}
//		err             error
//	)
//	ctx := c.Request.Context()
//
//	// 1. Lấy Ticket ID
//	ticketIDStr := c.Param("id")
//	ticketID, err := primitive.ObjectIDFromHex(ticketIDStr)
//	if err != nil {
//		utils.ResponseError(c, http.StatusBadRequest, "Ticket ID không hợp lệ", err.Error())
//		return
//	}
//
//	// Lấy thông tin người xóa và quyền
//	deleterID, ok := utils.GetAccountID(c)
//	if !ok {
//		return
//	}
//	roles, err := utils.GetRoles(c)
//	if err != nil {
//		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi khi lấy quyền", err.Error())
//		return
//	}
//
//	// Lấy loại vé hiện tại
//	filter := bson.M{"_id": ticketID, "deleted_at": bson.M{"$exists": false}}
//	err = ticketTypeEntry.First(ctx, filter)
//	switch {
//	case errors.Is(err, mongo.ErrNoDocuments):
//		utils.ResponseError(c, http.StatusNotFound, "", "Không tìm thấy loại vé")
//		return
//	case err != nil:
//		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống khi tìm loại vé", err.Error())
//		return
//	}
//
//	// Kiểm tra quyền
//	err = eventEntry.First(ctx, bson.M{"_id": ticketTypeEntry.EventID})
//	if err != nil {
//		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi khi tìm sự kiện cha", err.Error())
//		return
//	}
//	if !utils.CanModifyResource(eventEntry.CreatedBy, deleterID, roles) {
//		utils.ResponseError(c, http.StatusForbidden, "", "Bạn không có quyền xóa loại vé này")
//		return
//	}
//
//	// Validate nghiệp vụ: Không cho xóa nếu đã có người đăng ký
//	if ticketTypeEntry.RegisteredCount > 0 {
//		msg := fmt.Sprintf("Không thể xóa loại vé này vì đã có %d người đăng ký. Hãy hủy vé hoặc đặt số lượng về 0.", ticketTypeEntry.RegisteredCount)
//		utils.ResponseError(c, http.StatusBadRequest, "", msg)
//		return
//	}
//
//	// 6. Thực hiện Soft Delete
//	updateDoc := bson.M{
//		"$set": bson.M{
//			"deleted_at": time.Now(),
//			"deleted_by": deleterID,
//			"status":     consts.TicketTypeStatusCanceled,
//		},
//	}
//
//	err = ticketTypeEntry.Update(ctx, filter, updateDoc)
//	switch {
//	case err == nil:
//		utils.ResponseSuccess(c, http.StatusOK, "Xóa loại vé thành công", nil, nil)
//	default:
//		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống khi xóa", err.Error())
//	}
//}
