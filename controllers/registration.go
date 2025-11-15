package controllers

import (
	"EventHunting/collections"
	"EventHunting/configs"
	"EventHunting/consts"
	"EventHunting/database"
	"EventHunting/dto"
	"EventHunting/utils"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const maxTicketPerUser = 6

func RegistrationEvent(c *gin.Context) {

	var req dto.CreateRegistrationEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "Dữ liệu đầu vào không hợp lệ!", err.Error())
		return
	}

	if validateErrs := utils.ValidateCreateRegistrationReq(req); len(validateErrs) > 0 {
		utils.ResponseError(c, http.StatusBadRequest, "Dữ liệu đầu vào không hợp lệ!", validateErrs)
		return
	}

	if len(req.Tickets) == 0 {
		utils.ResponseError(c, http.StatusBadRequest, "Bạn phải chọn ít nhất 1 vé!", nil)
		return
	}
	eventID := c.Param("id")
	eventIDConvert, _ := primitive.ObjectIDFromHex(eventID)

	creatorID, ok := utils.GetAccountID(c)
	if !ok {
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

			var eventEntry collections.Event

			err := eventEntry.First(sessionContext, bson.M{
				"_id":        eventIDConvert,
				"deleted_at": bson.M{"$exists": false},
			})
			if err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					return nil, fmt.Errorf("Không tìm thấy sự kiện để đăng ký:%w!", mongo.ErrNoDocuments)
				}
				return nil, errors.New("Lỗi hệ thống khi tìm sự kiện!")
			}

			// Lấy thông tin các Loại Vé
			var ticketTypeIDs []primitive.ObjectID
			requestedTicketsMap := make(map[primitive.ObjectID]int)
			for _, ticket := range req.Tickets {
				ticketTypeIDs = append(ticketTypeIDs, ticket.TicketTypeID)
				requestedTicketsMap[ticket.TicketTypeID] = ticket.Quantity
			}

			ticketTypeRepo := collections.TicketType{}
			dbTicketTypes, err := ticketTypeRepo.Find(sessionContext, bson.M{
				"_id":      bson.M{"$in": ticketTypeIDs},
				"event_id": eventIDConvert,
			})

			if err != nil {
				return nil, errors.New("Lỗi khi tìm thông tin vé!")
			}
			if len(dbTicketTypes) != len(requestedTicketsMap) {
				return nil, errors.New("Một hoặc nhiều loại vé không hợp lệ hoặc không thuộc sự kiện này!")
			}

			// Lấy Đăng Ký TRƯỚC ĐÓ & Tạo Map Vé Đã Sở Hữu
			regisEntry := collections.Registration{}
			existingRegs, err := regisEntry.Find(sessionContext, bson.M{
				"event_id":   eventIDConvert,
				"created_by": creatorID,
				"status":     bson.M{"$ne": consts.RegistrationCancelled},
			})
			if err != nil {
				return nil, errors.New("Lỗi khi kiểm tra vé đã đăng ký!")
			}

			totalAlreadyRegistered := 0
			ownedTicketTypeMap := make(map[primitive.ObjectID]int)

			for _, reg := range existingRegs {
				totalAlreadyRegistered += reg.TotalQuantity
				for _, ticket := range reg.Tickets {
					ownedTicketTypeMap[ticket.TicketTypeID] += ticket.Quantity
				}
			}

			// Xử lý Logic Nghiệp Vụ
			var totalPrice int = 0
			var totalNewTickets int = 0
			var ticketEntry = &collections.TicketType{}
			for _, tt := range dbTicketTypes {
				requestedQty := requestedTicketsMap[tt.ID]
				isFreeTicket := (tt.Price == 0)

				if isFreeTicket {
					if requestedQty > 1 {
						return nil, fmt.Errorf("Vé miễn phí '%s' chỉ cho phép đăng ký tối đa 1 vé.", tt.Name)
					}

					//Kiểm tra xem đã SỞ HỮU vé free này CHƯA
					if ownedQty, ok := ownedTicketTypeMap[tt.ID]; ok && ownedQty > 0 {
						return nil, fmt.Errorf("Bạn đã đăng ký vé miễn phí '%s' này rồi.", tt.Name)
					}
				}

				//Kiểm tra Stock
				stockLeft := *tt.Quantity - tt.RegisteredCount
				if requestedQty > stockLeft {
					return nil, fmt.Errorf("Vé '%s' chỉ còn %d vé (bạn yêu cầu %d)", tt.Name, stockLeft, requestedQty)
				}

				// Tính toán tổng
				totalPrice += requestedQty * tt.Price
				totalNewTickets += requestedQty

				// Chuẩn bị lệnh update
				ticketTypeFilter := bson.M{
					"_id": tt.ID,
					"registered_count": bson.M{
						"$lte": *tt.Quantity - requestedQty,
					},
				}

				ticketTypeUpdate := bson.M{
					"$inc": bson.M{"registered_count": requestedQty},
				}

				err = ticketEntry.Update(sessionContext, ticketTypeFilter, ticketTypeUpdate)
				if err != nil {
					if errors.Is(err, mongo.ErrNoDocuments) {
						return nil, fmt.Errorf("Vé '%s' đã hết trong lúc bạn thao tác, vui lòng thử lại.", tt.Name)
					}
					return nil, err
				}
			}
			//Nếu dùng write model
			//var ticketUpdates []mongo.WriteModel
			//
			//for _, tt := range dbTicketTypes {
			//	requestedQty := requestedTicketsMap[tt.ID]
			//	isFreeTicket := (tt.Price == 0)
			//
			//	if isFreeTicket {
			//		if requestedQty > 1 {
			//			return nil, fmt.Errorf("Vé miễn phí '%s' chỉ cho phép đăng ký tối đa 1 vé.", tt.Name)
			//		}
			//		if ownedQty, ok := ownedTicketTypeMap[tt.ID]; ok && ownedQty > 0 {
			//			return nil, fmt.Errorf("Bạn đã đăng ký vé miễn phí '%s' này rồi.", tt.Name)
			//		}
			//	}
			//
			//	stockLeft := *tt.Quantity - tt.RegisteredCount
			//	if requestedQty > stockLeft {
			//		return nil, fmt.Errorf("Vé '%s' chỉ còn %d vé (bạn yêu cầu %d)", tt.Name, stockLeft, requestedQty)
			//	}
			//
			//	// Tính toán tổng
			//	totalPrice += requestedQty * tt.Price
			//	totalNewTickets += requestedQty
			//
			//	update := mongo.NewUpdateOneModel().
			//		SetFilter(bson.M{
			//			"_id": tt.ID,
			//			"registered_count": bson.M{
			//				"$lte": *tt.Quantity - requestedQty,
			//			},
			//		}).
			//		SetUpdate(bson.M{"$inc": bson.M{"registered_count": requestedQty}})
			//
			//	ticketUpdates = append(ticketUpdates, update)
			//}
			//
			//if totalAlreadyRegistered+totalNewTickets > maxTicketPerUser {
			//	return nil, fmt.Errorf("Bạn đã đăng ký %d vé. Bạn chỉ được đăng ký tổng cộng %d vé cho sự kiện này.", totalAlreadyRegistered, maxTicketPerUser)
			//}
			//if len(ticketUpdates) > 0 {
			//	ticketTypeCollection := database.GetDB().Collection("ticket_types")
			//
			//	result, err := ticketTypeCollection.BulkWrite(sessionContext, ticketUpdates)
			//	if err != nil {
			//		return nil, errors.New("Lỗi hệ thống khi cập nhật số lượng vé!")
			//	}
			//
			//	if result.ModifiedCount != int64(len(ticketUpdates)) {
			//		return nil, errors.New("Một vài vé đã hết trong lúc bạn thao tác, vui lòng thử lại.")
			//	}
			//}

			// Kiểm tra giới hạn tổng vé/user
			if totalAlreadyRegistered+totalNewTickets > maxTicketPerUser {
				return nil, fmt.Errorf("Bạn đã đăng ký %d vé. Bạn chỉ được đăng ký tổng cộng %d vé cho sự kiện này.", totalAlreadyRegistered, maxTicketPerUser)
			}

			//Cập nhật số người tham gia
			err = eventEntry.Update(sessionContext,
				bson.M{"_id": eventIDConvert},
				bson.M{"$inc": bson.M{"number_of_participants": totalNewTickets}},
			)

			if err != nil {
				return nil, errors.New("Lỗi hệ thống khi cập nhật sự kiện!")
			}

			//TẠO bản ghi Registration MỚI
			now := time.Now()
			newRegistration := collections.Registration{
				EventID:       eventIDConvert,
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
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống (kết quả transaction không hợp lệ)!", nil)
		return
	}

	utils.ResponseSuccess(c, http.StatusCreated, "Đăng ký thành công!", newRegistration, nil)
}

