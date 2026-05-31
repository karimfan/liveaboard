package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GuestCertification is a structured wrapper around a guest's
// dive certification document. The doc itself lives in
// guest_documents (Sprint 015); this row adds the metadata
// operators need (type, expiry, etc.). Sprint 026 Anchor 2.
type GuestCertification struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
	TripGuestID         uuid.UUID
	GuestDocumentID     *uuid.UUID
	CertificationType   string
	Issuer              *string
	CertificationNumber *string
	ExpiresOn           *time.Time
	VerifiedAt          *time.Time
	VerifiedByUserID    *uuid.UUID
	VerificationNotes   *string
	CreatedAt           time.Time
}

type CreateGuestCertificationInput struct {
	OrganizationID      uuid.UUID
	TripGuestID         uuid.UUID
	GuestDocumentID     *uuid.UUID
	CertificationType   string
	Issuer              *string
	CertificationNumber *string
	ExpiresOn           *time.Time
}

const guestCertificationColumns = `
	id, organization_id, trip_guest_id, guest_document_id, certification_type,
	issuer, certification_number, expires_on,
	verified_at, verified_by_user_id, verification_notes, created_at
`

func scanGuestCertification(row interface{ Scan(dest ...any) error }, c *GuestCertification) error {
	return row.Scan(
		&c.ID, &c.OrganizationID, &c.TripGuestID, &c.GuestDocumentID, &c.CertificationType,
		&c.Issuer, &c.CertificationNumber, &c.ExpiresOn,
		&c.VerifiedAt, &c.VerifiedByUserID, &c.VerificationNotes, &c.CreatedAt,
	)
}

func (p *Pool) CreateGuestCertification(ctx context.Context, in CreateGuestCertificationInput) (*GuestCertification, error) {
	if in.OrganizationID == uuid.Nil || in.TripGuestID == uuid.Nil {
		return nil, errors.New("guest_certification: ids required")
	}
	t := strings.TrimSpace(in.CertificationType)
	if t == "" {
		return nil, errors.New("guest_certification: certification_type required")
	}
	out := &GuestCertification{}
	err := scanGuestCertification(p.QueryRow(ctx, `
		INSERT INTO guest_certifications (
			organization_id, trip_guest_id, guest_document_id, certification_type,
			issuer, certification_number, expires_on
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING `+guestCertificationColumns,
		in.OrganizationID, in.TripGuestID, in.GuestDocumentID, t,
		in.Issuer, in.CertificationNumber, in.ExpiresOn,
	), out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pool) ListGuestCertifications(ctx context.Context, orgID, tripGuestID uuid.UUID) ([]*GuestCertification, error) {
	rows, err := p.Query(ctx, `
		SELECT `+guestCertificationColumns+`
		FROM guest_certifications
		WHERE organization_id = $1 AND trip_guest_id = $2
		ORDER BY created_at DESC
	`, orgID, tripGuestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*GuestCertification
	for rows.Next() {
		c := &GuestCertification{}
		if err := scanGuestCertification(rows, c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (p *Pool) MarkGuestCertificationVerified(ctx context.Context, orgID, id, byUserID uuid.UUID, notes string, now time.Time) (*GuestCertification, error) {
	var notesPtr *string
	if n := strings.TrimSpace(notes); n != "" {
		notesPtr = &n
	}
	out := &GuestCertification{}
	err := scanGuestCertification(p.QueryRow(ctx, `
		UPDATE guest_certifications
		SET verified_at = $1, verified_by_user_id = $2, verification_notes = $3
		WHERE organization_id = $4 AND id = $5
		RETURNING `+guestCertificationColumns,
		now, byUserID, notesPtr, orgID, id,
	), out)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}
