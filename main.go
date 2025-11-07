package main

import (
	"EventHunting/configs"
	"EventHunting/database"
	"EventHunting/routers"
	"EventHunting/utils"
	"fmt"
)

func main() {
	//TIP <p>Press <shortcut actionId="ShowIntentionActions"/> when your caret is at the underlined text
	// to see how GoLand suggests fixing the warning.</p><p>Alternatively, if available, click the lightbulb to view possible fixes.</p>
	configs.LoadFileConfig()
	//Kết nối đến database
	err := database.ConnectMongo()
	if err != nil {
		fmt.Println(err)
	}
	//Kết nối đến redis
	redisClient, err := database.NewRedisClient()
	if err != nil {
		fmt.Println(err)
	}
	//Kết nối với cloudinary
	err = utils.InitCloudinary()
	if err != nil {
		fmt.Println(err)
	}
	//Đăng ký router
	if err := routers.SetupRouter(redisClient.Client); err != nil {
		fmt.Println("Server chạy thất bại: %v", err)
	}
}