func GenerateTickets(c *gin.Context) {
	var (
		regisEntry      = &collections.Registration{}
		ticketTypeEntry = &collections.TicketType{}
		eventEntry      = &collections.Event{}
		ticketEntry     = &collections.Ticket{}
		accountEntry    = &collections.Account{}

		existingTickets = collections.Tickets{}
		err             error
		newTickets      = collections.Tickets{}
		embededMedia    = [][]byte{}
		ticketypeMap    = make(map[primitive.ObjectID]collections.TicketType)
	)

	regisIDStr := c.Param("id")
	regisID, err := primitive.ObjectIDFromHex(regisIDStr)
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống (ID không hợp lệ)!", err.Error())
		return
	}

	ctx := c.Request.Context()

	err = regisEntry.First(ctx, bson.M{
		"_id": regisID,
		"deleted_at": bson.M{
			"$exists": false,
		},
		"status": consts.RegistrationPaid,
	})

	switch err {
	case nil:
		if regisEntry.TicketEmailSentAt != nil {
			log.Printf("INFO: Email vé cho regis %s đã được gửi (Check 1). Bỏ qua.", regisIDStr)
			utils.ResponseSuccess(c, http.StatusOK, "Email vé cho đăng ký này đã được gửi trước đó.", nil, nil)
			return
		}
		if regisEntry.InvoiceID == nil {
			utils.ResponseError(c, http.StatusBadRequest, "Đăng ký này thiếu InvoiceID.", "Missing InvoiceID")
			return
		}
	case mongo.ErrNoDocuments:
		utils.ResponseError(c, http.StatusBadRequest, "", fmt.Errorf("Không tìm thấy đăng ký hoặc đăng ký chưa thanh toán : %w", mongo.ErrNoDocuments).Error())
		return
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống (Tìm đăng ký)!", err.Error())
		return
	}

	err = eventEntry.First(ctx, bson.M{
		"_id": regisEntry.EventID,
		"deleted_at": bson.M{
			"$exists": false,
		},
		"active": true,
	})

	switch {
	case err == nil:
	case errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusBadRequest, "", fmt.Errorf("Không tìm thấy event : %w", mongo.ErrNoDocuments).Error())
		return
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	err = accountEntry.First(bson.M{
		"_id":        regisEntry.CreatedBy,
		"deleted_at": nil,
	})
	switch {
	case err == nil:
	case errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusBadRequest, "", fmt.Errorf("Không tìm thấy tài khoản đăng ký : %w", mongo.ErrNoDocuments).Error())
		return
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	var ticketTypeIDS []primitive.ObjectID
	for _, ticket := range regisEntry.Tickets {
		ticketTypeIDS = append(ticketTypeIDS, ticket.TicketTypeID)
	}

	if len(ticketTypeIDS) == 0 {
		utils.ResponseError(c, http.StatusBadRequest, "", "Đăng ký không có vé.")
		return
	}

	ticketTypes, err := ticketTypeEntry.Find(ctx, bson.M{
		"_id": bson.M{
			"$in": ticketTypeIDS,
		},
	})
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống (ticket types)!", err.Error())
		return
	}
	for _, ticketType := range ticketTypes {
		ticketypeMap[ticketType.ID] = ticketType
	}

	existingTickets, err = ticketEntry.Find(ctx, bson.M{"invoice_id": *regisEntry.InvoiceID})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống khi kiểm tra vé!", err.Error())
		return
	}

	if len(existingTickets) > 0 {
		newTickets = existingTickets

		for _, ticket := range newTickets {
			qrLink := configs.GetServerDomain() + "/ticket/checkin/token?" + ticket.QRCodeData
			qrCodePng, err := qrcode.Encode(qrLink, qrcode.Medium, 256)
			if err != nil {
				utils.ResponseError(c, http.StatusInternalServerError, "Lỗi tái tạo QR code!", err.Error())
				return
			}
			embededMedia = append(embededMedia, qrCodePng)
		}

	} else {
		client := database.GetDB().Client()
		session, err := client.StartSession()
		if err != nil {
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống (session)!", err.Error())
			return
		}
		defer session.EndSession(ctx)

		_, err = session.WithTransaction(ctx,
			func(sessionContext mongo.SessionContext) (interface{}, error) {
				creationTime := time.Now()

				for _, ticket := range regisEntry.Tickets {
					for i := 0; i < ticket.Quantity; i++ {
						newUUID, err := uuid.NewUUID()
						if err != nil {
							return nil, err
						}

						createdTicket := collections.Ticket{
							Status:       consts.TicketStatusConfirmed,
							QRCodeData:   "TICKET-" + newUUID.String(),
							InvoiceID:    *regisEntry.InvoiceID,
							TicketTypeID: ticket.TicketTypeID,
							EventID:      eventEntry.ID,
							CreatedAt:    creationTime,
							CreatedBy:    regisEntry.CreatedBy,
							UpdatedAt:    creationTime,
							UpdatedBy:    regisEntry.CreatedBy,
						}
						newTickets = append(newTickets, createdTicket)

						//Tạo qr code
						qrLink := configs.GetServerDomain() + "/ticket/checkin/token?" + createdTicket.QRCodeData
						qrCodePng, err := qrcode.Encode(qrLink, qrcode.Medium, 256)
						if err != nil {
							return nil, fmt.Errorf("Lỗi tạo mã QR: %w", err)
						}
						embededMedia = append(embededMedia, qrCodePng)
					}
				}

				err = ticketEntry.CreateMany(sessionContext, newTickets)
				if err != nil {
					return nil, err
				}
				return nil, nil
			})

		// Xử lý lỗi Transaction
		if err != nil {
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi khi lưu vé vào hệ thống!", err.Error())
			return
		}
	}

	//Tạo nội dung mail
	eventName := eventEntry.Name
	eventLocation := eventEntry.EventLocation.Name + ", " + eventEntry.EventLocation.Address
	eventTime := eventEntry.EventTime.StartDate.Format("02/01/2006") + "-" + eventEntry.EventTime.EndDate.Format("02/01/2006") + " lúc: " + eventEntry.EventTime.StartTime

	recipientEmail := accountEntry.Email
	recipientName := accountEntry.Name

	var emailBody strings.Builder
	emailBody.WriteString("<html><body style='font-family: Arial, sans-serif; line-height: 1.6; margin: 0; padding: 0;'>")
	emailBody.WriteString("<div style='max-width: 640px; margin: 20px auto; padding: 20px; border: 1px solid #ddd; border-radius: 8px;'>")

	emailBody.WriteString(fmt.Sprintf("<h2>Xin chào %s,</h2>", recipientName))
	emailBody.WriteString("<p>Cảm ơn bạn đã đăng ký tham gia sự kiện của chúng tôi. Dưới đây là thông tin sự kiện và vé của bạn.</p>")

	// --- Thông tin sự kiện ---
	emailBody.WriteString("<h3 style='border-bottom: 2px solid #eee; padding-bottom: 5px;'>Thông tin sự kiện</h3>")
	emailBody.WriteString(fmt.Sprintf("<p style='margin: 5px 0;'><strong>Sự kiện:</strong> %s</p>", eventName))
	emailBody.WriteString(fmt.Sprintf("<p style='margin: 5px 0;'><strong>Thời gian:</strong> %s</p>", eventTime))
	emailBody.WriteString(fmt.Sprintf("<p style='margin: 5px 0;'><strong>Địa điểm:</strong> %s</p>", eventLocation))
	emailBody.WriteString("<br>")

	// --- Danh sách vé ---
	emailBody.WriteString("<h3 style='border-bottom: 2px solid #eee; padding-bottom: 5px;'>Chi tiết vé</h3>")
	emailBody.WriteString("<p>Vui lòng đưa mã QR bên dưới cho ban tổ chức tại cổng check-in.</p>")

	embeddedFiles := make(map[string][]byte)

	for i, ticket := range newTickets {
		qrCodePng := embededMedia[i]
		ticketTypeName := ticketypeMap[ticket.TicketTypeID].Name
		ticketPrice := ticketypeMap[ticket.TicketTypeID].Price

		cid := fmt.Sprintf("qrcode%d.png", i)
		embeddedFiles[cid] = qrCodePng

		emailBody.WriteString(
			"<div style='border: 1px solid #ddd; border-radius: 8px; padding: 16px; margin-bottom: 20px;'>")

		emailBody.WriteString(
			"<table border='0' cellpadding='0' cellspacing='0' width='100%'>")
		emailBody.WriteString("<tr>")

		// Cột 1: QR Code
		emailBody.WriteString(
			fmt.Sprintf(
				"<td width='140' style='width: 140px; padding-right: 15px; vertical-align: top;'>"+
					"<img src='cid:%s' alt='Mã QR' width='120' height='120' style='width: 120px; height: 120px; border: 1px solid #eee;' />"+
					"</td>",
				cid,
			),
		)

		// Cột 2: Thông tin vé
		emailBody.WriteString(
			"<td style='vertical-align: top; font-size: 14px; line-height: 1.7;'>")

		// Tên loại vé
		emailBody.WriteString(
			fmt.Sprintf(
				"<strong style='font-size: 16px; color: #333;'>%s</strong><br>",
				ticketTypeName,
			))

		// Giá vé
		emailBody.WriteString(
			fmt.Sprintf(
				"Giá vé: %d VNĐ<br>", // Sử dụng %d cho int theo code gốc của bạn
				ticketPrice,
			))

		// Ngày đăng ký vé
		emailBody.WriteString(
			fmt.Sprintf(
				"Ngày đăng ký: %s<br>",
				ticket.CreatedAt.In(time.FixedZone("ICT", 7*60*60)).Format("15:04 02/01/2006"), // Format giờ VN
			))

		// Mã vé
		emailBody.WriteString(
			fmt.Sprintf(
				"Mã vé: <code style='font-size: 13px; background-color: #f4f4f4; padding: 2px 5px; border-radius: 4px;'>%s</code>",
				ticket.QRCodeData,
			))

		emailBody.WriteString("</td>")

		emailBody.WriteString("</tr>")
		emailBody.WriteString("</table>")
		emailBody.WriteString("</div>")
	}

	emailBody.WriteString("<hr style='border: 0; border-top: 1px solid #eee; margin-top: 20px;'>")
	emailBody.WriteString("<p style='font-size: 12px; color: #777;'>Trân trọng,<br>Đội ngũ EventHunting</p>")
	emailBody.WriteString("</div>")
	emailBody.WriteString("</body></html>")

	emailSubject := fmt.Sprintf("Vé tham dự sự kiện: %s", eventName)
	emailContent := emailBody.String()

	// Gửi email trong một goroutine riêng
	go func(emailTo, subject, body string, files map[string][]byte, eventNameForLog string) {
		emailService := utils.NewEmailService()
		payload := utils.EmailPayload{
			To:             []string{emailTo},
			Subject:        subject,
			HTMLBody:       body,
			EmbeddedImages: files,
		}

		err := emailService.SendEmail(payload)
		if err != nil {
			log.Printf("ERROR: Đã set cờ nhưng GỬI EMAIL THẤT BẠI cho %s (sự kiện: %s): %v", emailTo, eventNameForLog, err)
		} else {
			now := time.Now()
			err = regisEntry.Update(ctx,
				bson.M{"_id": regisEntry.ID},
				bson.M{"$set": bson.M{"ticket_email_sent_at": now}},
			)

			if err != nil {
				log.Printf("ERROR: Không thể set cờ email_sent cho regis %s: %v", regisID.Hex(), err)
			} else {
				log.Printf("INFO: Đã gửi email vé thành công cho %s (sự kiện: %s)", emailTo, eventNameForLog)
			}
		}
	}(recipientEmail, emailSubject, emailContent, embeddedFiles, eventName)

	utils.ResponseSuccess(c, http.StatusCreated, "Tạo vé thành công! Vui lòng kiểm tra email.", nil, nil)
}

