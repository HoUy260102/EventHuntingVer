package database

import (
	"EventHunting/configs"
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	Client *redis.Client
	Ctx    context.Context
}

var (
	redisClient *RedisClient
)

func NewRedisClient() error {
	client := redis.NewClient(&redis.Options{
		Addr:     configs.GetRedisAddr(),
		Password: configs.GetRedisPassword(),
		DB:       configs.GetRedisDB(),
	})

	ctx := context.Background()
	if _, err := client.Ping(ctx).Result(); err != nil {
		log.Printf("Kết nối Redis thất bại: %v", err)
		return err
	}

	log.Println("Kết nối Redis thành công!")
	redisClient = &RedisClient{
		Client: client,
		Ctx:    ctx,
	}
	return nil
}

func GetRedisClient() *RedisClient {
	return redisClient
}
