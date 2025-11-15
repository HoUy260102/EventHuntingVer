package consts

type TicketTypeStatus string

const (
	TicketTypeActive   TicketTypeStatus = "active"
	TicketTypeInactive TicketTypeStatus = "inactive"
	TicketTypeCanceled TicketTypeStatus = "canceled"
)
