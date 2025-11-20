package controllers

import (
	"EventHunting/collections"
	"EventHunting/consts"
	"EventHunting/database"
	"EventHunting/dto"
	"EventHunting/jobs"
	"EventHunting/service"
	"EventHunting/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"time"

	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type VNPAYIPNResponse struct {
	RspCode string `json:"RspCode"`
	Message string `json:"Message"`
}

// Xử lý call back url thanh toán

func HandleCallbackVNPAY(c *gin.Context) {
	vnpParams := c.Request.URL.Query()

	// Verify Checksum
	if err := utils.VerifyIPNChecksum(vnpParams); err != nil {
		log.Printf("ERROR: VNPAY IPN Checksum thất bại: %v", err)
		utils.ResponseError(c, http.StatusBadRequest, "Lỗi sai chữ ký!", err.Error())
		return
	}

	vnp_ResponseCode := vnpParams.Get("vnp_ResponseCode")
	vnp_TxnRef := vnpParams.Get("vnp_TxnRef")
	vnp_Amount_str := vnpParams.Get("vnp_Amount")
	vnp_TxnRefObjectID, _ := primitive.ObjectIDFromHex(vnp_TxnRef)
	vnp_TransactionNo := vnpParams.Get("vnp_TransactionNo")

	// Kiểm tra mã lỗi từ VNPAY
	if vnp_ResponseCode != "00" {
		log.Printf("WARN: VNPAY IPN: Giao dịch %s thất bại. Code: %s", vnp_TxnRef, vnp_ResponseCode)
		utils.ResponseSuccess(c, http.StatusOK, "Payment Failed confirmed", VNPAYIPNResponse{
			RspCode: "00",
			Message: "Confirm",
		}, nil)
		return
	}

	// Tìm Registration trong DB
	regisEntry := &collections.Registration{}
	// Nếu đã PAID rồi thì trả về thành công luôn
	checkPaidErr := regisEntry.First(nil, bson.M{"_id": vnp_TxnRefObjectID, "status": consts.RegistrationPaid})
	if checkPaidErr == nil {
		utils.ResponseSuccess(c, http.StatusOK, "Order already confirmed", VNPAYIPNResponse{
			RspCode: "00",
			Message: "Confirm",
		}, nil)
		return
	}

	// Tìm đơn đang PENDING
	regisFilter := bson.M{
		"_id":    vnp_TxnRefObjectID,
		"status": consts.RegistrationPending,
	}
	err := regisEntry.First(nil, utils.GetFilter(regisFilter))

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ERROR: VNPAY IPN: Không tìm thấy TxnRef %s (Pending) trong DB", vnp_TxnRef)
			utils.ResponseError(c, http.StatusBadRequest, "Order not found", err.Error())
			return
		}
		log.Printf("ERROR: VNPAY IPN: Lỗi DB khi tìm TxnRef %s: %v", vnp_TxnRef, err)
		utils.ResponseError(c, http.StatusInternalServerError, "System Error", err.Error())
		return
	}

	// Kiểm tra số tiền
	vnp_Amount, _ := strconv.ParseInt(vnp_Amount_str, 10, 64)
	if vnp_Amount != int64(regisEntry.TotalPrice*100) {
		log.Printf("ERROR: VNPAY IPN: Sai số tiền. TxnRef %s. VNPAY: %d, DB: %d", vnp_TxnRef, vnp_Amount, (regisEntry.TotalPrice * 100))
		utils.ResponseError(c, http.StatusBadRequest, "Invalid Amount", nil)
		return
	}

	//Tạo hóa đơn
	newInvoice, err := service.CreateInvoiceForRegistration(vnp_TxnRefObjectID, vnp_TransactionNo, time.Now())
	if err != nil {
		log.Printf("ERROR: Lỗi prepare invoice data: %v", err)
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi dữ liệu", err.Error())
		return
	}

	client := database.GetDB().Client()
	session, err := client.StartSession()
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi kết nối DB Session", err.Error())
		return
	}
	defer session.EndSession(context.Background())

	callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
		if err := newInvoice.Create(sessCtx); err != nil {
			return nil, err
		}

		regisCollection := database.GetDB().Collection("registrations")
		update := bson.M{
			"$set": bson.M{
				"status":                   consts.RegistrationPaid,
				"invoice_id":               newInvoice.ID,
				"updated_at":               time.Now(),
				"payment_transaction_code": vnp_TransactionNo,
			},
		}
		_, err := regisCollection.UpdateOne(sessCtx, bson.M{"_id": vnp_TxnRefObjectID}, update)
		if err != nil {
			return nil, err
		}

		return nil, nil
	}

	_, err = session.WithTransaction(context.Background(), callback)
	if err != nil {
		log.Printf("CRITICAL: Transaction thất bại cho đơn %s: %v", vnp_TxnRef, err)
		utils.ResponseError(c, http.StatusInternalServerError, "Giao dịch thất bại", err.Error())
		return
	}

	// ĐẨY JOB VÀO REDIS
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		jobData := jobs.EmailJob{
			Type:       "ticket_email",
			Data:       bson.M{"registration_id": vnp_TxnRef}, // Lưu ID dưới dạng string cho an toàn JSON
			RetryCount: 0,
		}

		jobPayload, _ := json.Marshal(jobData)
		rdb := database.GetRedisClient().Client

		//Đẩy vào cuối hàng đợi
		if err := rdb.RPush(ctx, consts.QueueNameEmail, jobPayload).Err(); err != nil {
			log.Printf("CRITICAL: Đơn %s đã PAID nhưng đẩy Redis thất bại: %v", vnp_TxnRef, err)
		} else {
			log.Printf("INFO: Đã đẩy job sinh vé cho đơn %s vào queue.", vnp_TxnRef)
		}
	}()

	//Phản hồi thành công cho VNPAY
	log.Printf("INFO: VNPAY IPN: Xử lý thành công đơn hàng %s", vnp_TxnRef)
	utils.ResponseSuccess(c, http.StatusOK, "", VNPAYIPNResponse{
		RspCode: "00",
		Message: "Confirm",
	}, nil)
}

