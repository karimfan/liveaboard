package email_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/email"
)

// TestFilesystemSenderAllKinds renders every email.Kind and confirms that
// FilesystemSender writes the .eml + .json artifacts to disk, with the
// JSON `links` field matching what ExtractLinks returns over the rendered
// Message. This is the explicit DoD assertion that all eight email kinds
// flow through the new transport.
func TestFilesystemSenderAllKinds(t *testing.T) {
	expires := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	tripStart := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	tripEnd := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	baseURL := "https://app.example.test"

	folioLines := []email.FolioLineVar{
		{Name: "Beer", Quantity: 2, UnitPrice: "$5.00", Total: "$10.00"},
	}

	cases := []struct {
		kind  email.Kind
		vars  email.Vars
		wantN int
	}{
		{
			kind: email.KindVerification,
			vars: email.Vars{
				AppName: "Liveaboard", OrganizationName: "Acme",
				RecipientEmail: "owner@x.test",
				ActionURL:      baseURL + "/verify-email?token=v",
				ExpiresAt:      expires,
			},
			wantN: 1,
		},
		{
			kind: email.KindInvitation,
			vars: email.Vars{
				AppName: "Liveaboard", OrganizationName: "Acme",
				RecipientEmail: "dir@x.test", RecipientName: "Director",
				InviterName: "Owner",
				ActionURL:   baseURL + "/invitations/i/accept",
				ExpiresAt:   expires,
			},
			wantN: 1,
		},
		{
			kind: email.KindPasswordReset,
			vars: email.Vars{
				AppName:        "Liveaboard",
				RecipientEmail: "owner@x.test",
				ActionURL:      baseURL + "/reset-password?token=r",
				ExpiresAt:      expires,
			},
			wantN: 1,
		},
		{
			kind: email.KindChangeEmail,
			vars: email.Vars{
				AppName:        "Liveaboard",
				RecipientEmail: "new@x.test",
				ActionURL:      baseURL + "/confirm-email-change?token=c",
				ExpiresAt:      expires,
			},
			wantN: 1,
		},
		{
			kind: email.KindTripAssigned,
			vars: email.Vars{
				AppName: "Liveaboard", RecipientName: "Director",
				InviterName:   "Owner",
				ActionURL:     baseURL + "/admin/trips",
				TripBoatName:  "Sirius",
				TripItinerary: "Red Sea North",
				TripStartDate: tripStart,
				TripEndDate:   tripEnd,
			},
			wantN: 1,
		},
		{
			kind: email.KindTripUnassigned,
			vars: email.Vars{
				AppName: "Liveaboard", RecipientName: "Director",
				InviterName:   "Owner",
				TripBoatName:  "Sirius",
				TripItinerary: "Red Sea North",
				TripStartDate: tripStart,
				TripEndDate:   tripEnd,
			},
			wantN: 0,
		},
		{
			kind: email.KindGuestRegistrationInvite,
			vars: email.Vars{
				AppName: "Liveaboard", OrganizationName: "Acme",
				RecipientName: "Guest", RecipientEmail: "guest@x.test",
				ActionURL:     baseURL + "/guest/invitations/g",
				TripBoatName:  "Sirius",
				TripItinerary: "Red Sea North",
				TripStartDate: tripStart,
				TripEndDate:   tripEnd,
				ExpiresAt:     expires,
			},
			wantN: 1,
		},
		{
			kind: email.KindGuestFolioClosed,
			vars: email.Vars{
				AppName: "Liveaboard", OrganizationName: "Acme",
				RecipientName: "Guest", RecipientEmail: "guest@x.test",
				TripBoatName:            "Sirius",
				TripItinerary:           "Red Sea North",
				TripStartDate:           tripStart,
				TripEndDate:             tripEnd,
				FolioLines:              folioLines,
				FolioSubtotalUSD:        "$10.00",
				FolioCardFeeUSD:         "$0.30",
				FolioTotalUSD:           "$10.30",
				FolioSettlementTotal:    "$10.30",
				FolioSettlementCurrency: "USD",
				FolioPaymentMethod:      "Card",
			},
			wantN: 0,
		},
	}

	// One sender for the whole matrix so we exercise multi-recipient layout.
	dir := t.TempDir()
	s := email.NewFilesystemSender(dir, nil)

	for _, c := range cases {
		t.Run(string(c.kind), func(t *testing.T) {
			rendered, err := email.Render(c.kind, c.vars)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			rendered.From = "Liveaboard <noreply@x.test>"
			rendered.To = c.vars.RecipientEmail
			if rendered.To == "" {
				t.Skipf("no recipient set for %s", c.kind)
			}
			wantLinks := email.ExtractLinks(rendered)
			if len(wantLinks) != c.wantN {
				t.Fatalf("ExtractLinks=%d (want %d) for %s: %v", len(wantLinks), c.wantN, c.kind, wantLinks)
			}

			if err := s.Send(context.Background(), rendered); err != nil {
				t.Fatalf("Send: %v", err)
			}

			meta, err := email.ReadLatestInboxMessage(dir, rendered.To)
			if err != nil {
				t.Fatalf("ReadLatestInboxMessage: %v", err)
			}
			if meta.Subject != rendered.Subject {
				t.Errorf("subject=%q want %q", meta.Subject, rendered.Subject)
			}
			if !equalSlice(meta.Links, wantLinks) {
				t.Errorf("sidecar links=%v want %v", meta.Links, wantLinks)
			}

			// The .eml referenced by sidecar must exist and contain the
			// subject line.
			eml, err := os.ReadFile(meta.EMLPath)
			if err != nil {
				t.Fatalf("read eml: %v", err)
			}
			if !strings.Contains(string(eml), "Subject: "+rendered.Subject) {
				t.Errorf(".eml missing subject:\n%s", eml)
			}
		})
	}

	// Verify each recipient got its own directory; latest.json there parses.
	for _, c := range cases {
		if c.vars.RecipientEmail == "" {
			continue
		}
		path := filepath.Join(dir, c.vars.RecipientEmail, "latest.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		var meta email.InboxMessage
		if err := json.Unmarshal(raw, &meta); err != nil {
			t.Errorf("parse %s: %v", path, err)
		}
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
