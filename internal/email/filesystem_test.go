package email_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/email"
)

func newSender(t *testing.T) (*email.FilesystemSender, string) {
	t.Helper()
	dir := t.TempDir()
	s := email.NewFilesystemSender(dir, nil)
	// Deterministic clock + ids so concurrent-write tests don't flake.
	var (
		mu  sync.Mutex
		seq int
	)
	base := time.Date(2026, 5, 20, 14, 22, 31, 0, time.UTC)
	s.Now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		seq++
		return base.Add(time.Duration(seq) * time.Microsecond)
	}
	s.UUID = func() string {
		mu.Lock()
		defer mu.Unlock()
		// 12 hex chars after dashes are stripped.
		return [...]string{
			"7c2a1d-aaaa-aaaa",
			"7c2a1d-bbbb-bbbb",
			"7c2a1d-cccc-cccc",
			"7c2a1d-dddd-dddd",
			"7c2a1d-eeee-eeee",
			"7c2a1d-ffff-ffff",
			"7c2a1d-1111-1111",
			"7c2a1d-2222-2222",
		}[seq-1]
	}
	return s, dir
}

func TestFilesystemSenderWritesEMLAndJSON(t *testing.T) {
	s, dir := newSender(t)

	msg := email.Message{
		From:     "Liveaboard <noreply@x.test>",
		To:       "e2e+signup@example.invalid",
		Subject:  "Confirm your email",
		TextBody: "Visit http://localhost:5173/verify-email?token=abc123 to confirm.",
		HTMLBody: `<a href="http://localhost:5173/verify-email?token=abc123">click</a>`,
	}
	if err := s.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	recipientDir := filepath.Join(dir, "e2e+signup@example.invalid")
	entries, err := os.ReadDir(recipientDir)
	if err != nil {
		t.Fatalf("read recipient dir: %v", err)
	}

	var eml, sidecar string
	for _, e := range entries {
		name := e.Name()
		switch {
		case name == "latest.eml" || name == "latest.json":
			continue
		case strings.HasSuffix(name, ".eml"):
			eml = name
		case strings.HasSuffix(name, ".json"):
			sidecar = name
		}
	}
	if eml == "" || sidecar == "" {
		t.Fatalf("missing timestamped artifacts: %v", entries)
	}

	emlBytes, err := os.ReadFile(filepath.Join(recipientDir, eml))
	if err != nil {
		t.Fatal(err)
	}
	emlStr := string(emlBytes)
	for _, want := range []string{
		"From: Liveaboard <noreply@x.test>",
		"To: e2e+signup@example.invalid",
		"Subject: Confirm your email",
		"multipart/alternative",
	} {
		if !strings.Contains(emlStr, want) {
			t.Errorf(".eml missing %q\n---\n%s", want, emlStr)
		}
	}

	jsonBytes, err := os.ReadFile(filepath.Join(recipientDir, sidecar))
	if err != nil {
		t.Fatal(err)
	}
	var meta email.InboxMessage
	if err := json.Unmarshal(jsonBytes, &meta); err != nil {
		t.Fatalf("sidecar json: %v\n%s", err, jsonBytes)
	}
	if meta.To != "e2e+signup@example.invalid" {
		t.Errorf("To=%q", meta.To)
	}
	if meta.Subject != "Confirm your email" {
		t.Errorf("Subject=%q", meta.Subject)
	}
	if meta.EMLPath != filepath.Join(recipientDir, eml) {
		t.Errorf("EMLPath=%q want %q", meta.EMLPath, filepath.Join(recipientDir, eml))
	}
	if len(meta.Links) != 1 || meta.Links[0] != "http://localhost:5173/verify-email?token=abc123" {
		t.Errorf("Links=%v", meta.Links)
	}
	// sidecar must not duplicate full bodies.
	rawSidecar := string(jsonBytes)
	if strings.Contains(rawSidecar, "<a href=") {
		t.Errorf("sidecar should not duplicate html body, got %s", rawSidecar)
	}
}

