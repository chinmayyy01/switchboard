package mailer

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"os"
	"strings"
)

type Sender interface {
	Send(ctx context.Context, to, subject, body string) error
}

type LogSender struct{}

func (LogSender) Send(_ context.Context, to, subject, body string) error {
	slog.Info("development email", "to", to, "subject", subject, "body", body)
	return nil
}

type SMTPSender struct {
	host string
	port string
	user string
	pass string
	from string
}

func (s *SMTPSender) Send(_ context.Context, to, subject, body string) error {
	headers := []string{
		"From: " + s.from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
	}
	message := []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body + "\r\n")
	var auth smtp.Auth
	if s.user != "" {
		auth = smtp.PlainAuth("", s.user, s.pass, s.host)
	}
	return smtp.SendMail(s.host+":"+s.port, auth, s.from, []string{to}, message)
}

func NewFromEnv() Sender {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	if host == "" {
		return LogSender{}
	}
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if port == "" {
		port = "587"
	}
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	if from == "" {
		from = "no-reply@switchboard.local"
	}
	return &SMTPSender{
		host: host,
		port: port,
		user: strings.TrimSpace(os.Getenv("SMTP_USER")),
		pass: os.Getenv("SMTP_PASSWORD"),
		from: from,
	}
}

func Configured() bool { return strings.TrimSpace(os.Getenv("SMTP_HOST")) != "" }

func BaseURL() string {
	for _, name := range []string{"APP_BASE_URL", "NEXT_PUBLIC_SITE_URL"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return strings.TrimRight(value, "/")
		}
	}
	return "http://localhost:3000"
}

func Link(path, token string) string {
	base := BaseURL()
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("%s%s?token=%s", base, path, token)
}
