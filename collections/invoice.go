package collections

import (
	"EventHunting/database"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type InvoicePaymentDetails struct {
	Method          string    `bson:"method" json:"method"`
	TransactionCode string    `bson:"transaction_code" json:"transaction_code"`
	PaidAt          time.Time `bson:"paid_at" json:"paid_at"`
}

type InvoiceCustomerDetails struct {
	Name  string `bson:"name" json:"name"`
	Email string `bson:"email" json:"email"`
	Phone string `bson:"phone,omitempty" json:"phone,omitempty"`
}

type InvoiceLstItem struct {
	ItemID      primitive.ObjectID `bson:"item_id" json:"item_id"`
	Description string             `bson:"description" json:"description"` // Mô tả ("Vé VIP - Sự kiện X")
	Quantity    int                `bson:"quantity" json:"quantity"`
	UnitPrice   int                `bson:"unit_price" json:"unit_price"`
	TotalAmount int                `bson:"total_amount" json:"total_amount"`
}

type InvoiceEventDetails struct {
	EventID   primitive.ObjectID `bson:"event_id" json:"event_id"`
	Name      string             `bson:"name" json:"name"`
	StartDate time.Time          `bson:"start_date" json:"start_date"`
	EndDate   time.Time          `bson:"end_date" json:"end_date"`
}

type Invoice struct {
	ID              primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	InvoiceNumber   string                 `bson:"invoice_number" json:"invoice_number"`
	RegistrationID  primitive.ObjectID     `bson:"registration_id" json:"registration_id"`
	Status          string                 `bson:"status" json:"status"` // "Completed", "Refunded"
	PaymentDetails  InvoicePaymentDetails  `bson:"payment_details" json:"payment_details"`
	CustomerDetails InvoiceCustomerDetails `bson:"customer_details" json:"customer_details"`
	LineItems       []InvoiceLstItem       `bson:"line_items" json:"line_items"`
	EventDetails    InvoiceEventDetails    `bson:"event_details" json:"event_details"`

	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	CreatedBy primitive.ObjectID `bson:"created_by" json:"created_by"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
	UpdatedBy primitive.ObjectID `bson:"updated_by" json:"updated_by"`
}

type Invoices []Invoice

func (u *Invoice) getCollectionName() string {
	return "invoices"
}

func (u *Invoice) Create(ctx context.Context) error {
	var (
		db  = database.GetDB()
		err error
	)
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	// TODO: Thêm log
	if u.ID.IsZero() {
		u.ID = primitive.NewObjectID()
	}
	_, err = db.Collection(u.getCollectionName()).InsertOne(ctx, u)

	if err != nil {
		return err
	}
	return nil
}