func RegistrationEvent(c *gin.Context) {

	var req dto.CreateRegistrationEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "Dữ liệu đầu vào không hợp lệ!", err.Error())
		return
	}

	eventID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "Event ID không hợp lệ!", err.Error())
		return
	}

	//Lấy id người tạo đăng ký
	creatorID, ok := utils.GetAccountID(c)
	if !ok {
		utils.ResponseError(c, http.StatusUnauthorized, "Yêu cầu xác thực!", nil)
		return
	}

	//Validate đầu vào
	validationErrors, lstTicketTypes, requestedTicketsMap, totalPrice, totalNewTickets := validateRegistrationRules(
		req,
		eventID,
		creatorID,
	)

	if len(validationErrors) > 0 {
		utils.ResponseError(c, http.StatusBadRequest, "Yêu cầu không hợp lệ:", validationErrors)
		return
	}

	client := database.GetDB().Client()
	session, err := client.StartSession()
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống (session)!", err.Error())
		return
	}
	defer session.EndSession(c.Request.Context())

	result, err := session.WithTransaction(c.Request.Context(),
		func(sessionContext mongo.SessionContext) (interface{}, error) {

			var newRegistration collections.Registration
			var err error
			var ticketTypeEntry = &collections.TicketType{}

			for _, tickType := range lstTicketTypes {
				requestedQty := requestedTicketsMap[tickType.ID]

				var ticketTypeFilter bson.M
				//Logic xử lý nếu số lượng có giới hạn/ ko giới hạn
				if tickType.Quantity == nil {
					ticketTypeFilter = bson.M{
						"_id": tickType.ID,
					}
				} else {
					ticketTypeFilter = bson.M{
						"_id":              tickType.ID,
						"registered_count": bson.M{"$lte": *tickType.Quantity - requestedQty},
					}
				}
				ticketTypeFilter = utils.GetFilter(ticketTypeFilter)
				ticketTypeUpdate := bson.M{
					"$inc": bson.M{"registered_count": requestedQty},
				}

				err = ticketTypeEntry.Update(sessionContext, ticketTypeFilter, ticketTypeUpdate)
				if err != nil {
					if errors.Is(err, mongo.ErrNoDocuments) {
						return nil, fmt.Errorf("Vé '%s' đã hết trong lúc bạn thao tác, vui lòng thử lại.", tickType.Name)
					}
					return nil, err
				}
			}

			var eventEntry collections.Event
			err = eventEntry.Update(sessionContext,
				bson.M{"_id": eventID},
				bson.M{"$inc": bson.M{"number_of_participants": totalNewTickets}},
			)
			if err != nil {
				return nil, errors.New("Lỗi hệ thống khi cập nhật sự kiện!")
			}

			now := time.Now()

			newRegistration = collections.Registration{
				ID:            primitive.NewObjectID(),
				EventID:       eventID,
				Tickets:       req.Tickets,
				TotalQuantity: totalNewTickets,
				TotalPrice:    totalPrice,
				Status:        consts.RegistrationPending,
				CreatedBy:     creatorID,
				UpdatedBy:     creatorID,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			err = newRegistration.Create(sessionContext)
			if err != nil {
				return nil, errors.New("Lỗi hệ thống khi tạo đăng ký!")
			}

			return newRegistration, nil
		})

	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "", err.Error())
		return
	}

	newRegistration, ok := result.(collections.Registration)
	if !ok {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống!", nil)
		return
	}

	//Tạo url thanh toán
	ipAddr := utils.GetClientIpAdrr(c)
	orderInfo := url.QueryEscape(newRegistration.ID.Hex())
	urlRe, _ := utils.CreatePaymentURL(newRegistration.ID.Hex(), int64(newRegistration.TotalPrice), orderInfo, ipAddr)
	utils.ResponseSuccess(c, http.StatusCreated, "Đăng ký thành công!", bson.M{
		"registration": newRegistration,
		"url":          urlRe,
	}, nil)
}

