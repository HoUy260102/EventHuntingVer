package dto

import "go.mongodb.org/mongo-driver/bson/primitive"

type CreateRegistrationEventRequest struct {
	Tickets []struct {
		TicketTypeID primitive.ObjectID `json:"ticket_type_id"`
		Quantity     int                `json:"quantity"`
	} `json:"tickets"`
}
