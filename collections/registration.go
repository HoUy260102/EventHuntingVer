package collections

import (
	"EventHunting/consts"
	"EventHunting/database"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Registration struct {
	ID        primitive.ObjectID  `bson:"_id" json:"id"`
	EventID   primitive.ObjectID  `bson:"event_id" json:"event_id"`
	InvoiceID *primitive.ObjectID `bson:"invoice_id" json:"invoice_id"`

	//PaymentMethod *string    `bson:"payment_method" json:"payment_method"`
	//Type
	Tickets []struct {
		TicketTypeID primitive.ObjectID `json:"ticket_type_id"`
		Quantity     int                `json:"quantity"`
	} `bson:"tickets" json:"tickets"`
	TotalQuantity int                            `bson:"total_quantity" json:"total_quantity"`
	TotalPrice    int                            `bson:"total_price" json:"total_price"`
	Status        consts.EventRegistrationStatus `bson:"status" json:"status"` // pending, paid, cancelled, refunded

	PaidAt            *time.Time `bson:"paid_at,omitempty" json:"paid_at,omitempty"`
	CancelledAt       *time.Time `bson:"cancelled_at,omitempty" json:"cancelled_at,omitempty"`
	TicketEmailSentAt *time.Time `bson:"ticket_email_sent_at,omitempty" json:"ticket_email_sent_at,omitempty"`
	
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	CreatedBy primitive.ObjectID `bson:"created_by" json:"created_by"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
	UpdatedBy primitive.ObjectID `bson:"updated_by" json:"updated_by"`
	DeletedAt time.Time          `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
	DeletedBy primitive.ObjectID `bson:"deleted_by,omitempty" json:"deleted_by,omitempty"`
}
type Registrations []Registration

func (u *Registration) getCollectionName() string {
	return "registrations"
}

func (u *Registration) Create(ctx context.Context) error {
	var (
		db  = database.GetDB()
		err error
	)
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	if u.ID.IsZero() {
		u.ID = primitive.NewObjectID()
	}
	_, err = db.Collection(u.getCollectionName()).InsertOne(ctx, u)

	if err != nil {
		return err
	}
	return nil
}

func (u *Registration) First(ctx context.Context, filter bson.M, opts ...*options.FindOneOptions) error {
	var (
		db = database.GetDB()
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter, opts...).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Registration) FindById(ctx context.Context, id primitive.ObjectID) error {
	var (
		db     = database.GetDB()
		filter = bson.M{
			"_id":        id,
			"deleted_at": bson.M{"$exists": false},
		}
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	var blog Blog
	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter).Decode(&blog)
	if err != nil {
		return err
	}
	return nil
}

func (u *Registration) Find(ctx context.Context, filter bson.M, opts ...*options.FindOptions) (Registrations, error) {
	var (
		db            = database.GetDB()
		registrations Registrations
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	if filter == nil {
		filter = bson.M{}
	}
	filter["deleted_at"] = bson.M{"$exists": false}

	cursor, err := db.Collection(u.getCollectionName()).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &registrations); err != nil {
		return nil, err
	}

	if registrations == nil {
		registrations = []Registration{}
	}

	return registrations, nil
}

func (u *Registration) Update(ctx context.Context, filter bson.M, updateDoc bson.M, opts ...*options.UpdateOptions) error {
	var (
		db = database.GetDB()
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	if filter == nil {
		filter = bson.M{}
	}

	res, err := db.Collection(u.getCollectionName()).UpdateOne(ctx, filter, updateDoc, opts...)
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (u *Registration) DeleteMany(ctx context.Context, filter bson.M) error {
	var (
		db  = database.GetDB()
		err error
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	_, err = db.Collection(u.getCollectionName()).DeleteMany(ctx, filter)
	if err != nil {
		return err
	}

	return nil
}
