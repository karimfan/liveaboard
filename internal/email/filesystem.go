package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FilesystemSender writes outbound messages to disk under InboxDir instead
// of speaking SMTP. Intended for local development, demos, and end-to-end
// test harnesses. Never produces network I/O.
//
// For each Send, two files are written atomically under
// <InboxDir>/<recipient-slug>/:
//
//   - <stem>.eml  — exact MIME bytes from buildMIME(msg)
//   - <stem>.json — sidecar with from/to/subject/sent_at/eml_path/links
//
// After both files land, <recipient-slug>/latest.eml and
// <recipient-slug>/latest.json are atomically rewritten (as regular files,
// not symlinks) to mirror the newest message.
type FilesystemSender struct {
	InboxDir string
	Log      *slog.Logger

	// Now is injectable for deterministic tests. Defaults to
	// time.Now().UTC().
	Now func() time.Time

	// UUID is injectable for deterministic tests. Defaults to uuid.NewString.
	// Only the first 12 hex chars (with dashes removed) are used in the
	// filename stem.
	UUID func() string

	// mu serializes latest.* swaps within a single sender instance so
	// concurrent senders inside the same process don't interleave half-
	// written symlink-free pointer files.
	mu sync.Mutex
}

// NewFilesystemSender constructs a FilesystemSender with stdlib defaults.
func NewFilesystemSender(inboxDir string, log *slog.Logger) *FilesystemSender {
	return &FilesystemSender{
		InboxDir: inboxDir,
		Log:      log,
		Now:      func() time.Time { return time.Now().UTC() },
		UUID:     uuid.NewString,
	}
}

// InboxMessage is the JSON shape of the .json sidecar written next to each
// .eml. It is the machine-readable contract for E2E tests: every
// http(s)://… URL the rendered message contained is present in Links.
type InboxMessage struct {
	ID      string    `json:"id"`
	From    string    `json:"from"`
	To      string    `json:"to"`
	Subject string    `json:"subject"`
	SentAt  time.Time `json:"sent_at"`
	EMLPath string    `json:"eml_path"`
	Links   []string  `json:"links"`
}

