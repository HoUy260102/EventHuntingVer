package jobs

import (
	"EventHunting/database"
	"context"
)

func WorkerTransactionMail() {
	redisClient := database.GetRedisClient().Client
	ctx := context.Background()

}
