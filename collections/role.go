package collections

import (
	"EventHunting/database"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Role struct {
	Id            primitive.ObjectID   `bson:"_id" json:"id"`
	Name          string               `bson:"name" json:"name"`
	Status        string               `bson:"status" json:"status"`
	PermissionIds []primitive.ObjectID `bson:"permission_ids,omitempty" json:"permission_ids"`

	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	CreatedBy primitive.ObjectID `bson:"created_by" json:"created_by"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
	UpdatedBy primitive.ObjectID `bson:"updated_by" json:"updated_by"`
	DeletedAt time.Time          `bson:"deleted_at,omitempty" json:"deleted_at"`
	DeletedBy primitive.ObjectID `bson:"deleted_by,omitempty" json:"deleted_by"`
}

func (u *Role) getCollectionName() string {
	return "roles"
}

func (u *Role) FindById(roleId primitive.ObjectID) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()

	err := db.Collection(u.getCollectionName()).FindOne(ctx, bson.M{"_id": roleId}).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Role) First(filter bson.M, opts ...*options.FindOneOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()

	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Role) Find(filter bson.M, opts ...*options.FindOptions) ([]Role, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		roles       []Role
	)
	defer cancel()

	cursor, err := db.Collection(u.getCollectionName()).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var role Role
		if err := cursor.Decode(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return roles, nil
}

func (u *Role) Create() error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()
	u.Id = primitive.NewObjectID()
	_, err := db.Collection(u.getCollectionName()).InsertOne(ctx, u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Role) UpdateOne(filter bson.M, update bson.M) error {
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
