package collections

import (
	"EventHunting/consts"
	"EventHunting/database"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Ticket struct {
	ID primitive.ObjectID `json:"id" bson:"_id,omitempty"`

	EventID      primitive.ObjectID `json:"event_id" bson:"event_id"`
	TicketTypeID primitive.ObjectID `json:"ticket_type_id" bson:"ticket_type_id"`
	RegisID      primitive.ObjectID `json:"regis_id" bson:"regis_id"`

	QRCodeData string `json:"qr_code_data" bson:"qr_code_data"`

	Status consts.TicketStatus `json:"status" bson:"status"`

	CheckedInAt []string `json:"checked_in_at,omitempty" bson:"checked_in_at,omitempty"`

	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
	CreatedBy primitive.ObjectID `json:"created_by" bson:"created_by"`
	UpdatedAt time.Time          `json:"updated_at" bson:"updated_at"`
	UpdatedBy primitive.ObjectID `json:"updated_by" bson:"updated_by"`
}

type Tickets []Ticket

func (u *Ticket) getCollectionName() string {
	return "tickets"
}

func (u *Ticket) First(ctx context.Context, filter bson.M, opts ...*options.FindOneOptions) error {
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

func (u *Ticket) Find(ctx context.Context, filter bson.M, opts ...*options.FindOptions) (Tickets, error) {
	var (
		db      = database.GetDB()
		tickets Tickets
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

	if err = cursor.All(ctx, &tickets); err != nil {
		return nil, err
	}

	if tickets == nil {
		tickets = []Ticket{}
	}

	return tickets, nil
}

func (u *Ticket) Create(ctx context.Context) error {
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

func (u *Ticket) CreateMany(ctx context.Context, tickets []Ticket, opts ...*options.InsertManyOptions) error {
	var (
		db  = database.GetDB()
		err error
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	_, err = db.Collection(u.getCollectionName()).InsertMany(ctx, toTicketInterfaceSlice(tickets), opts...)
	if err != nil {
		return err
	}
	return nil
}

func toTicketInterfaceSlice(tickets []Ticket) []interface{} {
	var result []interface{}
	for _, ticket := range tickets {
		result = append(result, ticket)
	}
	return result
}
