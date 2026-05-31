package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Lead is a prospective-guest inquiry captured from the public
// catalog. Sprint 026 Anchor 1.
type Lead struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	TripID         *uuid.UUID
	Email          string
	Name           string
	PartySize      int
	DateWindow     *string
	Notes          *string
	SourceIP       *string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateLeadInput struct {
	OrganizationID uuid.UUID
	TripID         *uuid.UUID
	Email          string
	Name           string
	PartySize      int
	DateWindow     *string
	Notes          *string
	SourceIP       *string
}

const leadColumns = `
	id, organization_id, trip_id, email, name, party_size,
	date_window, notes, source_ip, status, created_at, updated_at
`

func scanLead(row interface{ Scan(dest ...any) error }, l *Lead) error {
	return row.Scan(
		&l.ID, &l.OrganizationID, &l.TripID, &l.Email, &l.Name, &l.PartySize,
		&l.DateWindow, &l.Notes, &l.SourceIP, &l.Status, &l.CreatedAt, &l.UpdatedAt,
	)
}

func (p *Pool) CreateLead(ctx context.Context, in CreateLeadInput) (*Lead, error) {
	if in.OrganizationID == uuid.Nil {
		return nil, errors.New("lead: organization_id required")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errors.New("lead: name required")
	}
	email := strings.TrimSpace(in.Email)
	if email == "" {
		return nil, errors.New("lead: email required")
	}
	if in.PartySize < 1 || in.PartySize > 50 {
		return nil, errors.New("lead: party_size must be 1..50")
	}
	out := &Lead{}
	err := scanLead(p.QueryRow(ctx, `
		INSERT INTO leads (organization_id, trip_id, email, name, party_size, date_window, notes, source_ip)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING `+leadColumns,
		in.OrganizationID, in.TripID, email, name, in.PartySize,
		in.DateWindow, in.Notes, in.SourceIP,
	), out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pool) GetLead(ctx context.Context, orgID, id uuid.UUID) (*Lead, error) {
	out := &Lead{}
	err := scanLead(p.QueryRow(ctx, `
		SELECT `+leadColumns+`
		FROM leads
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

func (p *Pool) ListLeads(ctx context.Context, orgID uuid.UUID, status string) ([]*Lead, error) {
	q := `
		SELECT ` + leadColumns + `
		FROM leads
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
	var out []*Lead
	for rows.Next() {
		l := &Lead{}
		if err := scanLead(rows, l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (p *Pool) UpdateLeadStatus(ctx context.Context, orgID, id uuid.UUID, status string) error {
	switch status {
	case "new", "contacted", "quoted", "won", "lost", "archived":
	default:
		return errors.New("lead: invalid status")
	}
	tag, err := p.Exec(ctx, `
		UPDATE leads SET status = $1, updated_at = now()
		WHERE organization_id = $2 AND id = $3
	`, status, orgID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
