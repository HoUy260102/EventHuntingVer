package utils

import (
	"EventHunting/configs"
	"io"
	"log"
	"strconv"

	"gopkg.in/gomail.v2"
)

type EmailService struct {
	dialer *gomail.Dialer
	from   string
}

func NewEmailService() *EmailService {
	host := configs.GetSMTPHost()
	port := configs.GetSMTPPort()
	senderEmail := configs.GetSenderEmail()
	appPassword := configs.GetAppPassword()
	portConvert, _ := strconv.Atoi(port)
	d := gomail.NewDialer(host, portConvert, senderEmail, appPassword)

	return &EmailService{
		dialer: d,
		from:   senderEmail,
	}
}

type EmailPayload struct {
	To       []string
	Subject  string
	HTMLBody string

	EmbeddedImages map[string][]byte
}

func (s *EmailService) SendEmail(payload EmailPayload) error {
	m := gomail.NewMessage()
	m.SetHeader("From", s.from)

	recipients := make([]string, len(payload.To))
	for i, email := range payload.To {
		recipients[i] = email
	}
	m.SetHeader("To", recipients...)

	m.SetHeader("Subject", payload.Subject)
	m.SetBody("text/html", payload.HTMLBody)

	if payload.EmbeddedImages != nil {
		for cid, data := range payload.EmbeddedImages {
			m.Embed(cid, gomail.SetCopyFunc(func(w io.Writer) error {
				_, err := w.Write(data)
				return err
			}))
		}
	}

	if err := s.dialer.DialAndSend(m); err != nil {
		log.Printf("LỖI GỬI MAIL (tới %v): %v", payload.To, err)
		return err
	}

	log.Printf("Đã chuẩn bị mail (gomail) thành công tới: %v", payload.To)
	return nil
}
