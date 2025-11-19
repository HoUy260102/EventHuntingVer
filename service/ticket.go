package service

import (
	"EventHunting/collections"
	"EventHunting/consts"
	"EventHunting/database"
	"EventHunting/utils"
	"EventHunting/view"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func ProcessTicketAndEmail(regisID primitive.ObjectID) error {
	var (
		regisEntry   = &collections.Registration{}
		eventEntry   = &collections.Event{}
		accountEntry = &collections.Account{}
		err          error
	)

	// Kiểm tra Regis (Phải là trạng thái PAID)
	regisFilter := bson.M{
		"_id":    regisID,
		"status": consts.RegistrationPaid,
	}
	err = regisEntry.First(nil, utils.GetFilter(regisFilter))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return fmt.Errorf("%w: %v", consts.ErrFatalDataNotFound, err)
		}
		return err
	}

	// Nếu email đã gửi thì không gửi nữa
	if regisEntry.TicketEmailSentAt != nil {
		log.Printf("INFO: Email vé cho đăng ký %s đã được gửi trước đó. Bỏ qua.", regisEntry.ID.Hex())
		return nil
	}

	// Lấy thông tin Event
	eventFilter := bson.M{
		"_id":    regisEntry.EventID,
		"active": true,
	}
	err = eventEntry.First(nil, utils.GetFilter(eventFilter))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return fmt.Errorf("%w: Event ID %s", consts.ErrFatalDataNotFound, regisEntry.EventID.Hex())
		}
		return err
	}

	// Lấy thông tin Account
	accountFilter := bson.M{
		"_id": regisEntry.CreatedBy,
	}
	err = accountEntry.First(utils.GetFilter(accountFilter))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return fmt.Errorf("%w: Account ID %s", consts.ErrFatalDataNotFound, regisEntry.CreatedBy.Hex())
		}
		return err
	}

	// Lấy thông tin loại vé
	ticketTypeMap, err := fetchTicketTypes(regisEntry)
	if err != nil {
		return err
	}

	// Sinh vé hoặc Lấy vé đã có (Quan trọng: Transaction)
	newTickets, err := getOrCreateTickets(regisEntry, eventEntry)
	if err != nil {
		return fmt.Errorf("lỗi sinh vé: %w", err)
	}

	// Build Email Content
	subject, htmlBody, embeddedFiles, err := view.BuildTicketEmail(eventEntry, accountEntry, newTickets, ticketTypeMap)
	if err != nil {
		return fmt.Errorf("lỗi build email: %w", err)
	}

	// Gửi Email
	emailService := utils.NewEmailService()
	payload := utils.EmailPayload{
		Subject:        subject,
		To:             []string{accountEntry.Email},
		HTMLBody:       htmlBody,
		EmbeddedImages: embeddedFiles,
	}

	if err := emailService.SendEmail(payload); err != nil {
		return fmt.Errorf("lỗi SMTP gửi mail: %w", err)
	}

	// Cập nhật cờ vào DB
	now := time.Now()
	err = regisEntry.Update(nil,
		bson.M{"_id": regisEntry.ID},
		bson.M{"$set": bson.M{"ticket_email_sent_at": now}},
	)
	if err != nil {
		// Mail đã gửi nhưng update DB lỗi.
		// Trường hợp này worker sẽ retry -> Gửi lại mail lần 2 -> Khách nhận 2 mail.
		return fmt.Errorf("lỗi update ticket_email_sent_at: %w", err)
	}
	log.Printf("SUCCESS: Đã hoàn tất xử lý đơn hàng %s", regisEntry.ID.Hex())
	return nil
}