func GenerateTickets1(c *gin.Context) {
	var (
		regisEntry      = &collections.Registration{}
		ticketTypeEntry = &collections.TicketType{}
		eventEntry      = &collections.Event{}
		ticketEntry     = &collections.Ticket{}
		accountEntry    = &collections.Account{}

		existingTickets = collections.Tickets{}
		err             error
		newTickets      = collections.Tickets{}
		ticketypeMap    = make(map[primitive.ObjectID]collections.TicketType)
	)

	regisIDStr := c.Param("id")
	regisID, err := primitive.ObjectIDFromHex(regisIDStr)
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống (ID không hợp lệ)!", err.Error())
		return
	}

	ctx := c.Request.Context()

	err = regisEntry.First(ctx, bson.M{
		"_id": regisID,
		"deleted_at": bson.M{
			"$exists": false,
		},
		"status": consts.RegistrationPaid,
	})

	switch {
	case err == nil:
	case errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusBadRequest, "", fmt.Errorf("Không tìm thấy đăng ký hoặc đăng ký chưa thanh toán : %w", mongo.ErrNoDocuments).Error())
		return
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống (Tìm đăng ký)!", err.Error())
		return
	}

	if regisEntry.TicketEmailSentAt != nil {
		log.Printf("INFO: Email vé cho regis %s đã được gửi (Check 1). Bỏ qua.", regisIDStr)
		utils.ResponseSuccess(c, http.StatusOK, "Email vé cho đăng ký này đã được gửi trước đó.", nil, nil)
		return
	}

	if regisEntry.InvoiceID == nil {
		utils.ResponseError(c, http.StatusBadRequest, "Đăng ký này thiếu InvoiceID.", "Missing InvoiceID")
		return
	}

	err = eventEntry.First(ctx, bson.M{
		"_id": regisEntry.EventID,
		"deleted_at": bson.M{
			"$exists": false,
		},
		"active": true,
	})

	switch {
	case err == nil:
	case errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusBadRequest, "", fmt.Errorf("Không tìm thấy event : %w", mongo.ErrNoDocuments).Error())
		return
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	err = accountEntry.First(bson.M{
		"_id": regisEntry.CreatedBy,
	})
	switch {
	case err == nil:
	case errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusBadRequest, "", fmt.Errorf("Không tìm thấy tài khoản đăng ký : %w", mongo.ErrNoDocuments).Error())
		return
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	var ticketTypeIDS []primitive.ObjectID
	for _, ticket := range regisEntry.Tickets {
		ticketTypeIDS = append(ticketTypeIDS, ticket.TicketTypeID)
	}

	if len(ticketTypeIDS) == 0 {
		utils.ResponseError(c, http.StatusBadRequest, "", "Đăng ký không có vé.")
		return
	}

	ticketTypes, err := ticketTypeEntry.Find(ctx, bson.M{
		"_id": bson.M{
			"$in": ticketTypeIDS,
		},
	})
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống (ticket types)!", err.Error())
		return
	}
	for _, ticketType := range ticketTypes {
		ticketypeMap[ticketType.ID] = ticketType
	}

	existingTickets, err = ticketEntry.Find(ctx, bson.M{"invoice_id": *regisEntry.InvoiceID})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống khi kiểm tra vé!", err.Error())
		return
	}

	if len(existingTickets) > 0 {
		//newTickets = existingTickets
	} else {
		client := database.GetDB().Client()
		session, err := client.StartSession()
		if err != nil {
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống (session)!", err.Error())
			return
		}
		defer session.EndSession(ctx)

		_, err = session.WithTransaction(ctx,
			func(sessionContext mongo.SessionContext) (interface{}, error) {
				creationTime := time.Now()

				for _, ticket := range regisEntry.Tickets {
					for i := 0; i < ticket.Quantity; i++ {
						newUUID, err := uuid.NewUUID()
						if err != nil {
							return nil, err
						}

						createdTicket := collections.Ticket{
							Status:       consts.TicketStatusConfirmed,
							QRCodeData:   "TICKET-" + newUUID.String(),
							InvoiceID:    *regisEntry.InvoiceID,
							TicketTypeID: ticket.TicketTypeID,
							EventID:      eventEntry.ID,
							CreatedAt:    creationTime,
							CreatedBy:    regisEntry.CreatedBy,
							UpdatedAt:    creationTime,
							UpdatedBy:    regisEntry.CreatedBy,
						}
						newTickets = append(newTickets, createdTicket)
					}
				}

				err = ticketEntry.CreateMany(sessionContext, newTickets)
				if err != nil {
					return nil, err
				}
				return nil, nil
			})

		// Xử lý lỗi Transaction
		if err != nil {
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi khi lưu vé vào hệ thống!", err.Error())
			return
		}
	}
	data := map[string]interface{}{
		"registration_id": regisIDStr,
	}

	jobPayload := map[string]interface{}{
		"task_name":   "send_ticket_email",
		"retry_count": 0,
		"data":        data,
	}
	jobData, err := json.Marshal(jobPayload)
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi tạo job (marshal)!", err.Error())
		return
	}

	redisClient := database.GetRedisClient().Client
	err = redisClient.RPush(ctx, "transactional_email_queue", jobData).Err()
	if err != nil {
		log.Printf("ERROR: Không thể đẩy job vào Redis cho regis %s: %v", regisIDStr, err)
		utils.ResponseError(c, http.StatusServiceUnavailable, "Hệ thống đang bận, vui lòng thử lại sau.", err.Error())
		return
	}

	err = regisEntry.Update(ctx,
		bson.M{"_id": regisEntry.ID},
		bson.M{"$set": bson.M{"ticket_job_queued_at": time.Now()}},
	)
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi cập nhật trạng thái job.", err.Error())
		return
	}

	log.Printf("INFO: Đã xếp hàng job thành công cho regis %s.", regisIDStr)
	utils.ResponseSuccess(c, http.StatusAccepted, "Yêu cầu tạo vé đã được tiếp nhận! Vé sẽ được gửi đến email của bạn trong vài phút.", nil, nil)
}
