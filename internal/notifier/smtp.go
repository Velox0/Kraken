package notifier

import (
	"fmt"
	"net/smtp"
	"strings"
)

type SMTPProfile struct {
	Host              string
	Port              int
	Username          string
	PasswordEncrypted string
	FromEmail         string
}

type SMTPClient struct{}

func NewSMTPClient() *SMTPClient {
	return &SMTPClient{}
}

func (c *SMTPClient) Send(profile SMTPProfile, recipients []string, subject, body string) error {
	if len(recipients) == 0 {
		return fmt.Errorf("no recipients configured")
	}
	password, err := decryptPassword(profile.PasswordEncrypted)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", profile.Host, profile.Port)
	auth := smtp.PlainAuth("", profile.Username, password, profile.Host)

	msg := strings.Builder{}
	msg.WriteString("From: " + profile.FromEmail + "\r\n")
	msg.WriteString("To: " + strings.Join(recipients, ",") + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return smtp.SendMail(addr, auth, profile.FromEmail, recipients, []byte(msg.String()))
}

func decryptPassword(ciphertext string) (string, error) {
	// Placeholder for KMS-backed or app-key-backed decryption.
	if strings.TrimSpace(ciphertext) == "" {
		return "", fmt.Errorf("empty smtp password")
	}
	return ciphertext, nil
}
