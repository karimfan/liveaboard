package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// OrganizationByClerkID looks up an organization by its Clerk org id.
// Returns ErrNotFound if no org has been linked to that provider id.
func (p *Pool) OrganizationByClerkID(ctx context.Context, clerkOrgID string) (*Organization, error) {
	org := &Organization{}
	err := p.QueryRow(ctx, `
		SELECT id, name, currency, created_at, updated_at
		FROM organizations
		WHERE clerk_org_id = $1
	`, clerkOrgID).Scan(&org.ID, &org.Name, &org.Currency, &org.CreatedAt, &org.UpdatedAt)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return org, nil
}

// CreateExternalOrganization inserts an org linked to a provider id. Used by
// the signup-complete flow when a brand-new Clerk user creates the first org.
//
// The caller is expected to also create the first user in the same
// transaction; see CreateExternalOrgAndAdmin below for the combined helper.
func (p *Pool) CreateExternalOrganization(ctx context.Context, name, clerkOrgID string) (*Organization, error) {
	org := &Organization{}
	err := p.QueryRow(ctx, `
		INSERT INTO organizations (name, clerk_org_id)
		VALUES ($1, $2)
		RETURNING id, name, currency, created_at, updated_at
	`, name, clerkOrgID).Scan(&org.ID, &org.Name, &org.Currency, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err, "organizations_clerk_org_id_key") {
			return nil, ErrOrgClerkIDTaken
		}
		return nil, err
	}
	return org, nil
}

// UpdateOrganizationName updates the org name from a provider event.
func (p *Pool) UpdateOrganizationName(ctx context.Context, clerkOrgID, name string, at time.Time) error {
	_, err := p.Exec(ctx, `
		UPDATE organizations
		SET name = $2, updated_at = $3
		WHERE clerk_org_id = $1
	`, clerkOrgID, name, at)
	return err
}

// CreateExternalOrgAndAdmin atomically creates an organization plus its
// first org_admin user, both linked to provider ids. Used by
// /api/signup-complete after a Clerk user has been created and a Clerk
// org has been provisioned.
//
// Returns ErrEmailTaken if a user with the same email already exists.
func (p *Pool) CreateExternalOrgAndAdmin(
	ctx context.Context,
	orgName, clerkOrgID string,
	clerkUserID, email, fullName string,
) (*Organization, *User, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	org := &Organization{}
	if err := tx.QueryRow(ctx, `
		INSERT INTO organizations (name, clerk_org_id)
		VALUES ($1, $2)
		RETURNING id, name, currency, created_at, updated_at
	`, orgName, clerkOrgID).Scan(&org.ID, &org.Name, &org.Currency, &org.CreatedAt, &org.UpdatedAt); err != nil {
		if isUniqueViolation(err, "organizations_clerk_org_id_key") {
			return nil, nil, ErrOrgClerkIDTaken
		}
		return nil, nil, err
	}

	user := &User{}
	if err := scanUser(tx.QueryRow(ctx, `
		INSERT INTO users (organization_id, email, full_name, role, clerk_user_id)
		VALUES ($1, $2, $3, 'org_admin', $4)
		RETURNING `+userColumns+`
	`, org.ID, email, fullName, clerkUserID), user); err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return nil, nil, ErrEmailTaken
		}
		if isUniqueViolation(err, "users_clerk_user_id_key") {
			return nil, nil, ErrUserClerkIDTaken
		}
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return org, user, nil
}

// ErrOrgClerkIDTaken indicates a uniqueness conflict on organizations.clerk_org_id.
// In practice this means the same Clerk org has been linked twice; webhook
// handlers and signup-complete should treat it as idempotent.
var ErrOrgClerkIDTaken = errors.New("store: organization already linked to that clerk_org_id")

// The existing OrganizationByID helper in users.go remains valid (it's
// tenant-agnostic and used by both auth and the dashboard endpoint) and
// will be moved here in a follow-up cleanup.

// ErrOrgAmbiguous indicates that OrganizationByName matched more than
// one row. The caller (typically the scrape-boat CLI) should ask the
// operator to use the org's UUID instead.
var ErrOrgAmbiguous = errors.New("store: organization name matches multiple rows")

// OrganizationByName returns the unique organization whose name matches
// the given string case-insensitively. Returns ErrNotFound if no rows
// match and ErrOrgAmbiguous if more than one matches.
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
// Returns the updated row. Tenant scoping is implicit (the caller
// passes the org id from the authenticated session).
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
