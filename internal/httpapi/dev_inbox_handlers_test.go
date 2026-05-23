package httpapi_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/email"
	"github.com/karimfan/liveaboard/internal/httpapi"
	"github.com/karimfan/liveaboard/internal/imports"
	"github.com/karimfan/liveaboard/internal/org"
	"github.com/karimfan/liveaboard/internal/testdb"
)

// devInboxHarness mirrors newHarness but lets the test specify a
// DevInboxDir up-front. Conditionally mounting /dev/inbox is part of the
// contract under test, so it has to happen at Router() time.
func devInboxHarness(t *testing.T, devInboxDir string) *httptest.Server {
	t.Helper()
	pool := testdb.Pool(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	mock := &email.MockSender{}
	svc := auth.New(pool, mock, log, "http://localhost:5173", "Liveaboard <noreply@x.test>")
	svc.BcryptCost = 4
	svc.SessionDuration = time.Hour
	session := &auth.SessionMiddleware{Store: pool, Log: log}
	guestSession := &auth.GuestSessionMiddleware{Store: pool, Log: log}
	runner := &imports.Runner{Store: pool, Log: log, Months: 1}
	srv := &httpapi.Server{
		Org:          org.New(pool),
		Log:          log,
		Auth:         svc,
		Session:      session,
		GuestSession: guestSession,
		AdminAPI:     &httpapi.AdminHandlers{Store: pool},
		ImportRunner: runner,
		CookieSecure: false,
		DevInboxDir:  devInboxDir,
	}
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts
}

func TestDevInboxNotMountedWhenDirEmpty(t *testing.T) {
	// DevInboxDir empty: /dev/inbox should fall through to the SPA
	// handler, which returns the SPA index (no inbox-specific markup).
	ts := devInboxHarness(t, "")
	resp, err := http.Get(ts.URL + "/dev/inbox")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "<h1>Dev inbox</h1>") {
		t.Errorf("dev inbox content served when DevInboxDir is empty: %s", body)
	}
}

func TestDevInboxIndexEmpty(t *testing.T) {
	dir := t.TempDir()
	ts := devInboxHarness(t, dir)

	resp, err := http.Get(ts.URL + "/dev/inbox")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "No messages yet") {
		t.Errorf("expected empty marker, got %s", body)
	}
}

func TestDevInboxListsRecipientsAndMessages(t *testing.T) {
	dir := t.TempDir()
	s := email.NewFilesystemSender(dir, nil)
	for _, m := range []email.Message{
		{From: "n@x.test", To: "owner@x.test", Subject: "First", TextBody: "http://x.test/1"},
		{From: "n@x.test", To: "dir@x.test", Subject: "Second", TextBody: "http://x.test/2"},
	} {
		if err := s.Send(context.Background(), m); err != nil {
			t.Fatal(err)
		}
	}
	ts := devInboxHarness(t, dir)

	// Index shows both recipients.
	resp, _ := http.Get(ts.URL + "/dev/inbox")
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	for _, want := range []string{"owner@x.test", "dir@x.test", "First", "Second"} {
		if !strings.Contains(bodyStr, want) {
			t.Errorf("index missing %q: %s", want, bodyStr)
		}
	}

	// Per-recipient page shows the message subject.
	resp, _ = http.Get(ts.URL + "/dev/inbox/owner@x.test")
	if resp.StatusCode != 200 {
		t.Fatalf("recipient status %d", resp.StatusCode)
	}
	body, _ = io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "First") {
		t.Errorf("recipient page missing subject: %s", body)
	}
}

func TestDevInboxRejectsBadSlug(t *testing.T) {
	dir := t.TempDir()
	ts := devInboxHarness(t, dir)

	for _, path := range []string{
		"/dev/inbox/..",
		"/dev/inbox/UPPERCASE@x.test",
		"/dev/inbox/has%20space@x.test",
		"/dev/inbox/owner@x.test/..%2flatest.eml",
	} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			t.Errorf("path %q should have been rejected", path)
		}
	}
}

func TestDevInboxServesEMLAsPlainText(t *testing.T) {
	dir := t.TempDir()
	s := email.NewFilesystemSender(dir, nil)
	if err := s.Send(context.Background(), email.Message{
		From: "n@x.test", To: "owner@x.test", Subject: "S", TextBody: "http://x.test/t",
	}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "owner@x.test"))
	var emlName string
	for _, e := range entries {
		if e.Name() != "latest.eml" && strings.HasSuffix(e.Name(), ".eml") {
			emlName = e.Name()
		}
	}
	if emlName == "" {
		t.Fatal("no timestamped .eml found")
	}

	ts := devInboxHarness(t, dir)
	resp, err := http.Get(ts.URL + "/dev/inbox/owner@x.test/" + emlName)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type=%q want text/plain", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Subject: S") {
		t.Errorf(".eml body missing subject: %s", body)
	}
}
