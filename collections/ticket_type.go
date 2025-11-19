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

type TicketType struct {
	ID              primitive.ObjectID `bson:"_id" json:"id"`
	EventID         primitive.ObjectID `bson:"event_id" json:"event_id"`
	Name            string             `bson:"name" json:"name"`
	Description     string             `bson:"description,omitempty" json:"description,omitempty"`
	Price           int                `bson:"price" json:"price"` // (0 nếu free)
	Quantity        *int               `bson:"quantity,omitempty" json:"quantity"`
	RegisteredCount int                `bson:"registered_count" json:"registered_count"`

	Status consts.TicketTypeStatus `bson:"status"` // active / inactive / canceled
	//Tiện ích danh sách tiện ích (optional)

	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	CreatedBy primitive.ObjectID `bson:"created_by" json:"created_by"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
	UpdatedBy primitive.ObjectID `bson:"updated_by" json:"updated_by"`
	DeletedAt time.Time          `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
	DeletedBy primitive.ObjectID `bson:"deleted_by,omitempty" json:"deleted_by,omitempty"`
}

type TicketTypes []TicketType

func (u *TicketType) getCollectionName() string {
	return "ticket_types"
}

func (u *TicketType) Create(ctx context.Context) error {
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

func (u *TicketType) First(ctx context.Context, filter bson.M, opts ...*options.FindOneOptions) error {
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

func (u *TicketType) FindById(ctx context.Context, id primitive.ObjectID) error {
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

	var blog TicketType
	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter).Decode(&blog)
	if err != nil {
		return err
	}
	return nil
}

func (u *TicketType) Find(ctx context.Context, filter bson.M, opts ...*options.FindOptions) (TicketTypes, error) {
	var (
		db          = database.GetDB()
		ticketTypes TicketTypes
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

	if err = cursor.All(ctx, &ticketTypes); err != nil {
		return nil, err
	}

	if ticketTypes == nil {
		ticketTypes = []TicketType{}
	}

	return ticketTypes, nil
}

func (u *TicketType) Update(ctx context.Context, filter bson.M, updateDoc bson.M, opts ...*options.UpdateOptions) error {
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

func (u *TicketType) DeleteMany(ctx context.Context, filter bson.M) error {
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

func (t *TicketType) ParseEntry() bson.M {
	result := bson.M{
		"_id":              t.ID,
		"event_id":         t.EventID,
		"name":             t.Name,
		"description":      t.Description,
		"registered_count": t.RegisteredCount,
		"status":           t.Status,
		"created_at":       t.CreatedAt,
		"created_by":       t.CreatedBy,
		"updated_at":       t.UpdatedAt,
		"updated_by":       t.UpdatedBy,
	}

	if t.Price == 0 {
		result["price"] = "Vé miễn phí"
	} else {
		result["price"] = t.Price
	}

	if t.Quantity != nil {
		result["quantity"] = *t.Quantity
	} else {
		result["quantity"] = "Vé này không giới hạn số lượng bán vé!"
	}

	if !t.DeletedAt.IsZero() {
		result["deleted_at"] = t.DeletedAt
	}

	if t.DeletedBy != primitive.NilObjectID {
		result["deleted_by"] = t.DeletedBy
	}

	return result
}
