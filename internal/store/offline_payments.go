package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// OfflinePayment is an operator-confirmed deposit or refund
// record. No money moves through Liveaboard; this table is the
// platform's ledger of what the operator says happened offline.
// Sprint 026 Anchor 1.
type OfflinePayment struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	QuoteID          *uuid.UUID
	TripGuestID      *uuid.UUID
	Direction        string
	AmountCents      int64
	Currency         string
	Method           string
	ReceivedOn       *time.Time
	Reference        *string
	Notes            *string
	RecordedByUserID uuid.UUID
	RecordedAt       time.Time
}

type RecordOfflinePaymentInput struct {
	OrganizationID   uuid.UUID
	QuoteID          *uuid.UUID
	TripGuestID      *uuid.UUID
	Direction        string
	AmountCents      int64
	Currency         string
	Method           string
	ReceivedOn       *time.Time
	Reference        *string
	Notes            *string
	RecordedByUserID uuid.UUID
}

const offlinePaymentColumns = `
	id, organization_id, quote_id, trip_guest_id, direction,
	amount_cents, currency, method, received_on, reference, notes,
	recorded_by_user_id, recorded_at
`

func scanOfflinePayment(row interface{ Scan(dest ...any) error }, p *OfflinePayment) error {
	return row.Scan(
		&p.ID, &p.OrganizationID, &p.QuoteID, &p.TripGuestID, &p.Direction,
		&p.AmountCents, &p.Currency, &p.Method, &p.ReceivedOn, &p.Reference, &p.Notes,
		&p.RecordedByUserID, &p.RecordedAt,
	)
}

func (p *Pool) RecordOfflinePayment(ctx context.Context, in RecordOfflinePaymentInput) (*OfflinePayment, error) {
	if in.OrganizationID == uuid.Nil || in.RecordedByUserID == uuid.Nil {
		return nil, errors.New("offline_payment: ids required")
	}
	switch in.Direction {
	case "deposit", "refund":
	default:
		return nil, errors.New("offline_payment: direction must be deposit|refund")
	}
	switch in.Method {
	case "bank_transfer", "cash", "wise", "card_external", "other":
	default:
		return nil, errors.New("offline_payment: invalid method")
	}
	if in.AmountCents <= 0 {
		return nil, errors.New("offline_payment: amount_cents must be > 0")
	}
	cur, err := NormalizeCurrency(in.Currency)
	if err != nil {
		return nil, err
	}
	out := &OfflinePayment{}
	err = scanOfflinePayment(p.QueryRow(ctx, `
		INSERT INTO offline_payments (
			organization_id, quote_id, trip_guest_id, direction,
			amount_cents, currency, method, received_on, reference, notes,
			recorded_by_user_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING `+offlinePaymentColumns,
		in.OrganizationID, in.QuoteID, in.TripGuestID, in.Direction,
		in.AmountCents, cur, in.Method, in.ReceivedOn, in.Reference, in.Notes,
		in.RecordedByUserID,
	), out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pool) ListOfflinePaymentsForQuote(ctx context.Context, orgID, quoteID uuid.UUID) ([]*OfflinePayment, error) {
	rows, err := p.Query(ctx, `
		SELECT `+offlinePaymentColumns+`
		FROM offline_payments
		WHERE organization_id = $1 AND quote_id = $2
		ORDER BY recorded_at ASC
	`, orgID, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*OfflinePayment
	for rows.Next() {
		op := &OfflinePayment{}
		if err := scanOfflinePayment(rows, op); err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	return out, rows.Err()
}

// NetDepositForQuote returns deposits minus refunds. The caller
// uses this to decide whether a refund tips the quote to "fully
// refunded" (sum == 0) and the berth should be released.
func (p *Pool) NetDepositForQuote(ctx context.Context, orgID, quoteID uuid.UUID) (int64, error) {
	var net int64
	err := p.QueryRow(ctx, `
		SELECT COALESCE(SUM(
			CASE direction WHEN 'deposit' THEN amount_cents ELSE -amount_cents END
		), 0)
		FROM offline_payments
		WHERE organization_id = $1 AND quote_id = $2
	`, orgID, quoteID).Scan(&net)
	return net, err
}
