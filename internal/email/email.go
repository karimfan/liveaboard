// Package email is the small SMTP-backed sender used for verification,
// invitation, password-reset, and change-email emails. The transport
// is net/smtp with STARTTLS + PLAIN auth; the message format is
// multipart/alternative (text + HTML).
//
// Tests inject a MockSender via the Sender interface to assert
// outbound message contents without hitting a real SMTP server.
// A hand-rolled in-process SMTP listener (fakesmtp_test.go) covers
// the wire format end-to-end.
package email

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Message is the rendered email ready for delivery.
type Message struct {
	From     string
	To       string
	Subject  string
	TextBody string
	HTMLBody string // optional; if empty, send text-only
}

// Sender is the abstraction over the email transport.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// MockSender records messages and never touches the network. Used in
// service tests to assert outbound content (subject, recipient, the
// presence of token URLs in the body).
type MockSender struct {
	mu       sync.Mutex
	Messages []Message
}

func (m *MockSender) Send(_ context.Context, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = append(m.Messages, msg)
	return nil
}

// Reset clears recorded messages between subtests.
func (m *MockSender) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = nil
}

// Last returns the most-recently-recorded message, or zero-value if
// none.
func (m *MockSender) Last() Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Messages) == 0 {
		return Message{}
	}
	return m.Messages[len(m.Messages)-1]
}

// LinkFor scans the most-recent message addressed to `to` whose body
// contains needle, and returns the first http(s) URL on the line that
// contains needle. Tests use it to recover token-bearing URLs without
// re-implementing template parsing. Returns "" if no match.
func (m *MockSender) LinkFor(to, needle string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.Messages) - 1; i >= 0; i-- {
		msg := m.Messages[i]
		if !strings.Contains(strings.ToLower(msg.To), strings.ToLower(to)) {
			continue
		}
		for _, line := range strings.Split(msg.TextBody, "\n") {
			if !strings.Contains(line, needle) {
				continue
			}
			if u := firstURL(line); u != "" {
				return u
			}
		}
	}
	return ""
}

func firstURL(line string) string {
	for _, prefix := range []string{"https://", "http://"} {
		if i := strings.Index(line, prefix); i >= 0 {
			rest := line[i:]
			end := len(rest)
			for j, ch := range rest {
				if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == ')' || ch == '>' {
					end = j
					break
				}
			}
			return rest[:end]
		}
	}
	return ""
}

// Vars are the template variables every email kind shares.
type Vars struct {
	AppName          string
	OrganizationName string
	RecipientEmail   string
	InviterName      string
	ActionURL        string
	ExpiresAt        time.Time
}
