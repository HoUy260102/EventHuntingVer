package jobs

import (
	"EventHunting/configs"
	"EventHunting/consts"
	"EventHunting/database"
	"EventHunting/service"
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EmailJob struct {
	Type       string `json:"type"`
	Data       bson.M `json:"data"`
	RetryCount int    `json:"retry_count"`
}

func StartEmailQueue() {
	rdb := database.GetRedisClient().Client
	log.Printf("WORKER STARTED: Đang lắng nghe queue '%s'...", consts.QueueNameEmail)

	for {
		result, err := rdb.BLPop(context.Background(), 0, consts.QueueNameEmail).Result()
		if err != nil {
			log.Printf("ERROR: Kết nối redis lỗi: %v. Retry in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// Parse Job
		jobPayload := []byte(result[1])
		var job EmailJob
		if err := json.Unmarshal(jobPayload, &job); err != nil {
			log.Printf("ERROR: JSON unmarshal -> BỎ QUA JOB: %v", err)
			continue
		}

		log.Printf("WORKER: Nhận job [%s] (Retry: %d)", job.Type, job.RetryCount)

		processSingleJob(job)
	}
}

func processSingleJob(job EmailJob) {
	var err error

	switch job.Type {
	case "ticket_email":
		regIDStr, ok := job.Data["registration_id"].(string)
		if !ok {
			log.Printf("ERROR: Dữ liệu job thiếu 'registration_id' -> BỎ QUA")
			return
		}

		regID, parseErr := primitive.ObjectIDFromHex(regIDStr)
		if parseErr != nil {
			log.Printf("ERROR: ID không hợp lệ: %s -> BỎ QUA", regIDStr)
			return
		}

		err = service.ProcessTicketAndEmail(regID)

	default:
		log.Printf("WARN: Không biết loại job '%s' -> BỎ QUA", job.Type)
		return
	}

	// Xử lý kết quả
	if err != nil {
		log.Printf("FAIL: Job thất bại: %v", err)

		//Kiểm tra các lỗi cần retry
		if errors.Is(err, consts.ErrFatalDataNotFound) || errors.Is(err, consts.ErrFatalInvalidData) {
			log.Printf("DROP: Lỗi dữ liệu không thể cứu vãn -> Hủy Job.")
			return
		}

		// Nếu là lỗi thường
		if job.RetryCount < configs.GetMaxRetries() {
			job.RetryCount++
			log.Printf("RETRY: Đẩy lại vào queue (Lần %d/%d)", job.RetryCount, configs.GetMaxRetries())
			pushBackToQueue(job)
		} else {
			log.Printf("DROP: Đã thử %d lần vẫn lỗi -> Hủy Job để tránh kẹt queue.", configs.GetMaxRetries())
		}
	} else {
		log.Printf("DONE: Xử lý xong job %s", job.Type)
	}
}

func pushBackToQueue(job EmailJob) {
	payload, _ := json.Marshal(job)
	database.GetRedisClient().Client.RPush(context.Background(), consts.QueueNameEmail, payload)
}

//func processEmailJob(jobPayload []byte) {
//	var (
//		job         EmailJob
//		redisClient = database.GetRedisClient().Client
//		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
//	)
//	defer cancel()
//
//	if err := json.Unmarshal(jobPayload, &job); err != nil {
//		log.Printf("ERROR: Không thể unmarshal job: %v.", err)
//		return
//	}
//
//	log.Printf("INFO: (Worker) Nhận được job loại: [%s], retry: %d", job.Type, job.RetryCount)
//	var jobErr error
//
//	jobData := job.Data
//	if jobData == nil {
//		jobData = bson.M{}
//	}
//
//	switch job.Type {
//	case "ticket_mail":
//		regIDRaw, ok := jobData["registration_id"]
//		if !ok {
//			log.Printf("ERROR: Job '%s' thiếu 'registration_id'. Đẩy vào DLQ.", job.Type)
//			return
//		}
//
//		regIDStr, ok := regIDRaw.(string)
//		if !ok || regIDStr == "" {
//			log.Printf("ERROR: Job '%s' có 'registration_id' không phải string. Đẩy vào DLQ.", job.Type)
//			return
//		}
//
//		regisObjectID, err := primitive.ObjectIDFromHex(regIDStr)
//		if err != nil {
//			log.Printf("ERROR: Job '%s' có 'registration_id' không phải ObjectID: %v. Đẩy vào DLQ.", job.Type, err)
//			return
//		}
//		//Gửi lại mail
//		jobErr = service.RetrySendTicket(regisObjectID)
//
//	default:
//		log.Printf("WARN: (Worker) Không biết cách xử lý job loại: [%s]. Đẩy vào DLQ.", job.Type)
//		return
//	}
//
//	if jobErr != nil {
//		log.Printf("WARN: (Worker) Job [%s] thất bại: %v", job.Type, jobErr)
//		//Retry nếu gửi mail thất bại
//		if job.RetryCount < configs.GetMaxRetries() {
//			job.RetryCount++
//			jobBytes, _ := json.Marshal(job)
//			//Đẩy vào lại hàng đợi
//			redisClient.LPush(ctx, "transactional_email_queue", jobBytes)
//		} else {
//			log.Printf("CRITICAL: (Worker) Job [%s] thất bại %d lần", job.Type, configs.GetMaxRetries())
//		}
//	} else {
//		log.Printf("INFO: (Worker) Xử lý job [%s] THÀNH CÔNG.", job.Type)
//	}
//}
//
//func StartEmailQueue() {
//	var (
//		workerName  = "[EmailQueueConsumer]"
//		QueueName   = "transactional_email_queue"
//		redisClient = database.GetRedisClient().Client
//		ctx         = context.Background()
//	)
//	log.Printf("INFO: %s Bắt đầu lắng nghe trên queue '%s'", workerName, QueueName)
//
//	for {
//		result, err := redisClient.BRPop(ctx, 0, QueueName).Result()
//		if err != nil {
//			log.Printf("ERROR: %s (BRPop): %v. Thử lại sau 5s", workerName, err)
//			time.Sleep(5 * time.Second)
//			continue
//		}
//
//		jobPayload := []byte(result[1])
//
//		log.Printf("INFO: %s Nhận được job. Bắt đầu xử lý...", workerName)
//
//		processEmailJob(jobPayload)
//
//		log.Printf("INFO: %s Xử lý xong. Đang chờ job tiếp theo...", workerName)
//	}
//}
