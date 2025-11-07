package collections

import (
	"EventHunting/database"
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Tag struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Slug        string             `bson:"slug" json:"slug"`

	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	CreatedBy primitive.ObjectID `bson:"created_by" json:"created_by"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
	UpdatedBy primitive.ObjectID `bson:"updated_by" json:"updated_by"`
	DeletedAt time.Time          `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
	DeletedBy primitive.ObjectID `bson:"deleted_by,omitempty" json:"deleted_by,omitempty"`
}

type Tags []Tag

func (u *Tag) getCollectionName() string {
	return "tags"
}

func (u *Tag) First(filter bson.M, opts ...options.FindOptions) error {
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

func (u *Tag) FindById(id primitive.ObjectID) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		filter      = bson.M{
			"_id":        id,
			"deleted_at": bson.M{"$exists": false},
		}
	)
	defer cancel()

	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter).Decode(u)
	if err != nil {
		return err
	}
	return err
}

func (u *Tag) Find(filter bson.M, opts ...*options.FindOptions) (Tags, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		tags        Tags
	)
	defer cancel()

	if filter == nil {
		filter = bson.M{}
	}

	filter["deleted_at"] = bson.M{"$exists": false}

	cursor, err := db.Collection(u.getCollectionName()).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &tags); err != nil {
		return nil, err
	}

	if tags == nil {
		tags = []Tag{}
	}

	return tags, nil
}

func (u *Tag) Create() error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()
	u.ID = primitive.NewObjectID()

	_, err = db.Collection(u.getCollectionName()).InsertOne(ctx, u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Tag) Update(filter bson.M, updateDoc bson.M, opts ...*options.UpdateOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()

	res, err := db.Collection(u.getCollectionName()).UpdateOne(ctx, filter, updateDoc, opts...)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (u *Tag) Delete(id primitive.ObjectID) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()

	filter := bson.M{
		"_id":        id,
		"deleted_at": bson.M{"$exists": false},
	}
	result, err := db.Collection(u.getCollectionName()).DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return errors.New("Không tìm thấy document")
	}
	return nil
}

func (t *Tag) ParseEntry() bson.M {
	result := bson.M{
		"_id":         t.ID,
		"name":        t.Name,
		"description": t.Description,
		"slug":        t.Slug,
		"created_at":  t.CreatedAt,
		"created_by":  t.CreatedBy,
		"updated_at":  t.UpdatedAt,
		"updated_by":  t.UpdatedBy,
		"deleted_at":  t.DeletedAt,
		"deleted_by":  t.DeletedBy,
	}

	return result
}
