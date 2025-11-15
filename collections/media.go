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

type Media struct {
	ID        primitive.ObjectID `bson:"_id" json:"id"`
	Url       string             `bson:"url" json:"url"`
	UrlId     string             `bson:"url_id" json:"url_id"`
	Type      consts.MediaFormat `bson:"type" json:"type"`
	Status    consts.MediaStatus `bson:"status" json:"status"`
	Extension string             `bson:"extension" json:"extension"`

	CollectionName string    `bson:"collection_name" json:"collection_name"`
	CreatedAt      time.Time `bson:"created_at" json:"created_at"`
}

type Medias []Media

func (u *Media) getCollectionName() string {
	return "medias"
}

func (u *Media) First(ctx context.Context, filter bson.M, opts ...*options.FindOneOptions) error {
	var (
		db  = database.GetDB()
		err error
	)
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	err = db.Collection(u.getCollectionName()).FindOne(ctx, filter, opts...).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Media) Find(ctx context.Context, filter bson.M, opts ...*options.FindOptions) ([]Media, error) {
	var (
		db     = database.GetDB()
		medias []Media
		err    error
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	cursor, err := db.Collection(u.getCollectionName()).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var media Media
		if err := cursor.Decode(&media); err != nil {
			return nil, err
		}
		medias = append(medias, media)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return medias, nil
}

func (u *Media) Create(ctx context.Context) error {
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

func (u *Media) CreateMany(ctx context.Context, medias []Media, opts ...*options.InsertManyOptions) error {
	var (
		db  = database.GetDB()
		err error
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	_, err = db.Collection(u.getCollectionName()).InsertMany(ctx, toInterfaceSlice(medias), opts...)
	if err != nil {
		return err
	}
	return nil
}

func (u *Media) Update(ctx context.Context, filter bson.M, update bson.M, opts ...*options.UpdateOptions) error {
	var (
		db  = database.GetDB()
		err error
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	_, err = db.Collection(u.getCollectionName()).UpdateOne(ctx, filter, update, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (u *Media) UpdateMany(ctx context.Context, filter bson.M, update bson.M, opts ...*options.UpdateOptions) error {
	var (
		db  = database.GetDB()
		err error
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	_, err = db.Collection(u.getCollectionName()).UpdateMany(ctx, filter, update, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (u *Media) DeleteMany(ctx context.Context, filter bson.M, opts ...*options.DeleteOptions) error {
	var (
		db  = database.GetDB()
		err error
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	_, err = db.Collection(u.getCollectionName()).DeleteMany(ctx, filter, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (u *Media) ParseEntry() bson.M {
	result := bson.M{
		"id":        u.ID,
		"url":       u.Url,
		"url_id":    u.UrlId,
		"extention": u.Extension,
		"type":      u.Type,
		"status":    u.Status,
	}
	return result
}

func toInterfaceSlice(medias []Media) []interface{} {
	var result []interface{}
	for _, media := range medias {
		result = append(result, media)
	}
	return result
}
