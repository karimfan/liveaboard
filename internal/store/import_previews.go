package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ImportPreview is a parsed spreadsheet upload, persisted server-side
// so the SPA can call `commit` with a preview_id reference instead of
// re-uploading the file. Sprint 012; rows expire after 1 hour.
type ImportPreview struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	StartedBy      uuid.UUID
	Filename       string
	Payload        json.RawMessage
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

const importPreviewColumns = `id, organization_id, started_by, filename, payload, created_at, expires_at`

func scanImportPreview(row interface {
	Scan(dest ...any) error
}, p *ImportPreview) error {
	return row.Scan(
		&p.ID, &p.OrganizationID, &p.StartedBy, &p.Filename, &p.Payload, &p.CreatedAt, &p.ExpiresAt,
	)
}

// CreateImportPreview persists a parsed spreadsheet payload. The
// caller serializes the payload to JSON; we store it as jsonb. TTL is
// hardcoded to 1 hour by the table's DEFAULT.
func (p *Pool) CreateImportPreview(
	ctx context.Context,
	orgID, userID uuid.UUID,
	filename string,
	payload []byte,
) (*ImportPreview, error) {
	out := &ImportPreview{}
	err := scanImportPreview(p.QueryRow(ctx, `
		INSERT INTO import_previews (organization_id, started_by, filename, payload)
		VALUES ($1, $2, $3, $4::jsonb)
		RETURNING `+importPreviewColumns,
		orgID, userID, filename, payload,
	), out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ImportPreviewByID looks up a preview scoped to org and not yet
// expired. Returns ErrNotFound for missing AND expired (callers don't
// need to distinguish).
func (p *Pool) ImportPreviewByID(ctx context.Context, orgID, previewID uuid.UUID, now time.Time) (*ImportPreview, error) {
	out := &ImportPreview{}
	err := scanImportPreview(p.QueryRow(ctx, `
		SELECT `+importPreviewColumns+` FROM import_previews
		WHERE id = $1 AND organization_id = $2 AND expires_at > $3
	`, previewID, orgID, now), out)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteImportPreview removes a preview row by id (org-scoped). Run
// after a successful commit so a stale row isn't reused.
func (p *Pool) DeleteImportPreview(ctx context.Context, orgID, previewID uuid.UUID) error {
	_, err := p.Exec(ctx, `
		DELETE FROM import_previews WHERE id = $1 AND organization_id = $2
	`, previewID, orgID)
	return err
}

// DeleteExpiredImportPreviews drops every preview whose expires_at has
// passed. Called once at server startup; not on a timer because the
// table is small and one cleanup per restart is enough.
func (p *Pool) DeleteExpiredImportPreviews(ctx context.Context, now time.Time) (int, error) {
	tag, err := p.Exec(ctx, `DELETE FROM import_previews WHERE expires_at < $1`, now)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
