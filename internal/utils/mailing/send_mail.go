package mailing

import (
	"Go-Starter-Template/internal/utils"
	"gopkg.in/gomail.v2"
	"strconv"
)

type MailConfig struct {
	AppURL       string
	SMTPHost     string
	SMTPPort     string
	SMTPSender   string
	SMTPEmail    string
	SMTPPassword string
}

func LoadMailConfig() MailConfig {
	return MailConfig{
		AppURL:       utils.GetConfig("APP_URL"),
		SMTPHost:     utils.GetConfig("SMTP_HOST"),
		SMTPPort:     utils.GetConfig("SMTP_PORT"),
		SMTPSender:   utils.GetConfig("SMTP_SENDER_NAME"),
		SMTPEmail:    utils.GetConfig("SMTP_AUTH_EMAIL"),
		SMTPPassword: utils.GetConfig("SMTP_AUTH_PASSWORD"),
	}
}

func SendMail(toEmail string, subject string, body string) error {
	emailConfig := LoadMailConfig()

	mailer := gomail.NewMessage()
	mailer.SetHeader("From", emailConfig.SMTPEmail)
	mailer.SetHeader("To", toEmail)
	mailer.SetHeader("Subject", subject)
	mailer.SetBody("text/html", body)
	port, err := strconv.Atoi(emailConfig.SMTPPort)
	if err != nil {
		return err
	}
	dialer := gomail.NewDialer(
		emailConfig.SMTPHost,
		port,
		emailConfig.SMTPEmail,
		emailConfig.SMTPPassword,
	)

	err = dialer.DialAndSend(mailer)
	if err != nil {
		return err
	}

	return nil
}
