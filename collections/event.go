package collections

import (
	"EventHunting/consts"
	"EventHunting/database"
	"EventHunting/utils"
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Event struct {
	ID        primitive.ObjectID `bson:"_id" json:"id"`
	Name      string             `bson:"name" json:"name"`
	EventTime struct {
		StartDate time.Time `bson:"start_date" json:"start_date"`
		EndDate   time.Time `bson:"end_date" json:"end_date"`
		StartTime string    `bson:"start_time" json:"start_time"`
		EndTime   string    `bson:"end_time" json:"end_time"`
	} `bson:"event_time" json:"event_time"`
	ThumbnailUrl         string               `bson:"thumbnail_url,omitempty" json:"thumbnail_url,omitempty"`
	ThumbnailId          primitive.ObjectID   `bson:"thumbnail_id,omitempty" json:"thumbnail_id,omitempty"`
	EventInfo            string               `bson:"event_info" json:"event_info"`
	EventInfoHtml        string               `bson:"event_info_html" json:"event_info_html"`
	MediaIDs             []primitive.ObjectID `bson:"media_ids" json:"media_ids"`
	View                 int                  `bson:"view" json:"view"`
	NumberOfParticipants int                  `bson:"number_of_participants" json:"number_of_participants"`

	Active        bool `bson:"active" json:"active"`
	EventLocation struct {
		Name    string `bson:"name" json:"name"`
		Address string `bson:"address" json:"address"`
		MapURL  string `bson:"map_url,omitempty" json:"map_url,omitempty"`
	} `bson:"event_location" json:"event_location"`
	TopicIDs   []primitive.ObjectID `bson:"topic_ids" json:"topic_ids"`
	IsEdit     bool                 `bson:"is_edit" json:"is_edit"`
	ProvinceId primitive.ObjectID   `bson:"province_id" json:"province_id"`

	Status       string      `bson:"-" json:"status,omitempty"`
	Account      Account     `bson:"-" json:"organizer_info,omitempty"`
	Medias       Medias      `bson:"-" json:"medias,omitempty"`
	Topics       Topics      `bson:"-" json:"topics,omitempty"`
	Comments     Comments    `bson:"-" json:"comments,omitempty"`
	CommentCount int         `bson:"-" json:"comment_count,omitempty"`
	Province     Province    `bson:"-" json:"province"`
	TicketTypes  TicketTypes `bson:"-" json:"ticket_types"`

	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	CreatedBy primitive.ObjectID `bson:"created_by" json:"created_by"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
	UpdatedBy primitive.ObjectID `bson:"updated_by" json:"updated_by"`
	DeletedAt time.Time          `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
	DeletedBy primitive.ObjectID `bson:"deleted_by,omitempty" json:"deleted_by,omitempty"`
}

type Events []Event

func (u *Event) getCollectionName() string {
	return "events"
}

func (u *Event) First(ctx context.Context, filter bson.M, opts ...*options.FindOneOptions) error {
	var (
		db  = database.GetDB()
		err error
	)
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	err = db.Collection(u.getCollectionName()).FindOne(ctx, filter, opts...).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Event) Find(ctx context.Context, filter bson.M, opts ...*options.FindOptions) (Events, error) {
	var (
		db     = database.GetDB()
		err    error
		events Events
	)
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}
	cursor, err := db.Collection(u.getCollectionName()).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	if events == nil {
		events = []Event{}
	}

	return events, nil
}

func (u *Event) Create(ctx context.Context) error {
	var (
		db  = database.GetDB()
		err error
	)
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	if u.ID.IsZero() {
		u.ID = primitive.NewObjectID()
	}
	_, err = db.Collection(u.getCollectionName()).InsertOne(ctx, u)

	if err != nil {
		return err
	}
	return nil
}

