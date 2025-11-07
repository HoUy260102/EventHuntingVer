package routers

import (
	"EventHunting/configs"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func SetupRouter(redisClient *redis.Client) error {
	r := gin.Default()
	api := r.Group("/api/v1")
	Register(api, redisClient)
	return r.Run(fmt.Sprintf(":%s", configs.GetServerPort()))
}