func validateRegistrationRules(
	req dto.CreateRegistrationEventRequest,
	eventID primitive.ObjectID,
	creatorID primitive.ObjectID,
) (
	[]string,
	[]collections.TicketType,
	map[primitive.ObjectID]int,
	int,
	int,
) {

	var validationErrors []string
	var lstTicketTypes []collections.TicketType
	var requestedTicketsMap = make(map[primitive.ObjectID]int)
	var totalPrice = 0
	var totalNewTickets = 0

	if len(req.Tickets) <= 0 {
		validationErrors = append(validationErrors, "Bạn phải chọn ít nhất 1 vé!")
		return validationErrors, lstTicketTypes, requestedTicketsMap, totalPrice, totalNewTickets
	}

	var eventEntry collections.Event
	//User có thể đăng ký các event của ng khác
	eventFilter := bson.M{
		"_id": eventID,
	}
	err := eventEntry.First(nil, utils.GetFilter(eventFilter))
	if err != nil {
		validationErrors = append(validationErrors, "Sự kiện không tồn tại hoặc đã bị xóa.")
		return validationErrors, lstTicketTypes, requestedTicketsMap, totalPrice, totalNewTickets
	}

	//Kiểm tra thời hạn đăng ký
	loc := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	now := time.Now()
	if !eventEntry.EventTime.EndDate.IsZero() {

		deadline := eventEntry.EventTime.EndDate
		if eventEntry.EventTime.StartTime != "" {
			parsedStart, err := time.Parse("15:04", eventEntry.EventTime.StartTime)
			if err == nil {
				deadline = time.Date(
					deadline.Year(), deadline.Month(), deadline.Day(),
					parsedStart.Hour(), parsedStart.Minute(), 0, 0,
					loc,
				)
			}
		}
		minutesToSubtract := 60
		deadline = deadline.Add(time.Duration(-minutesToSubtract) * time.Minute)
		if now.After(deadline) {
			timeStr := deadline.Format("15:04 02/01/2006")
			msg := fmt.Sprintf("Đã hết hạn đăng ký. Sự kiện ngày cuối cùng đã bắt đầu lúc %s.", timeStr)
			validationErrors = append(validationErrors, msg)
			return validationErrors, lstTicketTypes, requestedTicketsMap, totalPrice, totalNewTickets
		}
	}

	var ticketTypeIDs []primitive.ObjectID
	hasInvalidQuantity := false

	for _, ticket := range req.Tickets {
		if ticket.Quantity <= 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("Số lượng cho vé ID %s phải lớn hơn 0.", ticket.TicketTypeID.Hex()))
			hasInvalidQuantity = true
		}
		ticketTypeIDs = append(ticketTypeIDs, ticket.TicketTypeID)
		requestedTicketsMap[ticket.TicketTypeID] = ticket.Quantity
	}

	if hasInvalidQuantity {
		return validationErrors, lstTicketTypes, requestedTicketsMap, totalPrice, totalNewTickets
	}

	ticketTypeEntry := collections.TicketType{}
	ticketTypeFilter := bson.M{
		"_id":      bson.M{"$in": ticketTypeIDs},
		"event_id": eventID,
	}
	lstTicketTypes, err = ticketTypeEntry.Find(nil, utils.GetFilter(ticketTypeFilter))

	if err != nil {
		validationErrors = append(validationErrors, "Lỗi khi tìm thông tin vé!")
		return validationErrors, lstTicketTypes, requestedTicketsMap, totalPrice, totalNewTickets
	}

	if len(lstTicketTypes) != len(requestedTicketsMap) {
		validationErrors = append(validationErrors, "Một hoặc nhiều loại vé không hợp lệ hoặc không thuộc sự kiện này!")
		return validationErrors, lstTicketTypes, requestedTicketsMap, totalPrice, totalNewTickets
	}

	ownedTicketTypeMap := make(map[primitive.ObjectID]int)
	totalAlreadyRegistered := 0

	//Nếu phát sinh ra nhiều kiểu đăng ký khác (event, ...) (EvtRegistration)
	regisEntry := collections.Registration{}
	regisFilter := bson.M{
		"event_id":   eventID,
		"created_by": creatorID,
		"status": bson.M{"$in": []consts.EventRegistrationStatus{
			consts.RegistrationPending,
			consts.RegistrationPaid,
		}},
	}
	existingRegs, err := regisEntry.Find(nil, utils.GetFilter(regisFilter))

	if err != nil {
		validationErrors = append(validationErrors, "Lỗi khi kiểm tra vé đã đăng ký!")
		return validationErrors, lstTicketTypes, requestedTicketsMap, totalPrice, totalNewTickets
	}

	for _, reg := range existingRegs {
		totalAlreadyRegistered += reg.TotalQuantity
		for _, ticket := range reg.Tickets {
			ownedTicketTypeMap[ticket.TicketTypeID] += ticket.Quantity
		}
	}

	totalNewTickets = 0
	totalPrice = 0

	for _, tickType := range lstTicketTypes {
		requestedQty := requestedTicketsMap[tickType.ID]
		isFreeTicket := (tickType.Price == 0)

		if isFreeTicket {
			if requestedQty > 1 {
				validationErrors = append(validationErrors, fmt.Sprintf("Vé miễn phí '%s' chỉ cho phép đăng ký tối đa 1 vé.", tickType.Name))
			}
			if ownedQty, ok := ownedTicketTypeMap[tickType.ID]; ok && ownedQty > 0 {
				validationErrors = append(validationErrors, fmt.Sprintf("Bạn đã sở hữu (hoặc đang giữ) vé miễn phí '%s' này rồi.", tickType.Name))
			}
		}

		if tickType.Quantity != nil {
			if stockLeft := *tickType.Quantity - tickType.RegisteredCount; stockLeft < 0 || stockLeft < requestedQty {
				validationErrors = append(validationErrors, fmt.Sprintf("Vé '%s' chỉ còn %d vé (bạn yêu cầu %d)", tickType.Name, stockLeft, requestedQty))
			}
		}

		totalNewTickets += requestedQty
		totalPrice += tickType.Price * requestedQty
	}

	if totalAlreadyRegistered+totalNewTickets > eventEntry.MaxTicketPerUser {
		validationErrors = append(validationErrors, fmt.Sprintf("Bạn đang giữ %d vé. Bạn chỉ được đăng ký tổng cộng %d vé cho sự kiện này.", totalAlreadyRegistered, eventEntry.MaxTicketPerUser))
	}

	return validationErrors, lstTicketTypes, requestedTicketsMap, totalPrice, totalNewTickets
}

