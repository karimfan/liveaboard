package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	svix "github.com/svix/svix-webhooks/go"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

const testWebhookSecret = "whsec_C2FVsBE8+CIqwLrHLMyD6gtVsh5TfEKJ"

func newReceiver(t *testing.T, pool *store.Pool, p auth.Provider) *auth.WebhookReceiver {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	r, err := auth.NewWebhookReceiver(p, pool, log, testWebhookSecret)
	if err != nil {
		t.Fatalf("NewWebhookReceiver: %v", err)
	}
	return r
}

// signedRequest builds a webhook request with a valid svix signature
// using the test secret. msgID is the svix-id header value (used for
// idempotency); pass distinct values across tests to avoid colliding on
// the webhook_events PRIMARY KEY.
func signedRequest(t *testing.T, msgID string, payload []byte) *http.Request {
	t.Helper()
	wh, err := svix.NewWebhook(testWebhookSecret)
	if err != nil {
		t.Fatalf("svix.NewWebhook: %v", err)
	}
	now := time.Now()
	sig, err := wh.Sign(msgID, now, payload)
	if err != nil {
		t.Fatalf("svix.Sign: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
	req.Header.Set("svix-id", msgID)
	req.Header.Set("svix-timestamp", fmt.Sprintf("%d", now.Unix()))
	req.Header.Set("svix-signature", sig)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func envelope(t *testing.T, eventType string, data any) []byte {
	t.Helper()
	dataRaw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	out, err := json.Marshal(map[string]any{
		"type":      eventType,
		"object":    "event",
		"data":      json.RawMessage(dataRaw),
		"timestamp": time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return out
}

func TestWebhookRejectsInvalidSignature(t *testing.T) {
	pool := testdb.Pool(t)
	rcv := newReceiver(t, pool, auth.NewStubProvider())

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"type":"user.created"}`))
	req.Header.Set("svix-id", "evt_test_invalid")
	req.Header.Set("svix-timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	req.Header.Set("svix-signature", "v1,bogus")

	rec := httptest.NewRecorder()
	rcv.Handle(rec, req)
	if rec.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d want 401", rec.Result().StatusCode)
	}
}

func TestWebhookRejectsMissingEventID(t *testing.T) {
	pool := testdb.Pool(t)
	rcv := newReceiver(t, pool, auth.NewStubProvider())

	payload := envelope(t, "user.created", map[string]any{"id": "user_x"})
	wh, _ := svix.NewWebhook(testWebhookSecret)
	now := time.Now()
	sig, _ := wh.Sign("evt_signed", now, payload)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
	// Sign with one id, send with another header missing.
	req.Header.Set("svix-timestamp", fmt.Sprintf("%d", now.Unix()))
	req.Header.Set("svix-signature", sig)
	// Don't set svix-id at all -> verification must fail (svix needs all 3).
	rec := httptest.NewRecorder()
	rcv.Handle(rec, req)
	if rec.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d want 401", rec.Result().StatusCode)
	}
}

func TestWebhookIdempotentReplay(t *testing.T) {
	pool := testdb.Pool(t)
	rcv := newReceiver(t, pool, auth.NewStubProvider())

	payload := envelope(t, "user.updated", map[string]any{
		"id":         "user_idempotent",
		"first_name": "X",
	})

	for i := 0; i < 2; i++ {
		req := signedRequest(t, "evt_idempotent", payload)
		rec := httptest.NewRecorder()
		rcv.Handle(rec, req)
		if rec.Result().StatusCode != http.StatusOK {
			t.Fatalf("attempt %d status %d", i, rec.Result().StatusCode)
		}
	}
}

func TestWebhookOrganizationUpdatedSyncsName(t *testing.T) {
	pool := testdb.Pool(t)
	rcv := newReceiver(t, pool, auth.NewStubProvider())

	if _, _, err := pool.CreateExternalOrgAndAdmin(context.Background(),
		"Old Name", "org_clerk_w", "user_clerk_w", "a@x.test", "Alice"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	payload := envelope(t, "organization.updated", map[string]any{
		"id":   "org_clerk_w",
		"name": "Renamed",
	})
	req := signedRequest(t, "evt_org_updated", payload)
	rec := httptest.NewRecorder()
	rcv.Handle(rec, req)
	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status %d", rec.Result().StatusCode)
	}

	got, err := pool.OrganizationByClerkID(context.Background(), "org_clerk_w")
	if err != nil {
		t.Fatalf("OrganizationByClerkID: %v", err)
	}
	if got.Name != "Renamed" {
		t.Errorf("name = %q want Renamed", got.Name)
	}
}

func TestWebhookUserUpdatedSyncsIdentity(t *testing.T) {
	pool := testdb.Pool(t)
	rcv := newReceiver(t, pool, auth.NewStubProvider())

	if _, _, err := pool.CreateExternalOrgAndAdmin(context.Background(),
		"Org", "org_a", "user_a", "old@x.test", "Old Name"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	payload := envelope(t, "user.updated", map[string]any{
		"id":                       "user_a",
		"first_name":               "New",
		"last_name":                "Name",
		"primary_email_address_id": "ema_1",
		"email_addresses": []map[string]any{
			{"id": "ema_1", "email_address": "new@x.test"},
		},
	})
	req := signedRequest(t, "evt_user_updated", payload)
	rec := httptest.NewRecorder()
	rcv.Handle(rec, req)
	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status %d", rec.Result().StatusCode)
	}

	got, err := pool.UserByClerkID(context.Background(), "user_a")
	if err != nil {
		t.Fatalf("UserByClerkID: %v", err)
	}
	if got.Email != "new@x.test" || got.FullName != "New Name" {
		t.Errorf("got %+v", got)
	}
}

func TestWebhookMembershipCreatedHandlesInvitationAcceptance(t *testing.T) {
	pool := testdb.Pool(t)
	stub := auth.NewStubProvider()
	rcv := newReceiver(t, pool, stub)

	// Bootstrap the org admin (and thus the local organization).
	if _, _, err := pool.CreateExternalOrgAndAdmin(context.Background(),
		"Org", "org_a", "user_admin", "admin@x.test", "Admin"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Site Director accepts an invite -> Clerk fires this event.
	payload := envelope(t, "organizationMembership.created", map[string]any{
		"id":   "orgmem_1",
		"role": "site_director",
		"organization": map[string]any{
			"id":   "org_a",
			"name": "Org",
		},
		"public_user_data": map[string]any{
			"user_id":         "user_invitee",
			"first_name":      "Site",
			"last_name":       "Director",
			"identifier_type": "email_address",
			"identifier":      "site@x.test",
		},
	})
	req := signedRequest(t, "evt_membership_created", payload)
	rec := httptest.NewRecorder()
	rcv.Handle(rec, req)
	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status %d", rec.Result().StatusCode)
	}

	got, err := pool.UserByClerkID(context.Background(), "user_invitee")
	if err != nil {
		t.Fatalf("UserByClerkID: %v", err)
	}
	if got.Role != store.RoleSiteDirector {
		t.Errorf("role = %q", got.Role)
	}
	if got.Email != "site@x.test" || got.FullName != "Site Director" {
		t.Errorf("identity = %+v", got)
	}
}

func TestWebhookMembershipCreatedNoOpWhenLocalUserExists(t *testing.T) {
	pool := testdb.Pool(t)
	rcv := newReceiver(t, pool, auth.NewStubProvider())

	// Org admin signs up via /api/signup-complete (already linked).
	if _, _, err := pool.CreateExternalOrgAndAdmin(context.Background(),
		"Org", "org_a", "user_admin", "admin@x.test", "Admin"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Subsequent membership.created webhook for the same user should be a no-op.
	payload := envelope(t, "organizationMembership.created", map[string]any{
		"id":           "orgmem_1",
		"role":         "org_admin",
		"organization": map[string]any{"id": "org_a", "name": "Org"},
		"public_user_data": map[string]any{
			"user_id":    "user_admin",
			"identifier": "admin@x.test",
		},
	})
	req := signedRequest(t, "evt_membership_dup", payload)
	rec := httptest.NewRecorder()
	rcv.Handle(rec, req)
	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status %d", rec.Result().StatusCode)
	}

	// Still exactly one user row for clerk_user_id user_admin.
	got, err := pool.UserByClerkID(context.Background(), "user_admin")
	if err != nil {
		t.Fatalf("UserByClerkID: %v", err)
	}
	if got.Role != store.RoleOrgAdmin {
		t.Errorf("role drifted to %q", got.Role)
	}
}

func TestWebhookMembershipDeletedDeactivates(t *testing.T) {
	pool := testdb.Pool(t)
	rcv := newReceiver(t, pool, auth.NewStubProvider())

	_, user, err := pool.CreateExternalOrgAndAdmin(context.Background(),
		"Org", "org_a", "user_a", "a@x.test", "A")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	payload := envelope(t, "organizationMembership.deleted", map[string]any{
		"organization":     map[string]any{"id": "org_a"},
		"public_user_data": map[string]any{"user_id": "user_a"},
	})
	req := signedRequest(t, "evt_membership_deleted", payload)
	rec := httptest.NewRecorder()
	rcv.Handle(rec, req)
	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status %d", rec.Result().StatusCode)
	}

	got, err := pool.UserByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	if got.IsActive {
		t.Errorf("user should be deactivated")
	}
}

func TestWebhookUserDeletedDeactivatesAndRevokesSessions(t *testing.T) {
	pool := testdb.Pool(t)
	rcv := newReceiver(t, pool, auth.NewStubProvider())

	_, user, err := pool.CreateExternalOrgAndAdmin(context.Background(),
		"Org", "org_a", "user_a", "a@x.test", "A")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := pool.CreateAppSession(context.Background(), user.ID,
		auth.HashCookieToken("tk"), "user_a", "sess_a", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateAppSession: %v", err)
	}

	payload := envelope(t, "user.deleted", map[string]any{
		"id":      "user_a",
		"object":  "user",
		"deleted": true,
	})
	req := signedRequest(t, "evt_user_deleted", payload)
	rec := httptest.NewRecorder()
	rcv.Handle(rec, req)
	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status %d", rec.Result().StatusCode)
	}

	got, err := pool.UserByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	if got.IsActive {
		t.Errorf("user should be deactivated")
	}
	if _, err := pool.AppSessionByTokenHash(context.Background(),
		auth.HashCookieToken("tk"), time.Now()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("app_session should be gone")
	}
}

func TestWebhookUnknownEventTypeIs200(t *testing.T) {
	pool := testdb.Pool(t)
	rcv := newReceiver(t, pool, auth.NewStubProvider())

	payload := envelope(t, "session.created", map[string]any{"id": "sess_x"})
	req := signedRequest(t, "evt_unknown", payload)
	rec := httptest.NewRecorder()
	rcv.Handle(rec, req)
	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status %d", rec.Result().StatusCode)
	}
}
