package jobs

import (
	"EventHunting/collections"
	"EventHunting/database"
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func DeleteBlog() error {
	var (
		softDeleteDays = 30
		blogEntry      = &collections.Blog{}
		commentEntry   = &collections.Comment{}
		mediaEntry     = &collections.Media{}
		baseBlogFilter = bson.M{
			"deleted_at": bson.M{
				"$exists": true,
				"$lte":    time.Now().Add(-time.Duration(softDeleteDays) * 24 * time.Hour),
			},
		}
		mediaIds    = make([]primitive.ObjectID, 0)
		mediaUrlIDs = make([]string, 0)
		blogIDs     = make([]primitive.ObjectID, 0)
		ctx, cancel = context.WithTimeout(context.Background(), 1000*time.Second)
	)
	defer cancel()

	//Lấy tất cả các comment
	blogs, err := blogEntry.Find(nil, baseBlogFilter)
	if err != nil {
		return err
	}

	if len(blogs) == 0 {
		return nil
	}
	//Lấy danh sách các blogIDs
	for _, blog := range blogs {
		blogIDs = append(blogIDs, blog.ID)
		if !blog.ThumbnailID.IsZero() {
			mediaIds = append(mediaIds, blog.ThumbnailID)
		}
		if len(blog.MediaIDs) > 0 {
			mediaIds = append(mediaIds, blog.MediaIDs...)
		}
	}

	//Lấy danh sách các comment của blog được xóa
	comments, err := commentEntry.Find(bson.M{
		"blog_id": bson.M{
			"$in": blogIDs,
		},
	})

	if err != nil {
		return err
	}

	//Lấy danh sách các media cần xóa
	for _, comment := range comments {
		mediaIds = append(mediaIds, comment.MediaIds...)
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

	//Lấy danh sách public url id của media
	for _, media := range medias {
		mediaUrlIDs = append(mediaUrlIDs, media.UrlId)
	}

	//Xóa media trên cld
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

			// Xóa media
			if len(mediaIds) > 0 {
				if err := mediaEntry.DeleteMany(sessCtx, bson.M{"_id": bson.M{"$in": mediaIds}}); err != nil {
					return nil, err
				}
			}

			if len(blogIDs) > 0 {
				// Xóa comment
				if err := commentEntry.DeleteMany(sessCtx, bson.M{"blog_id": bson.M{"$in": blogIDs}}); err != nil {
					return nil, err
				}
				// Xóa blog
				if err := blogEntry.DeleteMany(sessCtx, bson.M{"_id": bson.M{"$in": blogIDs}}); err != nil {
					return nil, err
				}
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

func UpdateViewsBlogToMongo(ctx context.Context) {
	var (
		blogEntry = &collections.Blog{}
	)
	log.Println("Worker: bắt đầu ...")
	redisClient := database.GetRedisClient().Client
	db := database.GetDB()
	if redisClient == nil || db == nil {
		log.Println("Worker: redis lỗi nil")
		return
	}

	var cursor uint64
	var keys []string
	var err error

	for {
		keys, cursor, err = redisClient.Scan(ctx, cursor, "views:blog:*", 100).Result() // 100 keys mỗi lần
		if err != nil {
			log.Printf("Worker: Error scanning Redis keys: %v", err)
			return
		}

		for _, key := range keys {
			hotCountStr, err := redisClient.GetSet(ctx, key, 0).Result()
			if err != nil {
				log.Printf("Worker:Lỗi GETSET key %s: %v", key, err)
				continue
			}

			hotCount, _ := strconv.Atoi(hotCountStr)
			if hotCount == 0 {
				continue
			}

			// 3. Trích xuất ID từ key
			idStr := strings.TrimPrefix(key, "views:blog:")
			blogID, err := primitive.ObjectIDFromHex(idStr)
			if err != nil {
				log.Printf("Worker: Invalid blog ID from key %s", key)
				continue
			}

			// Cập nhật db
			filter := bson.M{"_id": blogID}
			update := bson.M{
				"$inc": bson.M{
					"view": hotCount,
				},
			}

			err = blogEntry.Update(ctx, filter, update)
			if err != nil {
				log.Printf("Worker: Lỗi cập nhật vào db %s: %v", hotCount, idStr, err)
			}
		}

		if cursor == 0 {
			break
		}
	}
	log.Println("Worker: Kết thúc.")
}
