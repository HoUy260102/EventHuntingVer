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

func (u *Session) FindById(db *mongo.Database, ctx context.Context, sessionId primitive.ObjectID) (Session, error) {
	var session Session
	err := db.Collection(u.getCollectionName()).FindOne(ctx, bson.M{"_id": sessionId}).Decode(&session)
	if err != nil {
		return Session{}, err
	}
	return session, nil
}

func (u *Session) First(db *mongo.Database, ctx context.Context, filter bson.M) (Session, error) {
	var session Session
	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter).Decode(&session)
	if err != nil {
		return Session{}, err
	}
	return session, nil
}

func (u *Session) Find(db *mongo.Database, ctx context.Context, filter bson.M, opts ...*options.FindOptions) ([]Session, error) {
	var sessions []Session

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

func (u *Session) Create(db *mongo.Database, ctx context.Context, session Session) (*mongo.InsertOneResult, error) {
	return db.Collection(u.getCollectionName()).InsertOne(ctx, session)
}

func (u *Session) Update(db *mongo.Database, ctx context.Context, filter bson.M, update bson.M) error {
	res, err := db.Collection(u.getCollectionName()).UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (u *Session) Delete(db *mongo.Database, ctx context.Context, filter bson.M) error {
	_, err := db.Collection(u.getCollectionName()).DeleteOne(ctx, filter)
	return err
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
