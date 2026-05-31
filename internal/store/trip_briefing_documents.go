package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TripBriefingDocument is an operator-uploaded PDF (or other
// document) attached to a trip. Guests in the trip portal can
// read it; nobody outside the trip can. Sprint 026 Anchor 2.
type TripBriefingDocument struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	TripID           uuid.UUID
	Title            string
	FilePath         string
	ContentType      string
	UploadedByUserID uuid.UUID
	CreatedAt        time.Time
}

type CreateTripBriefingDocumentInput struct {
	OrganizationID   uuid.UUID
	TripID           uuid.UUID
	Title            string
	FilePath         string
	ContentType      string
	UploadedByUserID uuid.UUID
}

const tripBriefingDocumentColumns = `
	id, organization_id, trip_id, title, file_path, content_type,
	uploaded_by_user_id, created_at
`

func scanTripBriefingDocument(row interface{ Scan(dest ...any) error }, d *TripBriefingDocument) error {
	return row.Scan(
		&d.ID, &d.OrganizationID, &d.TripID, &d.Title, &d.FilePath, &d.ContentType,
		&d.UploadedByUserID, &d.CreatedAt,
	)
}

func (p *Pool) CreateTripBriefingDocument(ctx context.Context, in CreateTripBriefingDocumentInput) (*TripBriefingDocument, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, errors.New("briefing: title required")
	}
	if in.OrganizationID == uuid.Nil || in.TripID == uuid.Nil || in.UploadedByUserID == uuid.Nil {
		return nil, errors.New("briefing: ids required")
	}
	out := &TripBriefingDocument{}
	err := scanTripBriefingDocument(p.QueryRow(ctx, `
		INSERT INTO trip_briefing_documents (
			organization_id, trip_id, title, file_path, content_type, uploaded_by_user_id
		)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING `+tripBriefingDocumentColumns,
		in.OrganizationID, in.TripID, title, in.FilePath, in.ContentType, in.UploadedByUserID,
	), out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pool) ListTripBriefingDocuments(ctx context.Context, orgID, tripID uuid.UUID) ([]*TripBriefingDocument, error) {
	rows, err := p.Query(ctx, `
		SELECT `+tripBriefingDocumentColumns+`
		FROM trip_briefing_documents
		WHERE organization_id = $1 AND trip_id = $2
		ORDER BY created_at ASC
	`, orgID, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*TripBriefingDocument
	for rows.Next() {
		d := &TripBriefingDocument{}
		if err := scanTripBriefingDocument(rows, d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (p *Pool) DeleteTripBriefingDocument(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := p.Exec(ctx, `
		DELETE FROM trip_briefing_documents
		WHERE organization_id = $1 AND id = $2
	`, orgID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
