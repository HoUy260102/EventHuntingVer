package collections

import (
	"EventHunting/database"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Permission struct {
	ID      primitive.ObjectID `bson:"_id" json:"id"`
	Name    string             `bson:"name" json:"name"`
	Subject string             `bson:"subject" json:"subject"`
	Action  string             `bson:"action" json:"action"`
	Disable bool               `bson:"disable" json:"disable"`
	Active  string             `bson:"active,omitempty" json:"active"`

	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	CreatedBy primitive.ObjectID `bson:"created_by" json:"created_by"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
	UpdatedBy primitive.ObjectID `bson:"updated_by" json:"updated_by"`
	DeletedAt time.Time          `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
	DeletedBy primitive.ObjectID `bson:"deleted_by,omitempty" json:"deleted_by,omitempty"`
}

type Permissions []Permission

func (u *Permission) getCollectionName() string {
	return "permissions"
}

func (u *Permission) First(filter bson.M, opts ...*options.FindOneOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()
	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter, opts...).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Permission) Find(filter bson.M, opts ...*options.FindOptions) (Permissions, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()

	cursor, err := db.Collection(u.getCollectionName()).Find(ctx, filter, opts...)
	if err != nil {
		return nil, fmt.Errorf("Lỗi khi tìm danh sách permission: %v", err)
	}
	defer cursor.Close(ctx)

	var permissions Permissions
	if err := cursor.All(ctx, &permissions); err != nil {
		return nil, fmt.Errorf("Lỗi khi đọc dữ liệu permission: %v", err)
	}

	return permissions, nil
}

func (u *Permission) Create() error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()
	u.ID = primitive.NewObjectID()
	_, err := db.Collection(u.getCollectionName()).InsertOne(ctx, u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Permission) UpdateOne(filter bson.M, update bson.M) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()
	res, err := db.Collection(u.getCollectionName()).UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (u *Permission) UpdateMany(filter bson.M, update bson.M, opts ...*options.UpdateOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()
	res, err := db.Collection(u.getCollectionName()).UpdateOne(ctx, filter, update, opts...)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (u *Permission) CountDocuments(filter bson.M) (int64, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()
	count, err := db.Collection(u.getCollectionName()).CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (p *Permission) ParseEntry() bson.M {
	result := bson.M{
		"_id":     p.ID,
		"name":    p.Name,
		"subject": p.Subject,
		"action":  p.Action,
		"disable": p.Disable,
		"active":  p.Active,

		"created_at": p.CreatedAt,
		"created_by": p.CreatedBy,
		"updated_at": p.UpdatedAt,
		"updated_by": p.UpdatedBy,
		"deleted_at": p.DeletedAt,
		"deleted_by": p.DeletedBy,
	}

	return result
}
