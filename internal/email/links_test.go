package email_test

import (
	"context"
	"testing"

	"github.com/karimfan/liveaboard/internal/email"
)

func TestExtractLinks(t *testing.T) {
	cases := []struct {
		name string
		msg  email.Message
		want []string
	}{
		{
			name: "text only",
			msg:  email.Message{TextBody: "Open http://example.test/x?t=1 in your browser."},
			want: []string{"http://example.test/x?t=1"},
		},
		{
			name: "https and http both present",
			msg:  email.Message{TextBody: "https://a.test/1 then http://b.test/2"},
			want: []string{"https://a.test/1", "http://b.test/2"},
		},
		{
			name: "dedupe across text and html",
			msg: email.Message{
				TextBody: "Visit http://x.test/verify?t=K to confirm.",
				HTMLBody: `<a href="http://x.test/verify?t=K">click</a>`,
			},
			want: []string{"http://x.test/verify?t=K"},
		},
		{
			name: "html only — terminates at double quote",
			msg: email.Message{
				HTMLBody: `<a href="https://x.test/path?q=1">click</a>`,
			},
			want: []string{"https://x.test/path?q=1"},
		},
		{
			name: "url in parens",
			msg:  email.Message{TextBody: "Visit (http://x.test/a) please."},
			want: []string{"http://x.test/a"},
		},
		{
			name: "no urls",
			msg:  email.Message{TextBody: "no links here", HTMLBody: "<p>none</p>"},
			want: []string{},
		},
		{
			name: "multiple distinct urls in one body",
			msg:  email.Message{TextBody: "first http://a.test then http://b.test"},
			want: []string{"http://a.test", "http://b.test"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := email.ExtractLinks(c.msg)
			if !equalSlice(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// TestMockSenderLinkForUsesExtractLinks confirms the refactored LinkFor
// still finds token URLs across the body kinds existing tests rely on.
func TestMockSenderLinkForUsesExtractLinks(t *testing.T) {
	m := &email.MockSender{}
	send := func(to, subj, text, html string) {
		_ = m.Send(context.Background(), email.Message{
			From: "n@x.test", To: to, Subject: subj, TextBody: text, HTMLBody: html,
		})
	}
	send("owner@x.test", "v", "Visit http://x.test/verify-email?token=K",
		`<a href="http://x.test/verify-email?token=K">v</a>`)
	send("owner@x.test", "r", "Visit http://x.test/reset-password?token=R",
		`<a href="http://x.test/reset-password?token=R">r</a>`)
	send("other@x.test", "unused", "http://x.test/elsewhere", "")

	if got := m.LinkFor("owner@x.test", "reset-password"); got != "http://x.test/reset-password?token=R" {
		t.Errorf("reset: got %q", got)
	}
	if got := m.LinkFor("owner@x.test", "verify-email"); got != "http://x.test/verify-email?token=K" {
		t.Errorf("verify: got %q", got)
	}
	if got := m.LinkFor("owner@x.test", "no-such"); got != "" {
		t.Errorf("missing needle: got %q", got)
	}
	if got := m.LinkFor("nobody@x.test", "verify-email"); got != "" {
		t.Errorf("unknown recipient: got %q", got)
	}
}
