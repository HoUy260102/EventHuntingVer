package dto

import (
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	vietnamPhoneRegex = regexp.MustCompile(`^(0|\+84)(3|5|7|8|9)[0-9]{8}$`)
)

type UpdateAccountReq struct {
	Name          *string       `json:"name"`
	Phone         *string       `json:"phone"`
	Address       *string       `json:"address"`
	UserInfo      *UserDTO      `json:"user_info"`
	OrganizerInfo *OrganizerDTO `json:"organizer_info"`
}

type CreateAccount struct {
	Email    string             `json:"email"`
	Name     string             `json:"name"`
	Password string             `json:"password"`
	RoleId   primitive.ObjectID `json:"role_id"`
	Phone    string             `json:"phone"`

	Address       *string       `json:"address"`
	UserInfo      *UserDTO      `json:"user_info"`
	OrganizerInfo *OrganizerDTO `json:"organizer_info"`
}

type OrganizerDTO struct {
	Decription  *string `json:"decription,omitempty"`
	WebsiteUrl  *string `json:"website_url,omitempty"`
	ContactName *string `json:"contact_name,omitempty"`
}

type UserDTO struct {
	Dob    *time.Time `json:"dob,omitempty"`
	IsMale *bool      `json:"is_male,omitempty"`
}

// LockAccountRequest
type LockAccountRequest struct {
	Message string     `json:"message" binding:"required"`
	Until   *time.Time `json:"until,omitempty"`
}

type CreateUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`

	Phone    *string  `json:"phone"`
	UserInfo *UserDTO `json:"user_info"`
}

type CreateOrganizerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`

	Phone         string        `json:"phone"`
	OrganizerInfo *OrganizerDTO `json:"organizer_info,omitempty"`
}
