package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

// Sprint 010: invitation flow now captures full_name (required) +
// phone (optional) at invite time. The invitee no longer types a name
// at accept time; the email greets them by name.

func TestInviteRequiresFullName(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/invitations", map[string]any{
		"email": "site@x.test",
		"role":  "cruise_director",
	}, cookie)
	if resp.StatusCode != 400 {
		t.Fatalf("missing full_name: %d %v want 400", resp.StatusCode, body)
	}
}

func TestInviteSeedsNameAndPhoneOnAccept(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)

	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/invitations", map[string]any{
		"email":     "maya@example.test",
		"full_name": "Maya Sanchez",
		"phone":     "+1 555 0142",
	}, cookie)
	if resp.StatusCode != 201 {
		t.Fatalf("invite: %d %v", resp.StatusCode, body)
	}

	link := h.mail.LinkFor("maya@example.test", "/invitations/")
	if link == "" {
		t.Fatalf("no invite link captured")
	}
	tok := tokenFromLink(t, link)

	// Lookup exposes full_name; phone is intentionally NOT exposed.
	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/invitations/lookup?token="+tok, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("lookup: %d %v", resp.StatusCode, body)
	}
	if body["full_name"] != "Maya Sanchez" {
		t.Errorf("lookup.full_name = %v", body["full_name"])
	}
	if _, leaked := body["phone"]; leaked {
		t.Errorf("lookup leaked phone: %v", body)
	}

	// Accept (password only) — name + phone come from invitation row.
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/invitations/accept", map[string]any{
		"token":    tok,
		"password": "Sup3rStrong!",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("accept: %d %v", resp.StatusCode, body)
	}
	if body["full_name"] != "Maya Sanchez" {
		t.Errorf("accept.full_name = %v", body["full_name"])
	}
	if body["phone"] != "+1 555 0142" {
		t.Errorf("accept.phone = %v", body["phone"])
	}
	if body["role"] != "cruise_director" {
		t.Errorf("accept.role = %v", body["role"])
	}
}

func TestInvitationEmailGreetsByName(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)

	resp, _ := doJSON(t, c, "POST", h.server.URL+"/api/invitations", map[string]any{
		"email":     "maya@example.test",
		"full_name": "Maya Sanchez",
	}, cookie)
	if resp.StatusCode != 201 {
		t.Fatalf("invite: %d", resp.StatusCode)
	}

	last := h.mail.Last()
	if last.To == "" {
		t.Fatalf("no email captured")
	}
	if !strings.Contains(last.TextBody, "Maya Sanchez") {
		t.Errorf("text body missing recipient name: %q", last.TextBody)
	}
	if !strings.Contains(last.HTMLBody, "Maya Sanchez") {
		t.Errorf("html body missing recipient name")
	}
	if !strings.Contains(last.Subject, "Cruise Director") {
		t.Errorf("subject missing role: %q", last.Subject)
	}
}
