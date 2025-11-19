package dto

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EventCreateReq struct {
	Name      string `json:"name"`
	EventTime struct {
		StartDate time.Time `json:"start_date"`
		EndDate   time.Time `json:"end_date"`
		StartTime string    `json:"start_time"`
		EndTime   string    `json:"end_time"`
	} `json:"event_time"`
	ThumbnailId      *primitive.ObjectID   `json:"thumbnail_id"`
	EventInfo        string                `json:"event_info"`
	EventInfoHtml    string                `json:"event_info_html"`
	ProvinceID       primitive.ObjectID    `json:"province_id"`
	MediaIDs         *[]primitive.ObjectID `json:"media_ids"`
	MaxParticipants  *int                  `json:"max_participants"`
	Price            int                   `json:"price"`
	MaxTicketPerUser int                   `json:"max_ticket_per_user"`
	EventLocation    struct {
		Name    string `json:"name"`
		Address string `json:"address"`
		MapURL  string `json:"map_url"`
	} `bson:"event_location" json:"event_location"`
	TopicIDs *[]primitive.ObjectID `bson:"topic_ids" json:"topic_ids"`
}

type EventUpdateReq struct {
	Name             *string               `json:"name"`
	EventInfo        *string               `json:"event_info"`
	EventInfoHtml    *string               `json:"event_info_html"`
	Price            *int                  `json:"price"`
	MaxTicketPerUser *int                  `json:"max_ticket_per_user"`
	Active           *bool                 `json:"active"`
	MaxParticipants  *int                  `json:"max_participants"`
	ThumbnailId      *primitive.ObjectID   `json:"thumbnail_id"`
	MediaIDs         *[]primitive.ObjectID `json:"media_ids"`
	TopicIDs         *[]primitive.ObjectID `json:"topic_ids"`
	ProvinceID       *primitive.ObjectID   `json:"province_id"`
	EventTime        *struct {
		StartDate *time.Time `json:"start_date"`
		EndDate   *time.Time `json:"end_date"`
		StartTime *string    `json:"start_time"`
		EndTime   *string    `json:"end_time"`
	} `json:"event_time"`

	EventLocation *struct {
		Name    *string `json:"name"`
		Address *string `json:"address"`
		MapURL  *string `json:"map_url"`
	} `json:"event_location"`
}
