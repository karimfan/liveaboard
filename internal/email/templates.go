package email

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	"text/template"
	"time"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

// Kind is the email kind. Each kind has three template files:
// {kind}.subject.tmpl, {kind}.txt.tmpl, {kind}.html.tmpl.
type Kind string

const (
	KindVerification            Kind = "verification"
	KindInvitation              Kind = "invitation"
	KindPasswordReset           Kind = "password_reset"
	KindChangeEmail             Kind = "change_email"
	KindTripAssigned            Kind = "trip_assigned"
	KindTripUnassigned          Kind = "trip_unassigned"
	KindGuestRegistrationInvite Kind = "guest_registration_invite"
)

// Render renders the three parts of an email kind against the given vars
// and returns a Message ready for the Sender. The caller fills in
// `From` and `To`.
func Render(kind Kind, vars Vars) (Message, error) {
	subj, err := renderText(string(kind)+".subject.tmpl", vars)
	if err != nil {
		return Message{}, fmt.Errorf("render subject: %w", err)
	}
	text, err := renderText(string(kind)+".txt.tmpl", vars)
	if err != nil {
		return Message{}, fmt.Errorf("render text: %w", err)
	}
	html, err := renderHTML(string(kind)+".html.tmpl", vars)
	if err != nil {
		return Message{}, fmt.Errorf("render html: %w", err)
	}
	return Message{
		Subject:  strings.TrimSpace(subj),
		TextBody: text,
		HTMLBody: html,
	}, nil
}

func renderText(name string, vars Vars) (string, error) {
	b, err := templatesFS.ReadFile("templates/" + name)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(name).Funcs(template.FuncMap{
		"fmtdate": fmtDate,
	}).Parse(string(b))
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, vars); err != nil {
		return "", err
	}
	return out.String(), nil
}

func renderHTML(name string, vars Vars) (string, error) {
	b, err := templatesFS.ReadFile("templates/" + name)
	if err != nil {
		return "", err
	}
	tmpl, err := htmltemplate.New(name).Funcs(htmltemplate.FuncMap{
		"fmtdate": fmtDate,
	}).Parse(string(b))
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, vars); err != nil {
		return "", err
	}
	return out.String(), nil
}

func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2 Jan 2006 15:04 UTC")
}
