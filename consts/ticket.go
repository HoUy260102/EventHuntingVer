package consts

type TicketStatus string

const (
	TicketStatusConfirmed TicketStatus = "confirmed"
	TicketStatusCheckedIn TicketStatus = "checked_in"
	TicketStatusCancelled TicketStatus = "cancelled"
	TicketStatusRefunded  TicketStatus = "refunded"
)
