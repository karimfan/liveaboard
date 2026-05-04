package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"time"
)

// SMTPSender is the production sender. Connects to Brevo (or any
// STARTTLS-capable SMTP relay) over port 587, authenticates with
// PLAIN, and DATA-streams a multipart message.
type SMTPSender struct {
	Host     string
	Port     int
	Username string
	Password string

	// Timeout is the per-Send wall-clock budget. Default 30s.
	Timeout time.Duration

	// TLSConfig overrides STARTTLS verification settings. Production
	// uses a nil config (system roots, ServerName=Host).
	TLSConfig *tls.Config

	// Dial is injectable for tests. Production uses net.Dialer.
	Dial func(ctx context.Context, network, addr string) (net.Conn, error)
}

// Send delivers msg via the configured SMTP relay.
func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if s.Host == "" {
		return errors.New("email: SMTP host is empty")
	}
	if _, err := mail.ParseAddress(msg.From); err != nil {
		return fmt.Errorf("invalid From: %w", err)
	}
	if _, err := mail.ParseAddress(msg.To); err != nil {
		return fmt.Errorf("invalid To: %w", err)
	}
	body, err := buildMIME(msg)
	if err != nil {
		return err
	}

	timeout := s.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	dial := s.Dial
	if dial == nil {
		d := &net.Dialer{Timeout: timeout}
		dial = d.DialContext
	}
	conn, err := dial(dialCtx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}

	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp newclient: %w", err)
	}
	defer c.Close()

	tlsConfig := s.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{ServerName: s.Host}
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)
	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if err := c.Mail(addressOf(msg.From)); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := c.Rcpt(addressOf(msg.To)); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		_ = w.Close()
		return fmt.Errorf("data write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("data close: %w", err)
	}
	return c.Quit()
}

// addressOf returns just the address part of a "Name <addr@host>"
// header, since SMTP MAIL FROM / RCPT TO take only the address.
func addressOf(s string) string {
	if a, err := mail.ParseAddress(s); err == nil {
		return a.Address
	}
	return s
}
