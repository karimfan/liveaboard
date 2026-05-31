package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BookingQuote is an operator-issued price proposal tied to a trip
// and a magic token a guest can use to acknowledge intent to send
// the deposit offline. Sprint 026 Anchor 1.
type BookingQuote struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	LeadID           *uuid.UUID
	TripID           uuid.UUID
	TokenHash        []byte
	GuestName        string
	GuestEmail       string
	PartySize        int
	QuotedTotalCents int64
	DepositDueCents  int64
	Currency         string
	Status           string
	AcceptedAt       *time.Time
	HoldExpiresAt    *time.Time
	CancelledReason  *string
	CreatedByUserID  uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreateBookingQuoteInput struct {
	OrganizationID   uuid.UUID
	LeadID           *uuid.UUID
	TripID           uuid.UUID
	GuestName        string
	GuestEmail       string
	PartySize        int
	QuotedTotalCents int64
	DepositDueCents  int64
	Currency         string
	CreatedByUserID  uuid.UUID
}

// CreatedBookingQuote bundles the inserted row with the raw token
// the caller needs to embed in the email link — the token is only
// returned at creation time, never from a subsequent read.
type CreatedBookingQuote struct {
	Quote *BookingQuote
	Token string
}

const bookingQuoteColumns = `
	id, organization_id, lead_id, trip_id, token_hash,
	guest_name, guest_email, party_size,
	quoted_total_cents, deposit_due_cents, currency, status,
	accepted_at, hold_expires_at, cancelled_reason,
	created_by_user_id, created_at, updated_at
`

func scanBookingQuote(row interface{ Scan(dest ...any) error }, q *BookingQuote) error {
	return row.Scan(
		&q.ID, &q.OrganizationID, &q.LeadID, &q.TripID, &q.TokenHash,
		&q.GuestName, &q.GuestEmail, &q.PartySize,
		&q.QuotedTotalCents, &q.DepositDueCents, &q.Currency, &q.Status,
		&q.AcceptedAt, &q.HoldExpiresAt, &q.CancelledReason,
		&q.CreatedByUserID, &q.CreatedAt, &q.UpdatedAt,
	)
}

// newQuoteToken returns a base64-url-safe 32-byte random token
// and its SHA-256 digest. The raw token is what the guest sees in
// the email; the hash is what the database stores.
func newQuoteToken() (raw string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:], nil
}

// HashQuoteToken is the public hashing function the handler layer
// uses when resolving a token from a URL path parameter.
func HashQuoteToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func (p *Pool) CreateBookingQuote(ctx context.Context, in CreateBookingQuoteInput) (*CreatedBookingQuote, error) {
	if in.OrganizationID == uuid.Nil || in.TripID == uuid.Nil || in.CreatedByUserID == uuid.Nil {
		return nil, errors.New("booking_quote: ids required")
	}
	if strings.TrimSpace(in.GuestName) == "" || strings.TrimSpace(in.GuestEmail) == "" {
		return nil, errors.New("booking_quote: guest name/email required")
	}
	if in.PartySize <= 0 {
		return nil, errors.New("booking_quote: party_size > 0")
	}
	if in.QuotedTotalCents < 0 || in.DepositDueCents < 0 {
		return nil, errors.New("booking_quote: amounts must be non-negative")
	}
	cur, err := NormalizeCurrency(in.Currency)
	if err != nil {
		return nil, err
	}
	raw, hash, err := newQuoteToken()
	if err != nil {
		return nil, err
	}
	out := &BookingQuote{}
	err = scanBookingQuote(p.QueryRow(ctx, `
		INSERT INTO booking_quotes (
			organization_id, lead_id, trip_id, token_hash,
			guest_name, guest_email, party_size,
			quoted_total_cents, deposit_due_cents, currency,
			status, created_by_user_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'draft',$11)
		RETURNING `+bookingQuoteColumns,
		in.OrganizationID, in.LeadID, in.TripID, hash,
		strings.TrimSpace(in.GuestName), strings.TrimSpace(in.GuestEmail), in.PartySize,
		in.QuotedTotalCents, in.DepositDueCents, cur,
		in.CreatedByUserID,
	), out)
	if err != nil {
		return nil, err
	}
	return &CreatedBookingQuote{Quote: out, Token: raw}, nil
}

