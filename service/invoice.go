package service

import (
	"EventHunting/collections"
	"EventHunting/utils"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Tạo hóa đơn
func CreateInvoiceForRegistration(regisID primitive.ObjectID, vnpTransactionNo string, payDate time.Time) (*collections.Invoice, error) {
	var (
		regisEntry   = &collections.Registration{}
		eventEntry   = &collections.Event{}
		accountEntry = &collections.Account{}
		err          error
	)

	// Lấy thông tin Registration
	err = regisEntry.First(nil, bson.M{"_id": regisID})
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy registration: %w", err)
	}

	// Lấy thông tin Event
	err = eventEntry.First(nil, bson.M{"_id": regisEntry.EventID})
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy event: %w", err)
	}

	// Lấy thông tin Account
	err = accountEntry.First(bson.M{"_id": regisEntry.CreatedBy})
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy account: %w", err)
	}

	// Lấy chi tiết các loại vé để tạo Line Items
	ticketTypeMap, err := fetchTicketTypes(regisEntry)
	if err != nil {
		return nil, fmt.Errorf("lỗi lấy loại vé: %w", err)
	}

	// Build danh sách hàng hóa (Line Items)
	var lstItems []collections.InvoiceLstItem
	for _, t := range regisEntry.Tickets {
		ticketType, exists := ticketTypeMap[t.TicketTypeID]
		desc := "Vé sự kiện"
		unitPrice := 0

		if exists {
			desc = fmt.Sprintf("Vé %s - %s", ticketType.Name, eventEntry.Name)
			unitPrice = ticketType.Price
		}

		item := collections.InvoiceLstItem{
			ItemID:      t.TicketTypeID,
			Description: desc,
			Quantity:    t.Quantity,
			UnitPrice:   unitPrice,
			TotalAmount: unitPrice * t.Quantity,
		}
		lstItems = append(lstItems, item)
	}

	// 6. Tạo struct Invoice
	newInvoice := &collections.Invoice{
		ID:             primitive.NewObjectID(),
		InvoiceNumber:  utils.GenerateInvoiceNumber(),
		RegistrationID: regisEntry.ID,
		Status:         "Completed",
		PaymentDetails: collections.InvoicePaymentDetails{
			Method:          "VNPAY",
			TransactionCode: vnpTransactionNo,
			PaidAt:          payDate,
		},
		CustomerDetails: collections.InvoiceCustomerDetails{
			Name:  accountEntry.Name,
			Email: accountEntry.Email,
			Phone: accountEntry.Phone,
		},
		EventDetails: collections.InvoiceEventDetails{
			EventID:   eventEntry.ID,
			Name:      eventEntry.Name,
			StartDate: eventEntry.EventTime.StartDate,
			EndDate:   eventEntry.EventTime.EndDate,
		},
		LineItems: lstItems,
		CreatedAt: time.Now(),
		CreatedBy: regisEntry.CreatedBy,
		UpdatedAt: time.Now(),
		UpdatedBy: regisEntry.CreatedBy,
	}
	return newInvoice, nil
}
