package consts

type EventRegistrationStatus string

const (
	RegistrationPending   EventRegistrationStatus = "PENDING"
	RegistrationPaid      EventRegistrationStatus = "PAID"
	RegistrationCancelled EventRegistrationStatus = "CANCELLED"
)
