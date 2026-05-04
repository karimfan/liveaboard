// Package auth implements authentication wiring around an external identity
// provider (currently Clerk). The Provider interface is the seam between
// the rest of the application and the provider; concrete implementations
// live in this package (clerk.go for production, stub.go for tests).
//
// The application keeps `users` and `organizations` as canonical app
// tables. The provider owns credentials, sessions, email verification,
// password reset, and invitation lifecycle.
package auth

import (
	"context"
	"errors"
	"time"
)

// Errors returned by Provider implementations.
var (
	// ErrInvalidToken is returned when a session JWT is missing, malformed,
	// expired, or fails signature verification.
	ErrInvalidToken = errors.New("auth: invalid session token")

	// ErrProviderNotFound is returned when the provider is asked about an
	// id (user, org, invite) it does not recognize.
	ErrProviderNotFound = errors.New("auth: provider resource not found")

	// ErrProviderConflict is returned for operations that conflict with
	// existing provider state (e.g., inviting an email that is already a
	// member).
	ErrProviderConflict = errors.New("auth: provider conflict")
)

// Claims is the verified subset of a session JWT we care about.
//
// Different providers expose different claim shapes; implementations
// translate into this struct. The application never reads provider-native
// claim formats directly.
type Claims struct {
	// UserID is the provider's stable user identifier (e.g., Clerk user_id).
	UserID string

	// SessionID is the provider's session identifier; used for revocation
	// (logout) and for binding an app_sessions row to a provider session.
	SessionID string

	// OrgID is the provider's active-organization id, if the JWT carries
	// one. Optional — most flows resolve org from our canonical users row.
	OrgID string

	// ExpiresAt is the JWT's expiration time. Implementations must reject
	// expired tokens before returning Claims, but callers may still use
	// this for telemetry or last-seen bookkeeping.
	ExpiresAt time.Time
}

// ProviderUser is the provider's view of a user. Treated as a sync source
// for our local users row — never authoritative for app-level role.
type ProviderUser struct {
	ID        string
	Email     string
	FullName  string
	CreatedAt time.Time
}

// ProviderOrganization is the provider's view of an organization.
type ProviderOrganization struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// ProviderInvitation is a pending org invitation.
type ProviderInvitation struct {
	ID     string
	OrgID  string
	Email  string
	Role   string
	Status string // "pending" | "accepted" | "revoked"
}

// Provider is the abstraction over the identity provider. The concrete
// Clerk implementation lives in clerk.go; the in-memory stub used for
// tests lives in stub.go.
type Provider interface {
	// VerifyJWT validates a session JWT and returns the verified Claims,
	// or ErrInvalidToken. Implementations must reject expired and
	// signature-invalid tokens.
	VerifyJWT(ctx context.Context, token string) (*Claims, error)

	// FetchUser returns the provider-side user record by id, or
	// ErrProviderNotFound.
	FetchUser(ctx context.Context, userID string) (*ProviderUser, error)

	// FetchOrganization returns the provider-side org record by id, or
	// ErrProviderNotFound.
	FetchOrganization(ctx context.Context, orgID string) (*ProviderOrganization, error)

	// CreateOrganization creates a new provider org with creatorUserID as
	// its admin. Returns the new ProviderOrganization. Implementations are
	// responsible for assigning the creator the admin role on the new org.
	CreateOrganization(ctx context.Context, name, creatorUserID string) (*ProviderOrganization, error)

	// InviteToOrganization sends an invitation email via the provider.
	// Returns the invitation id, used for resend/revoke. The role string
	// is the provider-side role slug (e.g., "org_admin",
	// "site_director" — matching our app roles).
	InviteToOrganization(ctx context.Context, orgID, email, role string) (*ProviderInvitation, error)

	// ResendInvitation resends the email for an existing invitation.
	ResendInvitation(ctx context.Context, orgID, inviteID string) error

	// RevokeInvitation cancels a pending invitation.
	RevokeInvitation(ctx context.Context, orgID, inviteID string) error

	// RemoveMembership removes a user from a provider org. Used for
	// US-6.2 deactivation. The user record is preserved on the provider
	// side; only the membership goes away.
	RemoveMembership(ctx context.Context, orgID, userID string) error

	// RevokeSession invalidates a single provider session. Used by
	// /api/logout to terminate the current browser session on the
	// provider side as well as locally.
	RevokeSession(ctx context.Context, sessionID string) error
}

// App-level role slugs. These also serve as the canonical provider-role
// strings we ask the provider to mirror, so the mapping is identity.
const (
	RoleOrgAdmin     = "org_admin"
	RoleSiteDirector = "site_director"
)
