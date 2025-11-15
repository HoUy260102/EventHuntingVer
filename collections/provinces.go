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

type Province struct {
	ID   primitive.ObjectID `bson:"_id" json:"id"`
	Name string             `bson:"name" json:"name"`
}

type Provinces []Province

func (u *Province) getCollectionName() string {
	return "provinces"
}

func (u *Province) Create(ctx context.Context) error {
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

	// TODO: Thêm vào redis nếu sử dụng cache-aside

	if err != nil {
		return err
	}
	return nil
}

func (u *Province) First(ctx context.Context, filter bson.M, opts ...*options.FindOneOptions) error {
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

func (u *Province) FindById(ctx context.Context, id primitive.ObjectID) error {
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

	var blog Province
	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter).Decode(&blog)
	if err != nil {
		return err
	}
	return nil
}

func (u *Province) Find(ctx context.Context, filter bson.M, opts ...*options.FindOptions) (Provinces, error) {
	var (
		db        = database.GetDB()
		provinces Provinces
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

	if err = cursor.All(ctx, &provinces); err != nil {
		return nil, err
	}

	if provinces == nil {
		provinces = []Province{}
	}

	return provinces, nil
}

func (u *Province) Update(ctx context.Context, filter bson.M, updateDoc bson.M, opts ...*options.UpdateOptions) error {
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

func (u *Province) DeleteMany(ctx context.Context, filter bson.M) error {
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