func TestFilesystemSenderLatestPointers(t *testing.T) {
	s, dir := newSender(t)
	to := "owner@x.test"
	for i := 0; i < 3; i++ {
		msg := email.Message{
			From:     "Liveaboard <noreply@x.test>",
			To:       to,
			Subject:  "msg-" + string(rune('0'+i)),
			TextBody: "https://example.invalid/" + string(rune('a'+i)),
		}
		if err := s.Send(context.Background(), msg); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}

	latestEML, err := os.ReadFile(filepath.Join(dir, to, "latest.eml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(latestEML), "Subject: msg-2") {
		t.Errorf("latest.eml does not reflect newest: %s", latestEML)
	}

	// latest.eml must be a regular file, not a symlink.
	st, err := os.Lstat(filepath.Join(dir, to, "latest.eml"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode()&fs.ModeSymlink != 0 {
		t.Errorf("latest.eml is a symlink; want regular file")
	}

	var meta email.InboxMessage
	raw, err := os.ReadFile(filepath.Join(dir, to, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Subject != "msg-2" {
		t.Errorf("latest.json subject=%q", meta.Subject)
	}
	if len(meta.Links) != 1 || meta.Links[0] != "https://example.invalid/c" {
		t.Errorf("latest.json links=%v", meta.Links)
	}
}

func TestFilesystemSenderRejectsBadAddresses(t *testing.T) {
	s, _ := newSender(t)
	cases := []struct {
		name string
		msg  email.Message
	}{
		{"bad-from", email.Message{From: "not-an-email", To: "owner@x.test", Subject: "s"}},
		{"bad-to", email.Message{From: "noreply@x.test", To: "x", Subject: "s"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.Send(context.Background(), tc.msg); err == nil {
				t.Fatalf("want error for %s", tc.name)
			}
		})
	}
}

func TestRecipientSlugNormalization(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"E2E+Signup@Example.Invalid", "e2e+signup@example.invalid"},
		{"owner@x.test", "owner@x.test"},
		// Quoted local-parts let `/` and spaces survive RFC 5322 parsing;
		// our slug must still neutralize them so the writer never produces
		// path-traversal directory names.
		{`"weird/../traversal"@x.test`, "weird_.._traversal@x.test"},
		{`"contains space"@x.test`, "contains_space@x.test"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			s := email.NewFilesystemSender(t.TempDir(), nil)
			s.Now = func() time.Time { return time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC) }
			s.UUID = func() string { return "abcdef012345" }
			msg := email.Message{
				From:     "noreply@x.test",
				To:       c.in,
				Subject:  "s",
				TextBody: "http://x.test/t",
			}
			if err := s.Send(context.Background(), msg); err != nil {
				t.Fatalf("Send: %v", err)
			}
			if _, err := os.Stat(filepath.Join(s.InboxDir, c.want, "latest.eml")); err != nil {
				t.Fatalf("expected directory %q under %s: %v", c.want, s.InboxDir, err)
			}
		})
	}
}

func TestFilesystemSenderConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	s := email.NewFilesystemSender(dir, nil)
	// Real clock + real uuid — this is the concurrency contract test.

	const n = 20
	to := "owner@x.test"
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msg := email.Message{
				From:     "noreply@x.test",
				To:       to,
				Subject:  "concurrent",
				TextBody: "http://x.test/abc",
			}
			if err := s.Send(context.Background(), msg); err != nil {
				t.Errorf("Send %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	entries, err := os.ReadDir(filepath.Join(dir, to))
	if err != nil {
		t.Fatal(err)
	}
	var emls int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "latest.eml" || name == "latest.json" {
			continue
		}
		if strings.HasSuffix(name, ".eml") {
			emls++
		}
	}
	if emls != n {
		t.Errorf("got %d timestamped .eml files, want %d", emls, n)
	}

	// latest.* must be a complete artifact (not partially written) from one
	// of the writers.
	latestEML, err := os.ReadFile(filepath.Join(dir, to, "latest.eml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(latestEML), "Subject: concurrent") {
		t.Errorf("latest.eml partial or wrong: %s", latestEML)
	}
}

func TestInboxLinkFor(t *testing.T) {
	s, dir := newSender(t)

	send := func(to, subject, text string) {
		t.Helper()
		if err := s.Send(context.Background(), email.Message{
			From:     "noreply@x.test",
			To:       to,
			Subject:  subject,
			TextBody: text,
		}); err != nil {
			t.Fatal(err)
		}
	}

	send("owner@x.test", "older", "http://x.test/older")
	send("owner@x.test", "newer", "https://x.test/verify-email?token=NEW")
	send("other@x.test", "unrelated", "http://x.test/elsewhere")

	got, err := email.InboxLinkFor(dir, "owner@x.test", "verify-email")
	if err != nil {
		t.Fatalf("InboxLinkFor: %v", err)
	}
	if got != "https://x.test/verify-email?token=NEW" {
		t.Errorf("got %q", got)
	}

	missing, err := email.InboxLinkFor(dir, "owner@x.test", "no-such-needle")
	if err != nil || missing != "" {
		t.Errorf("want empty/no error for missing, got %q / %v", missing, err)
	}
}

func TestReadLatestInboxMessage(t *testing.T) {
	s, dir := newSender(t)
	if err := s.Send(context.Background(), email.Message{
		From: "noreply@x.test", To: "owner@x.test", Subject: "S",
		TextBody: "http://x.test/t",
	}); err != nil {
		t.Fatal(err)
	}
	meta, err := email.ReadLatestInboxMessage(dir, "owner@x.test")
	if err != nil {
		t.Fatal(err)
	}
	if meta.Subject != "S" {
		t.Errorf("subject=%q", meta.Subject)
	}

	if _, err := email.ReadLatestInboxMessage(dir, "no-one@x.test"); err == nil {
		t.Errorf("expected error for missing recipient")
	}
}
