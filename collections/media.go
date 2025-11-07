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
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Url         string             `bson:"url" json:"url"`
	PublicUrlId string             `bson:"public_url_id" json:"public_url_id"`
	Type        consts.MediaFormat `bson:"type" json:"type"`

	Status         string             `bson:"status" json:"status"`
	CollectionName string             `bson:"collection_name" json:"collection_name"`
	DocumentId     primitive.ObjectID `bson:"document_id" json:"document_id"`

	DeletedAt time.Time `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
}

func (u *Media) getCollectionName() string {
	return "medias"
}

func (u *Media) First(filter bson.M, opts ...*options.FindOneOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()

	err = db.Collection(u.getCollectionName()).FindOne(ctx, filter, opts...).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Media) Find(filter bson.M, opts ...*options.FindOptions) ([]Media, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		db          = database.GetDB()
		medias      []Media
		err         error
	)
	defer cancel()

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

func (u *Media) Create() error {
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

func (u *Media) CreateMany(medias []Media, opts ...*options.InsertManyOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()
	_, err = db.Collection(u.getCollectionName()).InsertMany(ctx, toInterfaceSlice(medias), opts...)
	if err != nil {
		return err
	}
	return nil
}

func (u *Media) Update(filter bson.M, update bson.M, opts ...*options.UpdateOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()
	_, err = db.Collection(u.getCollectionName()).UpdateOne(ctx, filter, update, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (u *Media) UpdateMany(filter bson.M, update bson.M, opts ...*options.UpdateOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()
	_, err = db.Collection(u.getCollectionName()).UpdateMany(ctx, filter, update, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (u *Media) DeleteMany(filter bson.M, opts ...*options.DeleteOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()
	_, err = db.Collection(u.getCollectionName()).DeleteMany(ctx, filter, opts...)
	if err != nil {
		return err
	}
	return nil
}

func toInterfaceSlice(medias []Media) []interface{} {
	var result []interface{}
	for _, media := range medias {
		result = append(result, media)
	}
	return result
}
