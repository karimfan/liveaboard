package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ImportJob is one record of a trip-import operation. Sprint 012 has
// two sources: 'liveaboard_com' (kicked off async, polled by the SPA)
// and 'spreadsheet' (created synchronously by the commit handler once
// the upload + preview + mapping have already happened).
type ImportJob struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	StartedBy      uuid.UUID

	// 'liveaboard_com' or 'spreadsheet'.
	Source string
	// For liveaboard.com: the URL. For spreadsheet: the original filename.
	SourceInput string

	// 'queued', 'running', 'succeeded', 'failed'.
	Status string

	BoatsInserted *int
	BoatsUpdated  *int
	TripsInserted *int
	TripsUpdated  *int
	TripsDeleted  *int
	ErrorMessage  *string

	StartedAt   time.Time
	CompletedAt *time.Time
}

const (
	ImportSourceLiveaboard  = "liveaboard_com"
	ImportSourceSpreadsheet = "spreadsheet"

	ImportStatusQueued    = "queued"
	ImportStatusRunning   = "running"
	ImportStatusSucceeded = "succeeded"
	ImportStatusFailed    = "failed"
)

const importJobColumns = `id, organization_id, started_by, source, source_input,
	status, boats_inserted, boats_updated, trips_inserted, trips_updated, trips_deleted,
	error_message, started_at, completed_at`

func scanImportJob(row interface {
	Scan(dest ...any) error
}, j *ImportJob) error {
	return row.Scan(
		&j.ID, &j.OrganizationID, &j.StartedBy, &j.Source, &j.SourceInput,
		&j.Status, &j.BoatsInserted, &j.BoatsUpdated, &j.TripsInserted, &j.TripsUpdated, &j.TripsDeleted,
		&j.ErrorMessage, &j.StartedAt, &j.CompletedAt,
	)
}

// CreateImportJob inserts a new job in 'queued' status. The runner
// flips it to 'running' when its goroutine starts.
func (p *Pool) CreateImportJob(ctx context.Context, orgID, userID uuid.UUID, source, sourceInput string) (*ImportJob, error) {
	j := &ImportJob{}
	err := scanImportJob(p.QueryRow(ctx, `
		INSERT INTO import_jobs (organization_id, started_by, source, source_input, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+importJobColumns,
		orgID, userID, source, sourceInput, ImportStatusQueued,
	), j)
	if err != nil {
		return nil, err
	}
	return j, nil
}

// ImportJobByID returns a job scoped to its org for tenant isolation.
func (p *Pool) ImportJobByID(ctx context.Context, orgID, jobID uuid.UUID) (*ImportJob, error) {
	j := &ImportJob{}
	err := scanImportJob(p.QueryRow(ctx, `
		SELECT `+importJobColumns+` FROM import_jobs
		WHERE id = $1 AND organization_id = $2
	`, jobID, orgID), j)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return j, nil
}

// MarkImportJobRunning flips status from 'queued' to 'running'. Idempotent.
func (p *Pool) MarkImportJobRunning(ctx context.Context, jobID uuid.UUID) error {
	_, err := p.Exec(ctx, `
		UPDATE import_jobs SET status = $2 WHERE id = $1 AND status = $3
	`, jobID, ImportStatusRunning, ImportStatusQueued)
	return err
}

// ImportResult is the count payload written when a job ends.
type ImportResult struct {
	BoatsInserted int
	BoatsUpdated  int
	TripsInserted int
	TripsUpdated  int
	TripsDeleted  int
}

// MarkImportJobSucceeded writes the result counts and flips status
// to 'succeeded'. completed_at = now().
func (p *Pool) MarkImportJobSucceeded(ctx context.Context, jobID uuid.UUID, r ImportResult) error {
	_, err := p.Exec(ctx, `
		UPDATE import_jobs
		   SET status = $2,
		       boats_inserted = $3, boats_updated = $4,
		       trips_inserted = $5, trips_updated = $6, trips_deleted = $7,
		       completed_at = now()
		 WHERE id = $1
	`, jobID, ImportStatusSucceeded,
		r.BoatsInserted, r.BoatsUpdated,
		r.TripsInserted, r.TripsUpdated, r.TripsDeleted)
	return err
}

// MarkImportJobFailed records an error message and flips status to 'failed'.
func (p *Pool) MarkImportJobFailed(ctx context.Context, jobID uuid.UUID, msg string) error {
	_, err := p.Exec(ctx, `
		UPDATE import_jobs
		   SET status = $2, error_message = $3, completed_at = now()
		 WHERE id = $1
	`, jobID, ImportStatusFailed, msg)
	return err
}

// MarkInFlightImportJobsFailed flips any 'queued' or 'running' job in
// the table to 'failed' with the given message. Used at server startup
// to clean up jobs orphaned by a previous shutdown.
func (p *Pool) MarkInFlightImportJobsFailed(ctx context.Context, msg string) (int, error) {
	tag, err := p.Exec(ctx, `
		UPDATE import_jobs
		   SET status = $1, error_message = $2, completed_at = now()
		 WHERE status IN ($3, $4)
	`, ImportStatusFailed, msg, ImportStatusQueued, ImportStatusRunning)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
