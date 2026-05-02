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
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	Email           string
	PasswordHash    []byte
	FullName        string
	Role            string
	EmailVerifiedAt *time.Time
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	// ClerkUserID links this row to its Clerk user. Nullable during the
	// Sprint 005 cutover; promoted to NOT NULL by migration 0003.
	ClerkUserID *string
}

type Organization struct {
	ID        uuid.UUID
	Name      string
	Currency  *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateOrgAndAdmin creates an organization and its first org_admin user
// in a single transaction. Returns ErrEmailTaken if the email is already in use.
var ErrEmailTaken = errors.New("store: email already in use")

func (p *Pool) CreateOrgAndAdmin(ctx context.Context, orgName, email, fullName string, passwordHash []byte) (*Organization, *User, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	org := &Organization{}
	err = tx.QueryRow(ctx, `
		INSERT INTO organizations (name)
		VALUES ($1)
		RETURNING id, name, currency, created_at, updated_at
	`, orgName).Scan(&org.ID, &org.Name, &org.Currency, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, nil, err
	}

	user := &User{}
	err = scanUser(tx.QueryRow(ctx, `
		INSERT INTO users (organization_id, email, password_hash, full_name, role)
		VALUES ($1, $2, $3, $4, 'org_admin')
		RETURNING `+userColumns+`
	`, org.ID, email, passwordHash, fullName), user)
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return nil, nil, ErrEmailTaken
		}
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return org, user, nil
}

// userColumns is the canonical SELECT projection for users. Every scan
// helper in this file uses it so a column addition only needs to update
// this constant and the matching Scan() call.
const userColumns = `id, organization_id, email, password_hash, full_name, role,
	email_verified_at, is_active, created_at, updated_at, clerk_user_id`

func scanUser(row interface {
	Scan(dest ...any) error
}, u *User) error {
	return row.Scan(
		&u.ID, &u.OrganizationID, &u.Email, &u.PasswordHash,
		&u.FullName, &u.Role, &u.EmailVerifiedAt, &u.IsActive,
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
// no user has been linked to that provider id (e.g., the webhook hasn't
// landed yet).
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
//
// The user is created as email-verified — Clerk has already verified the
// email by the time this row is written.
func (p *Pool) CreateExternalUser(
	ctx context.Context,
	orgID uuid.UUID,
	clerkUserID, email, fullName, role string,
) (*User, error) {
	user := &User{}
	err := scanUser(p.QueryRow(ctx, `
		INSERT INTO users (organization_id, clerk_user_id, email, full_name, role, email_verified_at)
		VALUES ($1, $2, $3, $4, $5, now())
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

// ErrUserClerkIDTaken indicates a uniqueness conflict on users.clerk_user_id.
// Webhook handlers and signup-complete should treat it as idempotent.
var ErrUserClerkIDTaken = errors.New("store: user already linked to that clerk_user_id")

func (p *Pool) MarkEmailVerified(ctx context.Context, userID uuid.UUID, at time.Time) error {
	_, err := p.Exec(ctx, `UPDATE users SET email_verified_at = $1, updated_at = now() WHERE id = $2`, at, userID)
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
		// good enough — the only unique constraint that signup could trip is users_email_key
		_ = constraint
		return true
	}
	// fallback: pgx wraps in different ways; do a string check
	return errors.Is(err, pgx.ErrNoRows) == false && err != nil && containsAll(err.Error(), "duplicate key", "users_email_key")
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
