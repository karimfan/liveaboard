package email

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime/quotedprintable"
	"net/mail"
	"strings"
)

// buildMIME serializes a Message into RFC 5322 / multipart/alternative
// bytes ready for an SMTP DATA command.
//
// We send both text/plain and text/html parts when HTMLBody is set;
// text-only when it isn't. Bodies are quoted-printable encoded so
// non-ASCII / long lines don't break SMTP. CRLF line endings throughout.
func buildMIME(msg Message) ([]byte, error) {
	if _, err := mail.ParseAddress(msg.From); err != nil {
		return nil, fmt.Errorf("invalid From: %w", err)
	}
	if _, err := mail.ParseAddress(msg.To); err != nil {
		return nil, fmt.Errorf("invalid To: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("From: " + msg.From + "\r\n")
	buf.WriteString("To: " + msg.To + "\r\n")
	buf.WriteString("Subject: " + sanitizeHeader(msg.Subject) + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")

	if msg.HTMLBody == "" {
		// text-only
		buf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
		buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		if err := writeQP(&buf, msg.TextBody); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	boundary, err := newBoundary()
	if err != nil {
		return nil, err
	}
	buf.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")

	// text part
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	if err := writeQP(&buf, msg.TextBody); err != nil {
		return nil, err
	}
	buf.WriteString("\r\n")

	// html part
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	if err := writeQP(&buf, msg.HTMLBody); err != nil {
		return nil, err
	}
	buf.WriteString("\r\n")
	buf.WriteString("--" + boundary + "--\r\n")
	return buf.Bytes(), nil
}

func writeQP(buf *bytes.Buffer, body string) error {
	w := quotedprintable.NewWriter(buf)
	if _, err := w.Write([]byte(body)); err != nil {
		return err
	}
	return w.Close()
}

func newBoundary() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "lb-bdry-" + hex.EncodeToString(b), nil
}

// sanitizeHeader strips bare CR/LF that could inject extra headers.
func sanitizeHeader(s string) string {
	r := strings.NewReplacer("\r", " ", "\n", " ")
	return r.Replace(strings.TrimSpace(s))
}
