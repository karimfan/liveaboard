package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// OrganizationByID returns an organization by id. The caller is
// responsible for asserting that the requesting user belongs to the
// org — this is a low-level read.
func (p *Pool) OrganizationByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	org := &Organization{}
	err := p.QueryRow(ctx, `
		SELECT id, name, currency, created_at, updated_at FROM organizations WHERE id = $1
	`, id).Scan(&org.ID, &org.Name, &org.Currency, &org.CreatedAt, &org.UpdatedAt)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return org, nil
}

// ErrOrgAmbiguous indicates that OrganizationByName matched more than
// one row.
var ErrOrgAmbiguous = errors.New("store: organization name matches multiple rows")

// OrganizationByName returns the unique organization whose name matches
// the given string case-insensitively.
func (p *Pool) OrganizationByName(ctx context.Context, name string) (*Organization, error) {
	rows, err := p.Query(ctx, `
		SELECT id, name, currency, created_at, updated_at
		FROM organizations
		WHERE LOWER(name) = LOWER($1)
		LIMIT 2
	`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []*Organization
	for rows.Next() {
		o := &Organization{}
		if err := rows.Scan(&o.ID, &o.Name, &o.Currency, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		orgs = append(orgs, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	switch len(orgs) {
	case 0:
		return nil, ErrNotFound
	case 1:
		return orgs[0], nil
	default:
		return nil, ErrOrgAmbiguous
	}
}

// UpdateOrganizationProfile updates the org's name and currency.
func (p *Pool) UpdateOrganizationProfile(ctx context.Context, orgID uuid.UUID, name string, currency *string) (*Organization, error) {
	org := &Organization{}
	err := p.QueryRow(ctx, `
		UPDATE organizations
		SET name = $2, currency = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, name, currency, created_at, updated_at
	`, orgID, name, currency).Scan(&org.ID, &org.Name, &org.Currency, &org.CreatedAt, &org.UpdatedAt)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return org, nil
}
