package routers

import (
	"EventHunting/configs"
	"fmt"

	"github.com/gin-gonic/gin"
)

func SetupRouter() error {
	r := gin.Default()
	api := r.Group("/api/v1")
	Register(api)
	return r.Run(fmt.Sprintf(":%s", configs.GetServerPort()))
}
