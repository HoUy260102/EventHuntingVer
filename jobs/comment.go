package jobs

import (
	"EventHunting/collections"
	"EventHunting/database"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func DeleteComment() error {
	var (
		softDeleteDays    = 30
		commentEntry      = &collections.Comment{}
		mediaEntry        = &collections.Media{}
		baseCommentFilter = bson.M{
			"deleted_at": bson.M{
				"$exists": true,
				"$lte":    time.Now().Add(-time.Duration(softDeleteDays) * 24 * time.Hour),
			},
		}
		mediaIds    = make([]primitive.ObjectID, 0)
		mediaUrlIDs = make([]string, 0)
		commentIDs  = make([]primitive.ObjectID, 0)
		ctx, cancel = context.WithTimeout(context.Background(), 1000*time.Second)
	)
	defer cancel()

	//Lấy tất cả các comment
	comments, err := commentEntry.Find(baseCommentFilter)
	if err != nil {
		return err
	}

	if len(comments) == 0 {
		return nil
	}

	for _, comment := range comments {
		mediaIds = append(mediaIds, comment.MediaIds...)
		commentIDs = append(commentIDs, comment.ID)
	}
	var (
		mediaFilter = bson.M{
			"_id": bson.M{
				"$in": mediaIds,
			},
		}
	)

	medias, err := mediaEntry.Find(nil, mediaFilter)
	if err != nil {
		return err
	}

	for _, media := range medias {
		mediaUrlIDs = append(mediaUrlIDs, media.UrlId)
	}

	err = deletedCommentMedias(mediaUrlIDs)
	if err != nil {
		return err
	}

	session, err := database.GetDB().Client().StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	// Chạy transaction
	err = mongo.WithSession(ctx, session, func(sessCtx mongo.SessionContext) error {
		_, err := sessCtx.WithTransaction(sessCtx, func(sessCtx mongo.SessionContext) (interface{}, error) {
			//Xóa media trong DB
			err = mediaEntry.DeleteMany(sessCtx, bson.M{
				"_id": bson.M{
					"$in": mediaIds,
				},
			})

			if err != nil {
				return nil, err
			}

			//Xóa comment trong DB
			err = commentEntry.DeleteMany(sessCtx, bson.M{
				"_id": bson.M{
					"$in": commentIDs,
				},
			})

			if err != nil {
				return nil, err
			}
			return nil, nil
		})
		return err
	})
	if err != nil {
		return err
	}

	return nil
}