// Send writes the rendered message to disk under InboxDir/<recipient>/.
// ctx is accepted for interface compatibility but not consulted; filesystem
// writes are non-blocking enough that an explicit cancel surface is more
// complexity than it's worth here.
func (f *FilesystemSender) Send(_ context.Context, msg Message) error {
	if f.InboxDir == "" {
		return errors.New("email: FilesystemSender.InboxDir is empty")
	}
	if _, err := mail.ParseAddress(msg.From); err != nil {
		return fmt.Errorf("invalid From: %w", err)
	}
	toAddr, err := mail.ParseAddress(msg.To)
	if err != nil {
		return fmt.Errorf("invalid To: %w", err)
	}

	body, err := buildMIME(msg)
	if err != nil {
		return err
	}

	now := f.now()
	id := f.uuid()

	slug := recipientSlug(toAddr.Address)
	dir := filepath.Join(f.InboxDir, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir inbox: %w", err)
	}

	suffix := strings.ReplaceAll(id, "-", "")
	if len(suffix) > 12 {
		suffix = suffix[:12]
	}
	// 20260520T142231.123456Z-7c2a1d… — lexically sortable, microsecond
	// precision, plus a short uuid suffix so concurrent sends at the same
	// instant don't collide.
	stem := now.Format("20060102T150405.000000Z") + "-" + suffix

	emlPath := filepath.Join(dir, stem+".eml")
	jsonPath := filepath.Join(dir, stem+".json")

	meta := InboxMessage{
		ID:      id,
		From:    msg.From,
		To:      msg.To,
		Subject: msg.Subject,
		SentAt:  now,
		EMLPath: emlPath,
		Links:   ExtractLinks(msg),
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sidecar: %w", err)
	}
	metaBytes = append(metaBytes, '\n')

	if err := writeFileAtomic(emlPath, body, 0o644); err != nil {
		return fmt.Errorf("write eml: %w", err)
	}
	if err := writeFileAtomic(jsonPath, metaBytes, 0o644); err != nil {
		return fmt.Errorf("write sidecar: %w", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := writeFileAtomic(filepath.Join(dir, "latest.eml"), body, 0o644); err != nil {
		return fmt.Errorf("write latest.eml: %w", err)
	}
	if err := writeFileAtomic(filepath.Join(dir, "latest.json"), metaBytes, 0o644); err != nil {
		return fmt.Errorf("write latest.json: %w", err)
	}

	if f.Log != nil {
		f.Log.Debug("filesystem email written",
			"to", toAddr.Address,
			"subject", msg.Subject,
			"id", id,
			"eml", emlPath,
			"links", len(meta.Links),
		)
	}
	return nil
}

func (f *FilesystemSender) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now().UTC()
}

func (f *FilesystemSender) uuid() string {
	if f.UUID != nil {
		return f.UUID()
	}
	return uuid.NewString()
}

// writeFileAtomic writes data to a unique temp file in the same directory
// and renames it into place. A concurrent reader sees either the previous
// file (or no file) or the new one — never a partial write.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

// recipientSlug normalizes an email address to a path-safe directory name.
// Preserves @ . + _ -, lowercases the whole address, and replaces every
// other byte with _. The full local-part and domain remain readable.
// Path-traversal characters (/, \, .., control bytes) cannot survive this
// transform.
func recipientSlug(addr string) string {
	addr = strings.ToLower(addr)
	var b strings.Builder
	b.Grow(len(addr))
	for i := 0; i < len(addr); i++ {
		c := addr[i]
		switch {
		case c >= 'a' && c <= 'z':
			b.WriteByte(c)
		case c >= '0' && c <= '9':
			b.WriteByte(c)
		case c == '@' || c == '.' || c == '+' || c == '_' || c == '-':
			b.WriteByte(c)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		out = "_"
	}
	return out
}

// InboxLinkFor reads the newest message in inboxDir for the recipient
// matching `to` and returns the first link in its sidecar that contains
// needle. Empty needle matches the first link in the newest message.
// Returns "" with no error when no match is found.
func InboxLinkFor(inboxDir, to, needle string) (string, error) {
	addr, err := mail.ParseAddress(to)
	if err != nil {
		return "", fmt.Errorf("invalid to: %w", err)
	}
	slug := recipientSlug(addr.Address)
	dir := filepath.Join(inboxDir, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read inbox: %w", err)
	}

	// Collect timestamped sidecar files. Their lexical order matches their
	// chronological order because of the YYYYMMDDTHHMMSS.uuuuuuZ stem.
	var sidecars []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "latest.json" {
			continue
		}
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		sidecars = append(sidecars, name)
	}
	for i := len(sidecars) - 1; i >= 0; i-- {
		raw, err := os.ReadFile(filepath.Join(dir, sidecars[i]))
		if err != nil {
			continue
		}
		var meta InboxMessage
		if err := json.Unmarshal(raw, &meta); err != nil {
			continue
		}
		for _, u := range meta.Links {
			if strings.Contains(u, needle) {
				return u, nil
			}
		}
	}
	return "", nil
}

// ReadLatestInboxMessage parses the latest.json sidecar for the recipient
// matching `to`. Returns a wrapped os.ErrNotExist if the recipient has no
// inbox yet.
func ReadLatestInboxMessage(inboxDir, to string) (InboxMessage, error) {
	addr, err := mail.ParseAddress(to)
	if err != nil {
		return InboxMessage{}, fmt.Errorf("invalid to: %w", err)
	}
	slug := recipientSlug(addr.Address)
	path := filepath.Join(inboxDir, slug, "latest.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return InboxMessage{}, err
	}
	var meta InboxMessage
	if err := json.Unmarshal(raw, &meta); err != nil {
		return InboxMessage{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return meta, nil
}