func (p *Pool) GetBookingQuote(ctx context.Context, orgID, id uuid.UUID) (*BookingQuote, error) {
	out := &BookingQuote{}
	err := scanBookingQuote(p.QueryRow(ctx, `
		SELECT `+bookingQuoteColumns+`
		FROM booking_quotes
		WHERE organization_id = $1 AND id = $2
	`, orgID, id), out)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetBookingQuoteByTokenHash resolves a quote without scoping by
// organization — the token itself is the only authentication, and
// public quote handlers need this for /api/public/quotes/:token.
func (p *Pool) GetBookingQuoteByTokenHash(ctx context.Context, hash []byte) (*BookingQuote, error) {
	out := &BookingQuote{}
	err := scanBookingQuote(p.QueryRow(ctx, `
		SELECT `+bookingQuoteColumns+`
		FROM booking_quotes
		WHERE token_hash = $1
	`, hash), out)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pool) ListBookingQuotes(ctx context.Context, orgID uuid.UUID, status string) ([]*BookingQuote, error) {
	q := `
		SELECT ` + bookingQuoteColumns + `
		FROM booking_quotes
		WHERE organization_id = $1
	`
	args := []any{orgID}
	if status != "" {
		q += " AND status = $2"
		args = append(args, status)
	}
	q += " ORDER BY created_at DESC"
	rows, err := p.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*BookingQuote
	for rows.Next() {
		bq := &BookingQuote{}
		if err := scanBookingQuote(rows, bq); err != nil {
			return nil, err
		}
		out = append(out, bq)
	}
	return out, rows.Err()
}

// MarkBookingQuoteSent flips status from draft → sent and records
// the send timestamp via updated_at. The token was created at
// insert time; the handler emails it now.
func (p *Pool) MarkBookingQuoteSent(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := p.Exec(ctx, `
		UPDATE booking_quotes SET status = 'sent', updated_at = now()
		WHERE organization_id = $1 AND id = $2 AND status = 'draft'
	`, orgID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("booking_quote: not draft or not found")
	}
	return nil
}

// AcknowledgeBookingQuote flips status from sent → deposit_pending
// and sets accepted_at. Called from the public handler with the
// token hash; org is derived from the quote row.
func (p *Pool) AcknowledgeBookingQuote(ctx context.Context, hash []byte, now time.Time) (*BookingQuote, error) {
	out := &BookingQuote{}
	err := scanBookingQuote(p.QueryRow(ctx, `
		UPDATE booking_quotes
		SET status = 'deposit_pending', accepted_at = $1, updated_at = now()
		WHERE token_hash = $2 AND status = 'sent'
		RETURNING `+bookingQuoteColumns,
		now, hash,
	), out)
	if isNoRows(err) {
		return nil, errors.New("booking_quote: not sent or not found")
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// HoldBookingQuote moves an accepted quote into the held state
// with a hold_expires_at window. Called when a deposit is
// recorded. Returns the updated quote.
func (p *Pool) HoldBookingQuote(ctx context.Context, orgID, id uuid.UUID, holdExpiresAt time.Time) (*BookingQuote, error) {
	out := &BookingQuote{}
	err := scanBookingQuote(p.QueryRow(ctx, `
		UPDATE booking_quotes
		SET status = 'held', hold_expires_at = $1, updated_at = now()
		WHERE organization_id = $2 AND id = $3
		  AND status IN ('accepted','deposit_pending')
		RETURNING `+bookingQuoteColumns,
		holdExpiresAt, orgID, id,
	), out)
	if isNoRows(err) {
		return nil, errors.New("booking_quote: not in accepted/deposit_pending or not found")
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CancelBookingQuote moves a quote into the cancelled state with
// an optional reason. Caller decides whether the hold should be
// released elsewhere.
func (p *Pool) CancelBookingQuote(ctx context.Context, orgID, id uuid.UUID, reason string) (*BookingQuote, error) {
	var reasonPtr *string
	if r := strings.TrimSpace(reason); r != "" {
		reasonPtr = &r
	}
	out := &BookingQuote{}
	err := scanBookingQuote(p.QueryRow(ctx, `
		UPDATE booking_quotes
		SET status = 'cancelled', cancelled_reason = $1, updated_at = now()
		WHERE organization_id = $2 AND id = $3
		  AND status NOT IN ('cancelled','expired')
		RETURNING `+bookingQuoteColumns,
		reasonPtr, orgID, id,
	), out)
	if isNoRows(err) {
		return nil, errors.New("booking_quote: already cancelled or not found")
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ExpireBookingQuotesPastHold flips any held quotes whose
// hold_expires_at has passed into expired. Intended to be called
// from a periodic sweep or lazily on the next read.
func (p *Pool) ExpireBookingQuotesPastHold(ctx context.Context, now time.Time) (int, error) {
	tag, err := p.Exec(ctx, `
		UPDATE booking_quotes
		SET status = 'expired', updated_at = now()
		WHERE status = 'held' AND hold_expires_at IS NOT NULL AND hold_expires_at < $1
	`, now)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