//func HandleCallbackVNPAY(c *gin.Context) {
//	vnpParams := c.Request.URL.Query()
//
//	if err := utils.VerifyIPNChecksum(vnpParams); err != nil {
//		log.Printf("ERROR: VNPAY IPN Checksum thất bại: %v", err)
//		utils.ResponseError(c, http.StatusBadRequest, "Lỗi sai chữ ký khi call back url thanh toán!", err.Error())
//		return
//	}
//
//	vnp_ResponseCode := vnpParams.Get("vnp_ResponseCode")
//	vnp_TxnRef := vnpParams.Get("vnp_TxnRef")
//	vnp_Amount_str := vnpParams.Get("vnp_Amount")
//	vnp_TxnRefObjectID, _ := primitive.ObjectIDFromHex(vnp_TxnRef)
//
//	if vnp_ResponseCode != "00" {
//		log.Printf("WARN: VNPAY IPN: Giao dịch %s thất bại. Code: %s", vnp_TxnRef, vnp_ResponseCode)
//		utils.ResponseError(c, http.StatusBadRequest, utils.ResponsePaymentMessage(vnp_ResponseCode), nil)
//		return
//	}
//
//	// Tìm Registration (đơn hàng) trong DB
//	regisEntry := &collections.Registration{}
//	regisFilter := bson.M{
//		"_id":    vnp_TxnRefObjectID,
//		"status": consts.RegistrationPending,
//	}
//
//	err := regisEntry.First(nil, utils.GetFilter(regisFilter))
//	if err != nil {
//		if errors.Is(err, mongo.ErrNoDocuments) {
//			log.Printf("ERROR: VNPAY IPN: Không tìm thấy TxnRef %s trong DB", vnp_TxnRef)
//			utils.ResponseError(c, http.StatusBadRequest, "Không tìm thấy đơn hàng!", err.Error())
//			return
//		}
//		log.Printf("ERROR: VNPAY IPN: Lỗi DB khi tìm TxnRef %s: %v", vnp_TxnRef, err)
//		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
//		return
//	}
//
//	//Kiểm tra số tiền
//	vnp_Amount, _ := strconv.ParseInt(vnp_Amount_str, 10, 64)
//	if vnp_Amount != int64(regisEntry.TotalPrice*100) {
//		log.Printf("ERROR: VNPAY IPN: Sai số tiền. TxnRef %s. VNPAY: %d, DB: %d", vnp_TxnRef, vnp_Amount, (regisEntry.TotalPrice * 100))
//		utils.ResponseError(c, http.StatusBadRequest, "Số tiền thanh toán không hợp lệ!", nil)
//		return
//	}
//
//	// CẬP NHẬT TRẠNG THÁI "PAID"
//	err = regisEntry.Update(nil,
//		utils.GetFilter(regisFilter),
//		bson.M{"$set": bson.M{"status": consts.RegistrationPaid}},
//	)
//
//	if err != nil {
//		log.Printf("ERROR: VNPAY IPN: Không thể update status 'paid' cho TxnRef %s: %v", vnp_TxnRef, err)
//		utils.ResponseError(c, http.StatusBadRequest, "Lỗi do hệ thống!", err.Error())
//		return
//	}
//
//	// Gọi hàm gửi vé
//	err = service.GenerateAndSendTickets(vnp_TxnRefObjectID)
//	if err != nil {
//		log.Printf("ERROR: VNPAY IPN: Xử lý đơn %s thành công, nhưng GỬI VÉ thất bại: %v", vnp_TxnRef, err)
//		if errors.Is(err, consts.ErrRegistrationNotFound) {
//			utils.ResponseError(c, http.StatusBadRequest, "Không tìm thấy đăng ký!", err.Error())
//			return
//		}
//		if errors.Is(err, consts.ErrEventNotFound) {
//			utils.ResponseError(c, http.StatusBadRequest, "Không tìm thấy sự kiện đăng ký!", err.Error())
//			return
//		}
//		if errors.Is(err, consts.ErrAccountNotFound) {
//			utils.ResponseError(c, http.StatusBadRequest, "Không tìm thấy tài khoản đăng ký!", err.Error())
//			return
//		}
//		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
//		return
//	}
//
//	log.Printf("INFO: VNPAY IPN: Đã xử lý thành công đơn hàng %s", vnp_TxnRef)
//	utils.ResponseSuccess(c, http.StatusOK, "", VNPAYIPNResponse{
//		RspCode: "00",
//		Message: "Confirm",
//	}, nil)
//}
