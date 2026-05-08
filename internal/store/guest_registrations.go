package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type GuestTripRegistration struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	TripID         uuid.UUID
	TripGuestID    uuid.UUID
	GuestUserID    uuid.UUID
	Status         string
	Payload        json.RawMessage
	SubmittedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

const guestRegistrationColumns = `id, organization_id, trip_id, trip_guest_id, guest_user_id, status, payload, submitted_at, created_at, updated_at`

func scanGuestRegistration(row interface{ Scan(dest ...any) error }, r *GuestTripRegistration) error {
	return row.Scan(&r.ID, &r.OrganizationID, &r.TripID, &r.TripGuestID, &r.GuestUserID, &r.Status, &r.Payload, &r.SubmittedAt, &r.CreatedAt, &r.UpdatedAt)
}

func (p *Pool) GuestRegistrationByTripGuest(ctx context.Context, tripGuestID uuid.UUID) (*GuestTripRegistration, error) {
	r := &GuestTripRegistration{}
	err := scanGuestRegistration(p.QueryRow(ctx, `SELECT `+guestRegistrationColumns+` FROM guest_trip_registrations WHERE trip_guest_id = $1`, tripGuestID), r)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (p *Pool) SaveGuestRegistration(ctx context.Context, guest *TripGuest, guestUserID uuid.UUID, payload []byte, status string, submittedAt *time.Time) (*GuestTripRegistration, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	r := &GuestTripRegistration{}
	err = scanGuestRegistration(tx.QueryRow(ctx, `
		INSERT INTO guest_trip_registrations (
			organization_id, trip_id, trip_guest_id, guest_user_id, status, payload, submitted_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (trip_guest_id) DO UPDATE SET
			status = EXCLUDED.status,
			payload = EXCLUDED.payload,
			submitted_at = EXCLUDED.submitted_at,
			updated_at = now()
		RETURNING `+guestRegistrationColumns,
		guest.OrganizationID, guest.TripID, guest.ID, guestUserID, status, payload, submittedAt,
	), r)
	if err != nil {
		return nil, err
	}
	if status == "submitted" {
		// Preserve the original submission moment across re-submits so the
		// manifest "Submitted" timestamp doesn't drift every time the
		// guest edits their registration after first submit.
		if _, err := tx.Exec(ctx, `
			UPDATE trip_guests
			SET registration_submitted_at = COALESCE(registration_submitted_at, $2), updated_at = now()
			WHERE id = $1
		`, guest.ID, submittedAt); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r, nil
}
