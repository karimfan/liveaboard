package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	RoleOrgAdmin       = "org_admin"
	RoleCruiseDirector = "cruise_director"
)

type User struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	Email           string
	PasswordHash    []byte
	FullName        string
	Phone           *string
	Role            string
	EmailVerifiedAt *time.Time
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Organization struct {
	ID        uuid.UUID
	Name      string
	Currency  *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ErrEmailTaken is returned when a user insert hits the email unique
// constraint. Callers should treat this as a conflict at the API layer
// (or — for non-enumerating signup — silently swallow it).
var ErrEmailTaken = errors.New("store: email already in use")

const userColumns = `id, organization_id, email, password_hash, full_name, phone, role,
	email_verified_at, is_active, created_at, updated_at`

func scanUser(row interface {
	Scan(dest ...any) error
}, u *User) error {
	return row.Scan(
		&u.ID, &u.OrganizationID, &u.Email, &u.PasswordHash,
		&u.FullName, &u.Phone, &u.Role, &u.EmailVerifiedAt, &u.IsActive,
		&u.CreatedAt, &u.UpdatedAt,
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

// CreateOrgAndAdmin creates an organization and its first org_admin user
// in a single transaction. Used by /api/signup.
func (p *Pool) CreateOrgAndAdmin(
	ctx context.Context,
	orgName, email, fullName string,
	passwordHash []byte,
) (*Organization, *User, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	org := &Organization{}
	if err := tx.QueryRow(ctx, `
		INSERT INTO organizations (name)
		VALUES ($1)
		RETURNING id, name, currency, created_at, updated_at
	`, orgName).Scan(&org.ID, &org.Name, &org.Currency, &org.CreatedAt, &org.UpdatedAt); err != nil {
		return nil, nil, err
	}

	user := &User{}
	if err := scanUser(tx.QueryRow(ctx, `
		INSERT INTO users (organization_id, email, password_hash, full_name, role)
		VALUES ($1, $2, $3, $4, 'org_admin')
		RETURNING `+userColumns+`
	`, org.ID, email, passwordHash, fullName), user); err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return nil, nil, ErrEmailTaken
		}
		return nil, nil, err
	}

	if err := seedDefaultCatalogTx(ctx, tx, org.ID); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return org, user, nil
}

// CreateInvitedUser inserts a user that just accepted an invitation.
// They are created as already-verified — clicking the invite link
// proved possession of the email. The admin captured full_name and
// (optionally) phone at invite time, so we seed them here.
func (p *Pool) CreateInvitedUser(
	ctx context.Context,
	orgID uuid.UUID,
	email, fullName, role string,
	phone *string,
	passwordHash []byte,
) (*User, error) {
	user := &User{}
	err := scanUser(p.QueryRow(ctx, `
		INSERT INTO users (organization_id, email, password_hash, full_name, phone, role, email_verified_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		RETURNING `+userColumns+`
	`, orgID, email, passwordHash, fullName, phone, role), user)
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return user, nil
}

// UpdateUserProfile rewrites full_name + phone for the calling user.
// Email and role are NOT touched here (those have their own dedicated
// flows). Returns ErrNotFound if the user id doesn't exist.
func (p *Pool) UpdateUserProfile(ctx context.Context, userID uuid.UUID, fullName string, phone *string) error {
	tag, err := p.Exec(ctx, `
		UPDATE users
		   SET full_name = $2, phone = $3, updated_at = now()
		 WHERE id = $1
	`, userID, fullName, phone)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkEmailVerified flips email_verified_at to the given time for the user.
func (p *Pool) MarkEmailVerified(ctx context.Context, userID uuid.UUID, at time.Time) error {
	_, err := p.Exec(ctx, `
		UPDATE users SET email_verified_at = $2, updated_at = now() WHERE id = $1
	`, userID, at)
	return err
}

// UpdatePasswordHash replaces a user's bcrypt hash.
func (p *Pool) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash []byte) error {
	tag, err := p.Exec(ctx, `
		UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1
	`, userID, passwordHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateUserEmail swaps the user's email + sets email_verified_at = now().
// Used at the end of the change-email confirmation flow.
func (p *Pool) UpdateUserEmail(ctx context.Context, userID uuid.UUID, newEmail string, at time.Time) error {
	tag, err := p.Exec(ctx, `
		UPDATE users SET email = $2, email_verified_at = $3, updated_at = now() WHERE id = $1
	`, userID, newEmail, at)
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return ErrEmailTaken
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeactivateUser flips users.is_active to false. Provider-side
// membership removal is the caller's responsibility.
func (p *Pool) DeactivateUser(ctx context.Context, userID uuid.UUID) error {
	_, err := p.Exec(ctx, `
		UPDATE users SET is_active = false, updated_at = now() WHERE id = $1
	`, userID)
	return err
}

// UsersForOrg returns every user in an org, ordered by full_name.
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
func (p *Pool) CountActiveUsersByRole(ctx context.Context, orgID uuid.UUID, role string) (int, error) {
	var n int
	if err := p.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE organization_id = $1 AND role = $2 AND is_active = true`,
		orgID, role).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
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
