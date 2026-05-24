package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/mail"
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
	if err != nil && !os.IsNotExist(err) {
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
	// .eml — by default render an HTML preview with clickable links from
	// the JSON sidecar and the multipart/alternative HTML part embedded
	// in a sandboxed iframe. ?raw=1 serves the underlying MIME bytes as
	// plain text for inspection.
	if r.URL.Query().Get("raw") == "1" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(body)
		return
	}
	renderMessageHTML(w, abs, body, slug, file)
}

// renderMessageHTML renders the message-detail page: subject header,
// extracted links from the sidecar (clickable, no QP wrapping), and
// the HTML body in a sandboxed iframe so its inline styles render but
// it cannot execute scripts or escape the page.
func renderMessageHTML(w http.ResponseWriter, emlPath string, emlBytes []byte, slug, file string) {
	var meta email.InboxMessage
	sidecarPath := strings.TrimSuffix(emlPath, ".eml") + ".json"
	if raw, err := os.ReadFile(sidecarPath); err == nil {
		_ = json.Unmarshal(raw, &meta)
	}
	htmlBody := extractHTMLPart(emlBytes)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!doctype html><html><head><title>Inbox</title>`)
	fmt.Fprint(w, `<style>body{font-family:-apple-system,system-ui,sans-serif;max-width:780px;margin:32px auto;padding:0 16px;color:#1c1917}h1{font-weight:700;margin:0 0 8px}.muted{color:#78716c;font-size:13px}.kv{margin:4px 0;font-size:14px}a{color:#1f5fb3}.links{background:#f8f7f6;border:1px solid #e3e0dd;border-radius:8px;padding:12px 16px;margin:16px 0}.links h3{margin:0 0 8px;font-size:13px;text-transform:uppercase;letter-spacing:0.05em;color:#57534e}.links ul{margin:0;padding-left:18px;font-size:13px;word-break:break-all}iframe{width:100%;height:560px;border:1px solid #e3e0dd;border-radius:8px;margin-top:16px;background:white}</style>`)
	fmt.Fprint(w, `</head><body>`)
	fmt.Fprintf(w, `<p><a href="/dev/inbox/%s">← %s</a></p>`, html.EscapeString(slug), html.EscapeString(slug))
	fmt.Fprintf(w, `<h1>%s</h1>`, html.EscapeString(meta.Subject))
	fmt.Fprintf(w, `<div class="kv muted"><strong>From:</strong> %s</div>`, html.EscapeString(meta.From))
	fmt.Fprintf(w, `<div class="kv muted"><strong>To:</strong> %s</div>`, html.EscapeString(meta.To))
	if !meta.SentAt.IsZero() {
		fmt.Fprintf(w, `<div class="kv muted"><strong>Sent:</strong> %s</div>`,
			html.EscapeString(meta.SentAt.Format("2006-01-02 15:04:05 UTC")))
	}

	if len(meta.Links) > 0 {
		fmt.Fprint(w, `<div class="links"><h3>Links</h3><ul>`)
		for _, link := range meta.Links {
			esc := html.EscapeString(link)
			fmt.Fprintf(w, `<li><a href="%s">%s</a></li>`, esc, esc)
		}
		fmt.Fprint(w, `</ul></div>`)
	}

	if htmlBody != "" {
		// srcdoc + sandbox="" → renders inline HTML/styles, blocks scripts
		// and same-origin access. Links inside still navigate (the iframe
		// itself, or the top page if they explicitly target it).
		fmt.Fprintf(w, `<iframe sandbox="" srcdoc="%s"></iframe>`, html.EscapeString(htmlBody))
	} else {
		fmt.Fprint(w, `<p class="muted">No HTML body in this message.</p>`)
	}

	fmt.Fprintf(w, `<p class="muted" style="margin-top:24px"><a href="/dev/inbox/%s/%s?raw=1">View raw .eml</a></p>`,
		html.EscapeString(slug), html.EscapeString(file))
	fmt.Fprint(w, `</body></html>`)
}

// extractHTMLPart parses an .eml message and returns the decoded HTML
// body. Handles both single-part text/html messages and
// multipart/alternative containers. Returns "" if no HTML part is
// present or the message can't be parsed.
func extractHTMLPart(emlBytes []byte) string {
	msg, err := mail.ReadMessage(bytes.NewReader(emlBytes))
	if err != nil {
		return ""
	}
	ct := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return ""
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		if mediaType == "text/html" {
			return decodePart(msg.Body, msg.Header.Get("Content-Transfer-Encoding"))
		}
		return ""
	}
	boundary := params["boundary"]
	if boundary == "" {
		return ""
	}
	mr := multipart.NewReader(msg.Body, boundary)
	for {
		p, err := mr.NextPart()
		if err != nil {
			return ""
		}
		partCT, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if partCT == "text/html" {
			return decodePart(p, p.Header.Get("Content-Transfer-Encoding"))
		}
	}
}

func decodePart(r io.Reader, enc string) string {
	if strings.EqualFold(enc, "quoted-printable") {
		r = quotedprintable.NewReader(r)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return ""
	}
	return string(b)
}
