package dto

import (
	"EventHunting/consts"
)

type CreateTicketType struct {
	EventID     string                  `json:"event_id"`
	Name        string                  `json:"name"`
	Description *string                 `json:"description,omitempty"`
	Price       int                     `json:"price"`
	Quantity    *int                    `json:"quantity,omitempty"` // nil là unlimited
	Status      consts.TicketTypeStatus `json:"status"`
}

// UpdateTicketTypePayload chứa dữ liệu để cập nhật loại vé
type UpdateTicketType struct {
	Name         *string                  `json:"name,omitempty"`
	Description  *string                  `json:"description,omitempty"`
	Price        *int                     `json:"price,omitempty"`
	Quantity     *int                     `json:"quantity,omitempty"`
	SetUnlimited *bool                    `json:"set_unlimited,omitempty"` // Set thành true để làm cho quantity = nil
	Status       *consts.TicketTypeStatus `json:"status,omitempty"`
}
