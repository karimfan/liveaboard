package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type GuestDocument struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	TripID                uuid.UUID
	TripGuestID           uuid.UUID
	UploadedByUserID      *uuid.UUID
	UploadedByGuestUserID *uuid.UUID
	Category              string
	DisplayName           string
	OriginalFilename      string
	ContentType           string
	SizeBytes             int64
	SHA256Hex             string
	StorageKey            string
	Notes                 *string
	ArchivedAt            *time.Time
	ArchivedByUserID      *uuid.UUID
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type GuestDocumentInput struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	TripID                uuid.UUID
	TripGuestID           uuid.UUID
	UploadedByUserID      *uuid.UUID
	UploadedByGuestUserID *uuid.UUID
	Category              string
	DisplayName           string
	OriginalFilename      string
	ContentType           string
	SizeBytes             int64
	SHA256Hex             string
	StorageKey            string
	Notes                 *string
}

const guestDocumentColumns = `id, organization_id, trip_id, trip_guest_id, uploaded_by_user_id, uploaded_by_guest_user_id,
	category, display_name, original_filename, content_type, size_bytes, sha256_hex, storage_key, notes,
	archived_at, archived_by_user_id, created_at, updated_at`

func (p *Pool) CreateGuestDocument(ctx context.Context, in GuestDocumentInput) (*GuestDocument, error) {
	if in.OrganizationID == uuid.Nil || in.TripID == uuid.Nil || in.TripGuestID == uuid.Nil || in.StorageKey == "" {
		return nil, ErrInvalidInput
	}
	if in.ID == uuid.Nil {
		in.ID = uuid.New()
	}
	doc := &GuestDocument{}
	err := scanGuestDocument(p.QueryRow(ctx, `
		INSERT INTO guest_documents (
			id, organization_id, trip_id, trip_guest_id, uploaded_by_user_id, uploaded_by_guest_user_id,
			category, display_name, original_filename, content_type, size_bytes, sha256_hex, storage_key, notes
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING `+guestDocumentColumns,
		in.ID, in.OrganizationID, in.TripID, in.TripGuestID, in.UploadedByUserID, in.UploadedByGuestUserID,
		in.Category, in.DisplayName, in.OriginalFilename, in.ContentType, in.SizeBytes, in.SHA256Hex, in.StorageKey, in.Notes,
	), doc)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (p *Pool) GuestDocumentsForTripGuest(ctx context.Context, orgID, tripID, tripGuestID uuid.UUID, includeArchived bool) ([]*GuestDocument, error) {
	sql := `SELECT ` + guestDocumentColumns + ` FROM guest_documents WHERE organization_id = $1 AND trip_id = $2 AND trip_guest_id = $3`
	if !includeArchived {
		sql += ` AND archived_at IS NULL`
	}
	sql += ` ORDER BY created_at DESC`
	rows, err := p.Query(ctx, sql, orgID, tripID, tripGuestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*GuestDocument
	for rows.Next() {
		doc := &GuestDocument{}
		if err := scanGuestDocument(rows, doc); err != nil {
			return nil, err
		}
		out = append(out, doc)
	}
	return out, rows.Err()
}

func (p *Pool) GuestDocumentByID(ctx context.Context, orgID, tripID, tripGuestID, documentID uuid.UUID) (*GuestDocument, error) {
	doc := &GuestDocument{}
	err := scanGuestDocument(p.QueryRow(ctx, `
		SELECT `+guestDocumentColumns+`
		FROM guest_documents
		WHERE organization_id = $1 AND trip_id = $2 AND trip_guest_id = $3 AND id = $4
	`, orgID, tripID, tripGuestID, documentID), doc)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (p *Pool) ArchiveGuestDocument(ctx context.Context, orgID, tripID, tripGuestID, documentID, actorID uuid.UUID) (*GuestDocument, error) {
	doc := &GuestDocument{}
	err := scanGuestDocument(p.QueryRow(ctx, `
		UPDATE guest_documents
		SET archived_at = COALESCE(archived_at, now()), archived_by_user_id = COALESCE(archived_by_user_id, $5), updated_at = now()
		WHERE organization_id = $1 AND trip_id = $2 AND trip_guest_id = $3 AND id = $4
		RETURNING `+guestDocumentColumns,
		orgID, tripID, tripGuestID, documentID, actorID,
	), doc)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (p *Pool) TripGuestByID(ctx context.Context, orgID, tripID, tripGuestID uuid.UUID) (*TripGuest, error) {
	g := &TripGuest{}
	err := scanTripGuest(p.QueryRow(ctx, `
		SELECT `+tripGuestColumns+`
		FROM trip_guests
		WHERE organization_id = $1 AND trip_id = $2 AND id = $3
	`, orgID, tripID, tripGuestID), g)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return g, nil
}

func scanGuestDocument(row interface{ Scan(dest ...any) error }, doc *GuestDocument) error {
	return row.Scan(
		&doc.ID, &doc.OrganizationID, &doc.TripID, &doc.TripGuestID, &doc.UploadedByUserID, &doc.UploadedByGuestUserID,
		&doc.Category, &doc.DisplayName, &doc.OriginalFilename, &doc.ContentType, &doc.SizeBytes, &doc.SHA256Hex,
		&doc.StorageKey, &doc.Notes, &doc.ArchivedAt, &doc.ArchivedByUserID, &doc.CreatedAt, &doc.UpdatedAt,
	)
}
