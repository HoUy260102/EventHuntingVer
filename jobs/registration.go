package jobs

import (
	"EventHunting/collections"
	"EventHunting/configs"
	"EventHunting/consts"
	"EventHunting/database"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func HandleExpiredRegistrations() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	expirationTime := time.Now().Add(-time.Duration(configs.GetRegisExpirationMinutes()) * time.Minute)

	regisEntry := collections.Registration{}

	filter := bson.M{
		"status":     consts.RegistrationPending,
		"created_at": bson.M{"$lt": expirationTime},
	}

	expiredRegs, err := regisEntry.Find(ctx, filter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return
		}
		log.Println("CRON JOB:(registration) Lỗi do hệ thống!", err)
		return
	}

	if len(expiredRegs) == 0 {
		log.Println("CRON JOB: Không tìm thấy các đăng ký hết hạn!.")
		return
	}

	successCount := 0
	failCount := 0

	for _, reg := range expiredRegs {
		err := runPedingRegistrationTransaction(ctx, reg)

		if err != nil {
			log.Printf("CRON JOB thất bại (RegID: %s): %v", reg.ID.Hex(), err)
			failCount++
		} else {
			log.Printf("CRON JOB thành công (RegID: %s)", reg.ID.Hex())
			successCount++
		}
	}
}

func runPedingRegistrationTransaction(ctx context.Context, reg collections.Registration) error {
	var (
		db              = database.GetDB()
		err             error
		regisEntry      = collections.Registration{}
		ticketTypeEntry = collections.TicketType{}
		eventEntry      = collections.Event{}
	)
	session, err := db.Client().StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessionContext mongo.SessionContext) (interface{}, error) {

		regFilter := bson.M{"_id": reg.ID, "status": consts.RegistrationPending}
		regUpdate := bson.M{"$set": bson.M{
			"status":       consts.RegistrationCancelled,
			"cancelled_at": time.Now(),
		}}

		err = regisEntry.Update(sessionContext, regFilter, regUpdate)
		if err != nil {
			return nil, err
		}

		// Trả vé về kho
		for _, ticket := range reg.Tickets {
			err = ticketTypeEntry.Update(sessionContext,
				bson.M{"_id": ticket.TicketTypeID},
				bson.M{"$inc": bson.M{"registered_count": -ticket.Quantity}},
			)
			if err != nil {
				return nil, fmt.Errorf("failed to return TicketType stock: %v", err)
			}
		}

		//Giảm số người tham gia
		err = eventEntry.Update(sessionContext,
			bson.M{"_id": reg.EventID},
			bson.M{"$inc": bson.M{"number_of_participants": -reg.TotalQuantity}},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to return Event participants: %v", err)
		}

		return nil, nil
	})

	return err
}
