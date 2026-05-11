package service

import (
	"context"
	"fmt"
	"net/smtp"
	"time"

	"github.com/jordan-wright/email"
)

var (
	ErrEmailSendFailed = fmt.Errorf("failed to send email")
)

type SMTPSettings struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}
type NotificationService struct {
	smtpSettings SMTPSettings
}

func NewNotificationService(settings SMTPSettings) *NotificationService {
	return &NotificationService{
		smtpSettings: settings,
	}
}
func (s *NotificationService) SendEmail(ctx context.Context, to, subject, body string) error {
	e := email.NewEmail()
	e.From = s.smtpSettings.From
	e.To = []string{to}
	e.Subject = subject
	e.HTML = []byte(body)
	addr := fmt.Sprintf("%s:%d", s.smtpSettings.Host, s.smtpSettings.Port)
	auth := smtp.PlainAuth("", s.smtpSettings.Username, s.smtpSettings.Password, s.smtpSettings.Host)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	errChan := make(chan error, 1)
	go func() {
		errChan <- e.Send(addr, auth)
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: email sending timed out: %v", ErrEmailSendFailed, ctx.Err())
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("%w: %v", ErrEmailSendFailed, err)
		}
		return nil
	}
}
