package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	RoleOrgAdmin     = "org_admin"
	RoleSiteDirector = "site_director"
)

type User struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Email          string
	FullName       string
	Role           string
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// ClerkUserID links this row to its Clerk user. Always populated
	// after migration 0004.
	ClerkUserID string
}

type Organization struct {
	ID        uuid.UUID
	Name      string
	Currency  *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ErrEmailTaken is returned when a user insert hits the email unique
// constraint. Callers should treat this as a conflict at the API layer.
var ErrEmailTaken = errors.New("store: email already in use")

// ErrUserClerkIDTaken indicates a uniqueness conflict on users.clerk_user_id.
// Webhook handlers and signup-complete should treat it as idempotent.
var ErrUserClerkIDTaken = errors.New("store: user already linked to that clerk_user_id")

// userColumns is the canonical SELECT projection for users. Every scan
// helper in this file uses it so a column addition only needs to update
// this constant and the matching Scan() call.
const userColumns = `id, organization_id, email, full_name, role,
	is_active, created_at, updated_at, clerk_user_id`

func scanUser(row interface {
	Scan(dest ...any) error
}, u *User) error {
	return row.Scan(
		&u.ID, &u.OrganizationID, &u.Email,
		&u.FullName, &u.Role, &u.IsActive,
		&u.CreatedAt, &u.UpdatedAt, &u.ClerkUserID,
	)
}

func (p *Pool) UserByEmail(ctx context.Context, email string) (*User, error) {
	user := &User{}
	err := scanUser(p.QueryRow(ctx, `
		SELECT `+userColumns+` FROM users WHERE email = $1
	`, email), user)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (p *Pool) UserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	user := &User{}
	err := scanUser(p.QueryRow(ctx, `
		SELECT `+userColumns+` FROM users WHERE id = $1
	`, id), user)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// UserByClerkID looks up a user by Clerk user id. Returns ErrNotFound if
// no user has been linked to that provider id.
func (p *Pool) UserByClerkID(ctx context.Context, clerkUserID string) (*User, error) {
	user := &User{}
	err := scanUser(p.QueryRow(ctx, `
		SELECT `+userColumns+` FROM users WHERE clerk_user_id = $1
	`, clerkUserID), user)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// CreateExternalUser inserts a user that already exists on the provider
// side (i.e., has a clerk_user_id). The role argument must be one of
// RoleOrgAdmin / RoleSiteDirector.
func (p *Pool) CreateExternalUser(
	ctx context.Context,
	orgID uuid.UUID,
	clerkUserID, email, fullName, role string,
) (*User, error) {
	user := &User{}
	err := scanUser(p.QueryRow(ctx, `
		INSERT INTO users (organization_id, clerk_user_id, email, full_name, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+userColumns+`
	`, orgID, clerkUserID, email, fullName, role), user)
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return nil, ErrEmailTaken
		}
		if isUniqueViolation(err, "users_clerk_user_id_key") {
			return nil, ErrUserClerkIDTaken
		}
		return nil, err
	}
	return user, nil
}

// UpdateExternalUser keeps a user row in sync with provider events
// (user.updated webhook). Only mutable identity fields are touched —
// app-level role lives in users.role and is never overwritten by a
// provider event.
func (p *Pool) UpdateExternalUser(
	ctx context.Context,
	clerkUserID, email, fullName string,
) error {
	_, err := p.Exec(ctx, `
		UPDATE users
		SET email = $2, full_name = $3, updated_at = now()
		WHERE clerk_user_id = $1
	`, clerkUserID, email, fullName)
	return err
}

// DeactivateUser flips users.is_active to false. The provider-side
// membership removal is the caller's responsibility.
func (p *Pool) DeactivateUser(ctx context.Context, userID uuid.UUID) error {
	_, err := p.Exec(ctx, `
		UPDATE users
		SET is_active = false, updated_at = now()
		WHERE id = $1
	`, userID)
	return err
}

// OrganizationByID returns an organization by id. Caller is responsible for
// asserting that the requesting user belongs to it (this is a low-level read).
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

func isUniqueViolation(err error, constraint string) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		_ = constraint
		return true
	}
	return errors.Is(err, pgx.ErrNoRows) == false && err != nil && containsAll(err.Error(), "duplicate key", constraint)
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// UsersForOrg returns every user in an org, ordered by full_name.
// Used by the admin /api/admin/users endpoint.
func (p *Pool) UsersForOrg(ctx context.Context, orgID uuid.UUID) ([]*User, error) {
	rows, err := p.Query(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE organization_id = $1
		ORDER BY full_name
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u := &User{}
		if err := scanUser(rows, u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountActiveUsersByRole counts active users in an org with a given role.
// Used by the Overview's setup-completeness card.
func (p *Pool) CountActiveUsersByRole(ctx context.Context, orgID uuid.UUID, role string) (int, error) {
	var n int
	if err := p.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE organization_id = $1 AND role = $2 AND is_active = true`,
		orgID, role).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
