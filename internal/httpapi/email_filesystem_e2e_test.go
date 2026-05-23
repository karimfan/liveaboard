package httpapi_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/email"
	"github.com/karimfan/liveaboard/internal/httpapi"
	"github.com/karimfan/liveaboard/internal/imports"
	"github.com/karimfan/liveaboard/internal/org"
	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

// TestSignupVerifyViaFilesystemTransport boots an HTTP server with a real
// auth.Service wired to a FilesystemSender that writes into t.TempDir(),
// then runs signup → read inbox → follow verification link → confirm
// verified. This is the explicit DoD assertion that the filesystem
// transport completes a real flow end-to-end without an SMTP relay.
func TestSignupVerifyViaFilesystemTransport(t *testing.T) {
	inboxDir := t.TempDir()
	pool := testdb.Pool(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	sender := email.NewFilesystemSender(inboxDir, log)
	svc := auth.New(pool, sender, log, "http://localhost:5173", "Liveaboard <noreply@x.test>")
	svc.BcryptCost = 4
	svc.SessionDuration = time.Hour

	srv := &httpapi.Server{
		Org:          org.New(pool),
		Log:          log,
		Auth:         svc,
		Session:      &auth.SessionMiddleware{Store: pool, Log: log},
		GuestSession: &auth.GuestSessionMiddleware{Store: pool, Log: log},
		AdminAPI:     &httpapi.AdminHandlers{Store: pool},
		ImportRunner: &imports.Runner{Store: pool, Log: log, Months: 1},
		CookieSecure: false,
	}
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)

	c := &http.Client{}
	recipient := "e2e+signup@example.invalid"
	password := "Sup3rStrong!"

	resp, body := doJSON(t, c, "POST", ts.URL+"/api/auth/signup", map[string]any{
		"email":             recipient,
		"password":          password,
		"full_name":         "E2E Owner",
		"organization_name": "E2E Diving",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("signup: %d %v", resp.StatusCode, body)
	}

	// Read the verification link from the on-disk inbox the same way an
	// end-to-end harness would. Routes through the shared ExtractLinks
	// helper internally so the URL we recover is the same one MockSender
	// would have surfaced.
	link, err := email.InboxLinkFor(inboxDir, recipient, "verify-email")
	if err != nil {
		t.Fatalf("InboxLinkFor: %v", err)
	}
	if link == "" {
		t.Fatalf("no verification link in inbox at %s", inboxDir)
	}
	token := tokenFromLink(t, link)

	resp, body = doJSON(t, c, "POST", ts.URL+"/api/auth/verify-email", map[string]any{
		"token": token,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("verify-email: %d %v", resp.StatusCode, body)
	}

	resp, body = doJSON(t, c, "POST", ts.URL+"/api/auth/login", map[string]any{
		"email":    recipient,
		"password": password,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("login: %d %v", resp.StatusCode, body)
	}
	cookie := pickCookieFrom(resp.Cookies(), auth.SessionCookieName)
	if cookie == nil {
		t.Fatal("login did not set session cookie")
	}

	resp, body = doJSON(t, c, "GET", ts.URL+"/api/me", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("me: %d %v", resp.StatusCode, body)
	}
	if body["email"] != recipient {
		t.Errorf("me email=%v want %q", body["email"], recipient)
	}
	if body["email_verified"] != true {
		t.Errorf("expected email_verified=true, got %v", body["email_verified"])
	}
	if body["role"] != string(store.RoleOrgAdmin) {
		t.Errorf("role=%v", body["role"])
	}

	// Sidecar JSON should also resolve via the latest.json pointer.
	meta, err := email.ReadLatestInboxMessage(inboxDir, recipient)
	if err != nil {
		t.Fatalf("ReadLatestInboxMessage: %v", err)
	}
	if meta.To != recipient {
		t.Errorf("sidecar To=%q", meta.To)
	}
	if len(meta.Links) == 0 {
		t.Errorf("sidecar links empty; want at least the verification URL")
	}
}