func (u *Event) Update(ctx context.Context, filter bson.M, updateDoc bson.M, opts ...*options.UpdateOptions) error {
	var (
		db = database.GetDB()
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	if filter == nil {
		filter = bson.M{}
	}

	res, err := db.Collection(u.getCollectionName()).UpdateOne(ctx, filter, updateDoc, opts...)
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (u *Event) DeleteMany(ctx context.Context, filter bson.M) error {
	var (
		db  = database.GetDB()
		err error
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	_, err = db.Collection(u.getCollectionName()).DeleteMany(ctx, filter)
	if err != nil {
		return err
	}

	return nil
}

func (u *Event) CountDocuments(ctx context.Context, filter bson.M) (int64, error) {
	var (
		db = database.GetDB()
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	count, err := db.Collection(u.getCollectionName()).CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (u *Event) GetView() int {
	redisClient := database.GetRedisClient().Client
	coldCount := u.View
	var hotCount int

	if redisClient == nil {
		log.Println("Lỗi do redis nil!")
		return coldCount
	}

	redisKey := "views:event:" + u.ID.Hex()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	hotViewStr, err := redisClient.Get(ctx, redisKey).Result()

	if err == redis.Nil {
		hotCount = 0
	} else if err != nil {
		log.Printf("Lỗi do redis %s: %v", u.ID.Hex(), err)
		return coldCount
	} else {
		hotCount, err = strconv.Atoi(hotViewStr)
		if err != nil {
			log.Printf("Corrupt view count in Redis for event %s (value: %s): %v", u.ID.Hex(), hotViewStr, err)
			hotCount = 0
		}
	}

	return hotCount + coldCount
}

func getEventViewRedisKey(eventID primitive.ObjectID) string {
	return "views:event:" + eventID.Hex()
}

func getEventUniqueKey(eventID primitive.ObjectID) string {
	today := time.Now().UTC().Format("2006-01-02")
	return "unique_views:event:" + eventID.Hex() + ":" + today
}

func (u *Event) IncrementEventView(accountID string) {
	redisClient := database.GetRedisClient().Client
	if redisClient == nil {
		log.Println("Redis client nil")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	uniqueKey := getEventUniqueKey(u.ID)
	// Thêm account vào Set.
	added, err := redisClient.SAdd(ctx, uniqueKey, accountID).Result()
	if err != nil {
		log.Printf("Lỗi khi add và sadd view trong event %s: %v", u.ID.Hex(), err)
		return
	}

	redisClient.Expire(ctx, uniqueKey, 25*time.Hour)

	if added == 1 {
		err := redisClient.Incr(ctx, getEventViewRedisKey(u.ID)).Err()
		if err != nil {
			log.Printf("Lỗi khi tăng view blog redis %s: %v", u.ID.Hex(), err)
		}
	}
}

func (u *Event) Preload(entries Events, properties ...string) error {
	for _, property := range properties {
		switch property {
		case "AccountFirst":
			var (
				filter = bson.M{
					"_id":        u.CreatedBy,
					"deleted_at": nil,
				}
				err error
			)
			err = u.Account.First(filter)
			if err != nil {
				if err != mongo.ErrNoDocuments {
					return err
				}
			}
		case "AccountFind":
			var (
				accountMap map[primitive.ObjectID]Account
			)
			accountMap = make(map[primitive.ObjectID]Account)
			accountIDs := utils.ExtractUniqueIDs[Event](entries, func(event Event) []primitive.ObjectID {
				result := []primitive.ObjectID{
					event.CreatedBy,
				}
				return result
			})

			if len(accountIDs) == 0 {
				continue
			}

			filterAccount := bson.M{
				"_id": bson.M{
					"$in": accountIDs,
				},
			}

			accounts, err := u.Account.Find(filterAccount)
			if err != nil {
				return err
			}

			for _, a := range accounts {
				accountMap[a.ID] = a
			}
			for i := range entries {
				(entries)[i].Account = accountMap[(entries)[i].CreatedBy]
			}

		case "TopicFirst":
			if len(u.TopicIDs) == 0 {
				continue
			}

			topicEntry := &Topic{}
			topics, err := topicEntry.Find(bson.M{"_id": bson.M{"$in": u.TopicIDs}})
			if err != nil {
				return err
			}
			u.Topics = topics

		case "TopicFind":
			topicIDs := utils.ExtractUniqueIDs[Event](entries, func(event Event) []primitive.ObjectID {
				return event.TopicIDs
			})

			if len(topicIDs) == 0 {
				continue
			}
			var (
				topicEntry = &Topic{}
				topicsMap  = make(map[primitive.ObjectID]Topic)
			)
			topics, err := topicEntry.Find(bson.M{"_id": bson.M{"$in": topicIDs}})
			if err != nil {
				return err
			}
			for _, topic := range topics {
				topicsMap[topic.ID] = topic
			}
			for i := range entries {
				topicsRe := []Topic{}
				for _, topic := range (entries)[i].TopicIDs {
					topicsRe = append(topicsRe, topicsMap[topic])
				}
				(entries)[i].Topics = topicsRe
			}
		case "CommentCountFirst":
			var (
				commentEntry = &Comment{}
				filter       = bson.M{
					"document_id": u.ID,
					"deleted_at": bson.M{
						"$exists": false,
					},
				}
			)
			cmtCount, err := commentEntry.CountDocument(filter)
			if err != nil {
				return err
			}
			u.CommentCount = int(cmtCount)
		case "CommentCountFind":
			var (
				eventIds         = make([]primitive.ObjectID, 0)
				commentEntry     = &Comment{}
				eventCountCmtMap = make(map[primitive.ObjectID]int32)
			)
			//Lấy danh sách id từ event entries
			for _, event := range entries {
				eventIds = append(eventIds, event.ID)
			}
			if len(eventIds) == 0 {
				continue
			}
			//Lấy count cmt từ mỗi event
			pipeline := []bson.M{
				{
					"$match": bson.M{
						"document_id": bson.M{"$in": eventIds},
						"deleted_at": bson.M{
							"$exists": false,
						},
					},
				},
				{
					"$group": bson.M{
						"_id":           "$document_id",
						"comment_count": bson.M{"$sum": 1},
					},
				},
			}
			eventCountCmt, err := commentEntry.Aggregation(pipeline)
			if err != nil {
				return err
			}
			for _, b := range eventCountCmt {
				id, ok1 := b["_id"].(primitive.ObjectID)
				cnt, ok2 := b["comment_count"].(int32)
				if ok1 && ok2 {
					eventCountCmtMap[id] = cnt
				}
			}
			for i := range entries {
				if cnt, ok := eventCountCmtMap[entries[i].ID]; ok {
					entries[i].CommentCount = int(cnt)
				} else {
					entries[i].CommentCount = 0
				}
			}

		case "MediaFirst":
			if len(u.MediaIDs) == 0 {
				continue
			}
			var (
				mediaEntry  = &Media{}
				mediaFilter = bson.M{
					"_id": bson.M{
						"$in": u.MediaIDs,
					},
				}
			)
			medias, err := mediaEntry.Find(nil, mediaFilter)
			if err != nil {
				return err
			}
			u.Medias = medias
		case "MediaFind":
			mediaIds := utils.ExtractUniqueIDs[Event](entries, func(event Event) []primitive.ObjectID {
				return event.MediaIDs
			})

			if len(mediaIds) == 0 {
				continue
			}
			var (
				mediaEntry = &Media{}
				mediasMap  = make(map[primitive.ObjectID]Media)
			)
			medias, err := mediaEntry.Find(nil, bson.M{"_id": bson.M{"$in": mediaIds}})
			if err != nil {
				return err
			}
			for _, media := range medias {
				mediasMap[media.ID] = media
			}
			for i := range entries {
				mediasRe := []Media{}
				for _, mediaID := range (entries)[i].MediaIDs {
					mediasRe = append(mediasRe, mediasMap[mediaID])
				}
				(entries)[i].Medias = mediasRe
			}
		case "CommentFirst":
			var (
				commentEntry  = &Comment{}
				commentFilter = bson.M{
					"document_id": u.ID,
					"parent_id": bson.M{
						"$exists": false,
					},
					"deleted_at": bson.M{
						"$exists": false,
					},
				}
			)
			comments, err := commentEntry.Find(commentFilter)
			if err != nil {
				return err
			}
			err = commentEntry.Preload(u.Comments, "AccountFind", "MediaFind")
			if err != nil {
				return err
			}
			if len(comments) > 0 {
				u.Comments = comments
			}
		case "ProvinceFirst":
			var (
				provinceFilter = bson.M{
					"_id": u.ProvinceId,
				}
				err error
			)
			err = u.Province.First(nil, provinceFilter)
			if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
				return err
			}
		case "ProvinceFind":
			var (
				provinceMap map[primitive.ObjectID]Province
			)
			provinceMap = make(map[primitive.ObjectID]Province)
			provinceIDs := utils.ExtractUniqueIDs[Event](entries, func(event Event) []primitive.ObjectID {
				result := []primitive.ObjectID{
					event.ProvinceId,
				}
				return result
			})

			if len(provinceIDs) == 0 {
				continue
			}

			filterProvince := bson.M{
				"_id": bson.M{
					"$in": provinceIDs,
				},
			}

			provinces, err := u.Province.Find(nil, filterProvince)
			if err != nil {
				return err
			}

			for _, a := range provinces {
				provinceMap[a.ID] = a
			}

			for i := range entries {
				(entries)[i].Province = provinceMap[(entries)[i].ProvinceId]
			}
		case "TicketTypeFirst":
			var (
				ticketTypeEntry = &TicketType{}
				filter          = bson.M{
					"event_id": u.ID,
					"deleted_at": bson.M{
						"$exists": false,
					},
				}
			)
			numberOfParticipants := 0
			ticketTypes, err := ticketTypeEntry.Find(nil, filter)
			for _, ticketType := range ticketTypes {
				numberOfParticipants += ticketType.RegisteredCount
			}
			if err != nil {
				return err
			}
			u.TicketTypes = ticketTypes
			u.NumberOfParticipants = numberOfParticipants
		}
	}
	return nil
}

func (u *Event) ParseEntry() bson.M {
	var eventStatus string

	result := bson.M{
		"_id":  u.ID,
		"name": u.Name,
		"event_time": bson.M{
			"start_date": u.EventTime.StartDate,
			"end_date":   u.EventTime.EndDate,
			"start_time": u.EventTime.StartTime,
			"end_time":   u.EventTime.EndTime,
		},
		"thumbnail_url":          u.ThumbnailUrl,
		"thumbnail_id":           u.ThumbnailId,
		"event_info":             u.EventInfo,
		"event_info_html":        u.EventInfoHtml,
		"media_ids":              u.MediaIDs,
		"view":                   u.GetView(),
		"number_of_participants": u.NumberOfParticipants,
		"active":                 u.Active,
		"event_location": bson.M{
			"name":    u.EventLocation.Name,
			"address": u.EventLocation.Address,
			"map_url": u.EventLocation.MapURL,
		},
		"ticket_types":  u.TicketTypes,
		"province":      u.Province,
		"topic_ids":     u.TopicIDs,
		"comment_count": u.CommentCount,
		"created_at":    u.CreatedAt,
		"created_by":    u.CreatedBy,
		"updated_at":    u.UpdatedAt,
		"updated_by":    u.UpdatedBy,
		"deleted_at":    u.DeletedAt,
		"deleted_by":    u.DeletedBy,
	}

	//Xử lý event status
	if !u.Active {
		eventStatus = consts.EventStatusCancelled
	} else if time.Now().Before(u.EventTime.StartDate) {
		eventStatus = consts.EventStatusUpcoming
	} else if time.Now().After(u.EventTime.EndDate) {
		eventStatus = consts.EventStatusEnded
	} else {
		eventStatus = consts.EventStatusOngoing
	}
	result["event_status"] = eventStatus

	//Xủ lý account
	if u.Account.ID != primitive.NilObjectID {
		result["account"] = bson.M{
			"name":       u.Account.Name,
			"avatar_url": u.Account.AvatarUrl,
		}
	}

	//Xử lý topic
	if len(u.Topics) > 0 {
		topics := make([]bson.M, 0, len(u.Topics))
		for _, topic := range u.Topics {
			topics = append(topics, bson.M{
				"name": topic.Name,
				"slug": topic.Slug,
			})
		}
		result["topics"] = topics
	}

	//Xử lý medias
	if len(u.Medias) > 0 {
		medias := []bson.M{}
		for _, media := range u.Medias {
			medias = append(medias, media.ParseEntry())
		}
		result["medias"] = medias
	}

	if len(u.Comments) > 0 {
		comments := []bson.M{}
		for _, comment := range u.Comments {
			comments = append(comments, comment.ParseEntry())
		}
		result["comments"] = comments
	}

	return result
}
