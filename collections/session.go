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

type Session struct {
	Id            primitive.ObjectID `bson:"_id"`
	UserId        primitive.ObjectID `bson:"user_id"`
	RefreshToken  string             `bson:"refresh_token"`
	IsRevoked     bool               `bson:"is_revoked"`
	TrustedDevice bool               `bson:"trusted_device"`
	DeviceId      string             `bson:"device_id"`
	CreatedAt     time.Time          `bson:"created_at"`
	ExpiresAt     time.Time          `bson:"expires_at"`
	ApprovedToken string             `bson:"approved_token"`
}

func (s *Session) getCollectionName() string {
	return "sessions"
}

func (u *Session) FindById(ctx context.Context, sessionId primitive.ObjectID) error {
	var (
		db = database.GetDB()
	)
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	err := db.Collection(u.getCollectionName()).FindOne(ctx, bson.M{"_id": sessionId}).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Session) First(ctx context.Context, filter bson.M) error {
	var (
		db = database.GetDB()
	)
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Session) Find(ctx context.Context, filter bson.M, opts ...*options.FindOptions) ([]Session, error) {
	var (
		db       = database.GetDB()
		sessions []Session
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	cursor, err := db.Collection(u.getCollectionName()).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &sessions); err != nil {
		return nil, err
	}

	return sessions, nil
}

func (u *Session) Create(ctx context.Context, session Session) error {
	var (
		db = database.GetDB()
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	if u.Id.IsZero() {
		u.Id = primitive.NewObjectID()
	}
	_, err := db.Collection(u.getCollectionName()).InsertOne(ctx, session)
	if err != nil {
		return err
	}
	return nil
}

func (u *Session) Update(ctx context.Context, filter bson.M, update bson.M, opts ...*options.UpdateOptions) error {
	var (
		db = database.GetDB()
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	res, err := db.Collection(u.getCollectionName()).UpdateOne(ctx, filter, update, opts...)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (u *Session) Delete(ctx context.Context, filter bson.M) error {
	var (
		db = database.GetDB()
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	_, err := db.Collection(u.getCollectionName()).DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	return nil
}

func (u *Session) FindOneAndUpdate(session Session) (Session, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()

	filter := bson.M{
		"user_id":   session.UserId,
		"device_id": session.DeviceId,
	}

	if session.Id.IsZero() {
		session.Id = primitive.NewObjectID()
	}

	update := bson.M{
		"$set": bson.M{
			"refresh_token":  session.RefreshToken,
			"created_at":     session.CreatedAt,
			"expires_at":     session.ExpiresAt,
			"is_revoked":     session.IsRevoked,
			"trusted_device": session.TrustedDevice,
			"approved_token": session.ApprovedToken,
		},
		"$setOnInsert": bson.M{
			"_id":       session.Id, // Sử dụng ID đã tạo
			"user_id":   session.UserId,
			"device_id": session.DeviceId,
		},
	}

	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var updatedSession Session
	res := db.Collection(u.getCollectionName()).FindOneAndUpdate(ctx, filter, update, opts)
	if res.Err() != nil {
		return updatedSession, res.Err()
	}

	err := res.Decode(&updatedSession)
	return updatedSession, err
}
