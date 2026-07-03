package mail

import (
	"crypto/tls"
	"fmt"
	"log/slog"

	"github.com/wneessen/go-mail"
)

type Mailer interface {
	Send(to, subject, body string) error
}

type SMTPMailer struct {
	host, user, pass, from string
	port                   int
}

func NewSMTPMailer(host string, port int, user, pass, from string) *SMTPMailer {
	return &SMTPMailer{host: host, port: port, user: user, pass: pass, from: from}
}

func (s *SMTPMailer) Send(to, subject, body string) error {
	m := mail.NewMsg()
	if err := m.From(s.from); err != nil {
		return err
	}
	if err := m.To(to); err != nil {
		return err
	}
	m.Subject(subject)
	m.SetBodyString(mail.TypeTextHTML, body)

	c, err := mail.NewClient(s.host,
		mail.WithPort(s.port),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(s.user),
		mail.WithPassword(s.pass),
		mail.WithTLSConfig(&tls.Config{ServerName: s.host}),
		mail.WithTLSPolicy(mail.TLSMandatory),
	)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	return c.DialAndSend(m)
}

type FakeMailer struct {
	Messages []Message
	Log      bool // when true, Send also writes the message to slog for demo/dev
}

type Message struct {
	To, Subject, Body string
}

func NewFake() *FakeMailer { return &FakeMailer{} }

// NewFakeWithLogging returns a fake mailer that also mirrors sends to slog
// so operators running without real SMTP can still recover magic-link URLs
// from container logs.
func NewFakeWithLogging() *FakeMailer { return &FakeMailer{Log: true} }

func (f *FakeMailer) Send(to, subject, body string) error {
	f.Messages = append(f.Messages, Message{to, subject, body})
	if f.Log {
		slog.Info("fake mailer send", "to", to, "subject", subject, "body", body)
	}
	return nil
}
