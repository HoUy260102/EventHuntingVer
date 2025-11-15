package jobs

import (
	"EventHunting/collections"
	"EventHunting/database"
	"context"
	"log"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func UpdateViewsEventToMongo(ctx context.Context) {
	var (
		eventEntry = &collections.Event{}
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
		keys, cursor, err = redisClient.Scan(ctx, cursor, "views:event:*", 100).Result() // 100 keys mỗi lần
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
			eventID, err := primitive.ObjectIDFromHex(idStr)
			if err != nil {
				log.Printf("Worker: Invalid blog ID from key %s", key)
				continue
			}

			// Cập nhật db
			filter := bson.M{"_id": eventID}
			update := bson.M{
				"$inc": bson.M{
					"view": hotCount,
				},
			}

			err = eventEntry.Update(ctx, filter, update)
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