func fetchTicketTypes(regisEntry *collections.Registration) (map[primitive.ObjectID]collections.TicketType, error) {
	var ticketTypeIDS []primitive.ObjectID
	for _, ticket := range regisEntry.Tickets {
		ticketTypeIDS = append(ticketTypeIDS, ticket.TicketTypeID)
	}

	if len(ticketTypeIDS) == 0 {
		return nil, fmt.Errorf("đăng ký không có vé")
	}

	ticketTypeEntry := &collections.TicketType{}
	ticketTypes, err := ticketTypeEntry.Find(nil, bson.M{
		"_id": bson.M{"$in": ticketTypeIDS},
	})
	if err != nil {
		return nil, fmt.Errorf("lỗi hệ thống (ticket types): %w", err)
	}

	ticketTypeMap := make(map[primitive.ObjectID]collections.TicketType)
	for _, ticketType := range ticketTypes {
		ticketTypeMap[ticketType.ID] = ticketType
	}

	return ticketTypeMap, nil
}

func getOrCreateTickets(regisEntry *collections.Registration, eventEntry *collections.Event) (collections.Tickets, error) {
	var (
		ticketEntry = &collections.Ticket{}
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	)
	defer cancel()

	//Kiểm tra vé đã tồn tại chưa
	existingTickets, err := ticketEntry.Find(ctx, bson.M{"regis_id": regisEntry.ID, "status": consts.TicketStatusConfirmed})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("lỗi hệ thống khi kiểm tra vé: %w", err)
	}

	if len(existingTickets) > 0 {
		return existingTickets, nil
	}

	var newTickets collections.Tickets
	client := database.GetDB().Client()
	session, err := client.StartSession()
	if err != nil {
		return nil, fmt.Errorf("lỗi hệ thống (session): %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessionContext mongo.SessionContext) (interface{}, error) {
		creationTime := time.Now()
		newTickets = collections.Tickets{}

		for _, ticket := range regisEntry.Tickets {
			for i := 0; i < ticket.Quantity; i++ {
				newUUID, err := uuid.NewUUID()
				if err != nil {
					return nil, fmt.Errorf("lỗi tạo UUID: %w", err)
				}

				createdTicket := collections.Ticket{
					Status:     consts.TicketStatusConfirmed,
					QRCodeData: "TICKET-" + newUUID.String(),
					//InvoiceID:    *regisEntry.InvoiceID,
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

		if len(newTickets) == 0 {
			return nil, fmt.Errorf("không có vé nào được tạo (Số lượng vé = 0?)")
		}

		err = ticketEntry.CreateMany(sessionContext, newTickets)
		if err != nil {
			return nil, fmt.Errorf("lỗi CSDL khi tạo vé: %w", err)
		}
		return nil, nil
	})

	if err != nil {
		return nil, fmt.Errorf("lỗi transaction khi lưu vé: %w", err)
	}

	return newTickets, nil
}

//func GenerateAndSendTickets(regisID primitive.ObjectID) error {
//	var (
//		regisEntry   = &collections.Registration{}
//		eventEntry   = &collections.Event{}
//		accountEntry = &collections.Account{}
//		err          error
//	)
//
//	//Kiểm tra regis hợp lệ
//	err = regisEntry.First(nil, bson.M{
//		"_id":        regisID,
//		"deleted_at": bson.M{"$exists": false},
//		"status":     consts.RegistrationPaid,
//	})
//	if err != nil {
//		if errors.Is(err, mongo.ErrNoDocuments) {
//			return consts.ErrRegistrationNotFound
//		}
//		return fmt.Errorf("lỗi hệ thống khi tìm đăng ký: %w", err)
//	}
//
//	//Kiểm tra event có hợp lệ
//	err = eventEntry.First(nil, bson.M{
//		"_id":        regisEntry.EventID,
//		"deleted_at": bson.M{"$exists": false},
//		"active":     true,
//	})
//	if err != nil {
//		if errors.Is(err, mongo.ErrNoDocuments) {
//			return consts.ErrEventNotFound
//		}
//		return fmt.Errorf("lỗi hệ thống khi tìm sự kiện: %w", err)
//	}
//
//	//Kiểm tra tài khoản có hợp lệ
//	err = accountEntry.First(bson.M{
//		"_id":        regisEntry.CreatedBy,
//		"deleted_at": bson.M{"$exists": false},
//	})
//	if err != nil {
//		if errors.Is(err, mongo.ErrNoDocuments) {
//			return consts.ErrAccountNotFound
//		}
//		return fmt.Errorf("lỗi hệ thống khi tìm tài khoản: %w", err)
//	}
//
//	//Kiểm tra đã gửi mail chưa
//	if regisEntry.TicketEmailSentAt != nil {
//		log.Printf("INFO: Email vé cho đăng ký %s đã được gửi trước đó. Bỏ qua.", regisEntry.ID.Hex())
//		return nil
//	}
//
//	//Lấy thông tin loại vé
//	ticketTypeMap, err := fetchTicketTypes(regisEntry)
//	if err != nil {
//		return fmt.Errorf("%w: %v", consts.ErrTicketTypeFetch, err)
//	}
//
//	//Chuẩn bị vé
//	newTickets, err := getOrCreateTickets(regisEntry, eventEntry)
//	if err != nil {
//		return fmt.Errorf("%w: %v", consts.ErrTicketProcessing, err)
//	}
//
//	//Chuẩn bị gửi file
//	subject, htmlBody, embeddedFiles, err := view.BuildTicketEmail(eventEntry, accountEntry, newTickets, ticketTypeMap)
//	if err != nil {
//		return fmt.Errorf("%w: %v", consts.ErrEmailBuild, err)
//	}
//
//	// Gửi email và cập nhật cờ đã gửi
//	go func(subject string, to []string, htmlBody string, embeddedFiles map[string][]byte, regisID *collections.Registration) {
//		emailService := utils.NewEmailService()
//		payload := utils.EmailPayload{
//			Subject:        subject,
//			To:             to,
//			HTMLBody:       htmlBody,
//			EmbeddedImages: embeddedFiles,
//		}
//		mailErr := emailService.SendEmail(payload)
//		if mailErr == nil {
//			now := time.Now()
//			err = regisEntry.Update(nil,
//				bson.M{"_id": regisEntry.ID},
//				bson.M{"$set": bson.M{"ticket_email_sent_at": now}},
//			)
//			if err != nil {
//				log.Printf("ERROR: Không thể set cờ email_sent cho regis %s: %v", regisEntry.ID.Hex(), err)
//			} else {
//				log.Printf("INFO: Đã gửi email vé thành công cho %s (sự kiện: %s)")
//			}
//		} else {
//			var (
//				redisClient = database.GetRedisClient().Client
//				ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
//			)
//			defer cancel()
//
//			jobPayload := map[string]interface{}{
//				"type": "ticket_mail",
//				"data": bson.M{
//					"registration_id": regisEntry.ID.Hex(),
//				},
//				"retry_count": 0,
//			}
//			payloadBytes, _ := json.Marshal(jobPayload)
//			queueName := "transactional_email_queue"
//
//			err := redisClient.LPush(ctx, queueName, payloadBytes).Err()
//			if err != nil {
//				log.Printf("CRITICAL: Không thể LPUSH job vào Redis queue: %v", err)
//			} else {
//				log.Printf("INFO: Đã thêm email vé cho regis %s vào Redis queue.", regisEntry.ID.Hex())
//			}
//		}
//	}(subject, []string{accountEntry.Email}, htmlBody, embeddedFiles, regisEntry)
//	return nil
//}
//
//func RetrySendTicket(regisID primitive.ObjectID) error {
//	var (
//		regisEntry   = &collections.Registration{}
//		eventEntry   = &collections.Event{}
//		accountEntry = &collections.Account{}
//		err          error
//	)
//
//	//Kiểm tra regis hợp lệ
//	err = regisEntry.First(nil, bson.M{
//		"_id":        regisID,
//		"deleted_at": bson.M{"$exists": false},
//		"status":     consts.RegistrationPaid,
//	})
//	if err != nil {
//		if errors.Is(err, mongo.ErrNoDocuments) {
//			// Nếu không tìm thấy, job này bị hỏng -> trả về nil để worker xóa job
//			log.Printf("INFO: (Worker) Không tìm thấy regis %s. Hủy job.", regisID.Hex())
//			return nil
//		}
//		return fmt.Errorf("lỗi hệ thống khi tìm đăng ký: %w", err) // Lỗi DB, worker nên thử lại
//	}
//
//	//Kiểm tra event có hợp lệ
//	err = eventEntry.First(nil, bson.M{
//		"_id":        regisEntry.EventID,
//		"deleted_at": bson.M{"$exists": false},
//		"active":     true,
//	})
//	if err != nil {
//		if errors.Is(err, mongo.ErrNoDocuments) {
//			log.Printf("INFO: (Worker) Không tìm thấy event %s (của regis %s). Hủy job.", regisEntry.EventID.Hex(), regisID.Hex())
//			return nil // Hủy job
//		}
//		return fmt.Errorf("lỗi hệ thống khi tìm sự kiện: %w", err)
//	}
//
//	//Kiểm tra tài khoản có hợp lệ
//	err = accountEntry.First(bson.M{
//		"_id":        regisEntry.CreatedBy,
//		"deleted_at": bson.M{"$exists": false},
//	})
//	if err != nil {
//		if errors.Is(err, mongo.ErrNoDocuments) {
//			log.Printf("INFO: (Worker) Không tìm thấy account %s (của regis %s). Hủy job.", regisEntry.CreatedBy.Hex(), regisID.Hex())
//			return nil
//		}
//		return fmt.Errorf("lỗi hệ thống khi tìm tài khoản: %w", err)
//	}
//
//	//Kiểm tra đã gửi mail chưa
//	if regisEntry.TicketEmailSentAt != nil {
//		log.Printf("INFO: (Worker) Email vé cho %s đã được gửi (có thể do job khác chạy). Bỏ qua.", regisEntry.ID.Hex())
//		return nil
//	}
//
//	ticketTypeMap, err := fetchTicketTypes(regisEntry)
//	if err != nil {
//		return fmt.Errorf("%w: %v", consts.ErrTicketTypeFetch, err)
//	}
//
//	//Chuẩn bị vé
//	newTickets, err := getOrCreateTickets(regisEntry, eventEntry)
//	if err != nil {
//		return fmt.Errorf("%w: %v", consts.ErrTicketProcessing, err)
//	}
//
//	subject, htmlBody, embeddedFiles, err := view.BuildTicketEmail(eventEntry, accountEntry, newTickets, ticketTypeMap)
//	if err != nil {
//		return fmt.Errorf("%w: %v", consts.ErrEmailBuild, err)
//	}
//
//	emailService := utils.NewEmailService()
//	payload := utils.EmailPayload{
//		Subject:        subject,
//		To:             []string{accountEntry.Email},
//		HTMLBody:       htmlBody,
//		EmbeddedImages: embeddedFiles,
//	}
//
//	log.Printf("INFO: (Worker) Đang gửi lại email vé cho %s...", regisEntry.ID.Hex())
//	mailErr := emailService.SendEmail(payload)
//
//	if mailErr != nil {
//		log.Printf("WARN: (Worker) Gửi lại thất bại cho %s: %v. Báo lỗi để retry...", regisEntry.ID.Hex(), mailErr)
//		return mailErr
//	}
//
//	log.Printf("INFO: (Worker) Gửi mail thành công cho %s. Đang cập nhật cờ...", regisEntry.ID.Hex())
//	now := time.Now()
//	err = regisEntry.Update(nil,
//		bson.M{"_id": regisEntry.ID},
//		bson.M{"$set": bson.M{"ticket_email_sent_at": now}},
//	)
//	if err != nil {
//		log.Printf("ERROR: (Worker) Gửi mail OK nhưng không thể set cờ cho %s: %v", regisEntry.ID.Hex(), err)
//		return err
//	}
//
//	log.Printf("INFO: (Worker) Gửi lại email vé thành công cho %s.", regisEntry.ID.Hex())
//	return nil // Thành công
//}
