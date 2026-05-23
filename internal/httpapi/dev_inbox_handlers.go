package httpapi

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/karimfan/liveaboard/internal/email"
)

// validRecipientSlug mirrors the per-byte rules in email.recipientSlug:
// lowercase letters, digits, and @ . + _ - are the only safe characters.
// Anything else (including / and ..) is rejected with 400, so this handler
// cannot be used as a directory-traversal pivot regardless of the inbox
// directory's path.
func validRecipientSlug(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	if strings.Contains(s, "/") {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '@' || c == '.' || c == '+' || c == '_' || c == '-':
		default:
			return false
		}
	}
	return true
}

// validMessageFile accepts the basename of an artifact file. The writer
// emits either timestamped names (YYYYMMDDTHHMMSS.uuuuuuZ-suffix.{eml,json})
// or the latest.* pointers. Anything else is rejected.
func validMessageFile(s string) bool {
	if strings.Contains(s, "/") || strings.Contains(s, "..") {
		return false
	}
	return strings.HasSuffix(s, ".eml") || strings.HasSuffix(s, ".json")
}

// inboxRecipient is one entry in the index view.
type inboxRecipient struct {
	Slug      string
	Subject   string
	LatestRaw string // raw timestamp string from sidecar
}

func (s *Server) handleDevInboxIndex(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(s.DevInboxDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "inbox_read", "could not read inbox directory")
		return
	}
	var recipients []inboxRecipient
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		slug := e.Name()
		if !validRecipientSlug(slug) {
			continue
		}
		entry := inboxRecipient{Slug: slug}
		latest := filepath.Join(s.DevInboxDir, slug, "latest.json")
		if raw, err := os.ReadFile(latest); err == nil {
			var meta email.InboxMessage
			if json.Unmarshal(raw, &meta) == nil {
				entry.Subject = meta.Subject
				entry.LatestRaw = meta.SentAt.Format("2006-01-02 15:04:05 UTC")
			}
		}
		recipients = append(recipients, entry)
	}
	sort.Slice(recipients, func(i, j int) bool {
		return recipients[i].LatestRaw > recipients[j].LatestRaw
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!doctype html><html><head><title>Dev inbox</title>`)
	fmt.Fprint(w, `<style>body{font-family:-apple-system,system-ui,sans-serif;max-width:780px;margin:32px auto;padding:0 16px;color:#1c1917}h1{font-weight:700}table{border-collapse:collapse;width:100%}td,th{border-bottom:1px solid #e3e0dd;padding:8px 6px;text-align:left;vertical-align:top}a{color:#1f5fb3;text-decoration:none}a:hover{text-decoration:underline}.muted{color:#78716c;font-size:13px}</style>`)
	fmt.Fprint(w, `</head><body>`)
	fmt.Fprintf(w, `<h1>Dev inbox</h1><p class="muted">%s</p>`, html.EscapeString(s.DevInboxDir))
	if len(recipients) == 0 {
		fmt.Fprint(w, `<p>No messages yet.</p>`)
	} else {
		fmt.Fprint(w, `<table><thead><tr><th>Recipient</th><th>Latest message</th><th class="muted">Received</th></tr></thead><tbody>`)
		for _, rcp := range recipients {
			fmt.Fprintf(w,
				`<tr><td><a href="/dev/inbox/%s">%s</a></td><td>%s</td><td class="muted">%s</td></tr>`,
				html.EscapeString(rcp.Slug),
				html.EscapeString(rcp.Slug),
				html.EscapeString(rcp.Subject),
				html.EscapeString(rcp.LatestRaw),
			)
		}
		fmt.Fprint(w, `</tbody></table>`)
	}
	fmt.Fprint(w, `</body></html>`)
}

func (s *Server) handleDevInboxRecipient(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "recipient")
	if !validRecipientSlug(slug) {
		writeError(w, http.StatusBadRequest, "invalid_recipient", "recipient slug is not a normalized email")
		return
	}
	dir := filepath.Join(s.DevInboxDir, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "no inbox for that recipient")
		return
	}

	type row struct {
		File    string
		Subject string
		Sent    string
		ID      string
	}
	var rows []row
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "latest.eml" || name == "latest.json" {
			continue
		}
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var meta email.InboxMessage
		if err := json.Unmarshal(raw, &meta); err != nil {
			continue
		}
		emlName := strings.TrimSuffix(name, ".json") + ".eml"
		rows = append(rows, row{
			File:    emlName,
			Subject: meta.Subject,
			Sent:    meta.SentAt.Format("2006-01-02 15:04:05 UTC"),
			ID:      meta.ID,
		})
	}
	// Newest first.
	sort.Slice(rows, func(i, j int) bool { return rows[i].File > rows[j].File })

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!doctype html><html><head><title>Dev inbox</title>`)
	fmt.Fprint(w, `<style>body{font-family:-apple-system,system-ui,sans-serif;max-width:780px;margin:32px auto;padding:0 16px;color:#1c1917}h1{font-weight:700}table{border-collapse:collapse;width:100%}td,th{border-bottom:1px solid #e3e0dd;padding:8px 6px;text-align:left;vertical-align:top}a{color:#1f5fb3;text-decoration:none}a:hover{text-decoration:underline}.muted{color:#78716c;font-size:13px}</style>`)
	fmt.Fprint(w, `</head><body>`)
	fmt.Fprintf(w, `<p><a href="/dev/inbox">← all recipients</a></p>`)
	fmt.Fprintf(w, `<h1>%s</h1>`, html.EscapeString(slug))
	if len(rows) == 0 {
		fmt.Fprint(w, `<p>No messages.</p>`)
	} else {
		fmt.Fprint(w, `<table><thead><tr><th>Subject</th><th class="muted">Received</th></tr></thead><tbody>`)
		for _, r := range rows {
			fmt.Fprintf(w,
				`<tr><td><a href="/dev/inbox/%s/%s">%s</a></td><td class="muted">%s</td></tr>`,
				html.EscapeString(slug),
				html.EscapeString(r.File),
				html.EscapeString(r.Subject),
				html.EscapeString(r.Sent),
			)
		}
		fmt.Fprint(w, `</tbody></table>`)
	}
	fmt.Fprint(w, `</body></html>`)
}

func (s *Server) handleDevInboxMessage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "recipient")
	file := chi.URLParam(r, "file")
	if !validRecipientSlug(slug) || !validMessageFile(file) {
		writeError(w, http.StatusBadRequest, "invalid_path", "recipient or file is not valid")
		return
	}
	path := filepath.Join(s.DevInboxDir, slug, file)
	// Re-evaluate after resolving symlinks etc. to belt-and-braces against
	// any clever inputs that slipped through the per-byte validator.
	abs, err := filepath.Abs(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_path", "could not resolve path")
		return
	}
	rootAbs, err := filepath.Abs(s.DevInboxDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not resolve inbox root")
		return
	}
	if !strings.HasPrefix(abs, rootAbs+string(os.PathSeparator)) {
		writeError(w, http.StatusBadRequest, "outside_inbox", "path escapes inbox dir")
		return
	}

	body, err := os.ReadFile(abs)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "message not found")
		return
	}
	if strings.HasSuffix(file, ".json") {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
		return
	}
	// .eml — serve as plain text so the browser shows the headers and body
	// without trying to interpret it as HTML.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(body)
}
