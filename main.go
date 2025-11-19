package main

import (
	"EventHunting/configs"
	"EventHunting/database"
	"EventHunting/jobs"
	"EventHunting/routers"
	"EventHunting/utils"
	"fmt"
	"log"

	"github.com/robfig/cron/v3"
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
	err = database.NewRedisClient()
	if err != nil {
		fmt.Println(err)
	}

	//Kết nối với cloudinary
	err = utils.InitCloudinary()
	if err != nil {
		fmt.Println(err)
	}

	//Kêt nối google
	utils.InitOAuth()

	//Chạy cronjob
	c := cron.New(cron.WithSeconds())
	_, err = c.AddFunc("@every 2m", jobs.HandleExpiredRegistrations)
	if err != nil {
		log.Fatal("Lỗi khi thêm cron job:", err)
	}
	_, err = c.AddFunc("@every 2m", jobs.UpdateViewsBlogToMongo)
	if err != nil {
		log.Fatal("Lỗi khi thêm cron job:", err)
	}
	_, err = c.AddFunc("@every 2m", jobs.UpdateViewsEventToMongo)
	if err != nil {
		log.Fatal("Lỗi khi thêm cron job:", err)
	}
	_, err = c.AddFunc("@every 24h", func() {
		if err := jobs.DeleteComment(); err != nil {
			log.Printf("Lỗi khi chạy DeleteComment cron job: %v", err)
		} else {
			log.Println("DeleteComment cron job chạy thành công")
		}
	})
	if err != nil {
		log.Fatal("Lỗi khi thêm cron job:", err)
	}
	_, err = c.AddFunc("@every 24h", func() {
		if err := jobs.DeleteBlog(); err != nil {
			log.Printf("Lỗi khi chạy DeleteBlog cron job: %v", err)
		} else {
			log.Println("DeleteBlog cron job chạy thành công")
		}
	})
	if err != nil {
		log.Fatal("Lỗi khi thêm cron job:", err)
	}
	_, err = c.AddFunc("@every 1m", func() {
		if err := jobs.DeletedMedias("comments"); err != nil {
			log.Printf("Lỗi khi chạy DeletedMedias cron job: %v", err)
		} else {
			log.Println("DeletedMedias cron job chạy thành công")
		}
	})
	if err != nil {
		log.Fatal("Lỗi khi thêm cron job:", err)
	}
	_, err = c.AddFunc("@every 1m", func() {
		if err := jobs.DeletedMedias("Comments"); err != nil {
			log.Printf("Lỗi khi chạy DeletedMedias cron job: %v", err)
		} else {
			log.Println("DeletedMedias cron job chạy thành công")
		}
	})
	if err != nil {
		log.Fatal("Lỗi khi thêm cron job:", err)
	}
	_, err = c.AddFunc("@every 2m", func() {
		if err := jobs.DeletedMedias("Blogs"); err != nil {
			log.Printf("Lỗi khi chạy DeletedMedias cron job: %v", err)
		} else {
			log.Println("DeletedMedias cron job chạy thành công")
		}
	})
	if err != nil {
		log.Fatal("Lỗi khi thêm cron job:", err)
	}
	_, err = c.AddFunc("@every 2m", func() {
		if err := jobs.DeletedMedias("Events"); err != nil {
			log.Printf("Lỗi khi chạy DeletedMedias cron job: %v", err)
		} else {
			log.Println("DeletedMedias cron job chạy thành công")
		}
	})
	if err != nil {
		log.Fatal("Lỗi khi thêm cron job:", err)
	}
	c.Start()

	//Chạy worker
	for i := 0; i < 5; i++ {
		go jobs.StartEmailQueue()
	}

	//Đăng ký router
	if err := routers.SetupRouter(); err != nil {
		fmt.Println("Server chạy thất bại: %v", err)
	}
}
