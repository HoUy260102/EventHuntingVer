package utils

import (
	"EventHunting/configs"

	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/google"
)

func InitOAuth() {
	goth.UseProviders(
		google.New(
			configs.GetGoogleClientID(),
			configs.GetGoogleSecret(),
			configs.GetGoogleRedirectURL(),
			"email",
			"profile",
		),
	)
}
