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

func NewRedisClient() (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     configs.GetRedisAddr(),
		Password: configs.GetRedisPassword(),
		DB:       configs.GetRedisDB(),
	})

	ctx := context.Background()
	if _, err := client.Ping(ctx).Result(); err != nil {
		log.Printf("Kết nối Redis thất bại: %v", err)
		return nil, err
	}

	log.Println("Kết nối Redis thành công!")
	return &RedisClient{
		Client: client,
		Ctx:    ctx,
	}, nil
}
