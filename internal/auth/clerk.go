package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	clerksdk "github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/organization"
	"github.com/clerk/clerk-sdk-go/v2/organizationinvitation"
	"github.com/clerk/clerk-sdk-go/v2/organizationmembership"
	"github.com/clerk/clerk-sdk-go/v2/session"
	"github.com/clerk/clerk-sdk-go/v2/user"
)

// ClerkProvider implements Provider against Clerk's hosted identity
// service via clerk-sdk-go/v2.
//
// Construction validates the secret key is present; production wiring
// must call NewClerkProvider before binding a listener. JWT verification
// uses the SDK's built-in JWKS client, which fetches and caches the
// signing key set transparently.
type ClerkProvider struct {
	// Issuer is the JWT issuer pinned during verification, e.g.
	// "https://brave-giraffe-1.clerk.accounts.dev". When empty the SDK
	// will accept the token's claimed issuer; we set it explicitly so a
	// stolen token from a different Clerk instance cannot authenticate
	// against our app.
	Issuer string
}

// NewClerkProvider configures the SDK with the given secret key and
// returns a Provider. issuer is optional but recommended; if empty,
// clerk_test_user JWTs from any instance will verify.
func NewClerkProvider(secretKey, issuer string) (*ClerkProvider, error) {
	if strings.TrimSpace(secretKey) == "" {
		return nil, errors.New("auth: clerk secret key is empty")
	}
	clerksdk.SetKey(secretKey)
	return &ClerkProvider{Issuer: issuer}, nil
}

// VerifyJWT verifies a Clerk session JWT using the SDK's JWKS client and
// returns the subset of claims we care about.
func (c *ClerkProvider) VerifyJWT(ctx context.Context, token string) (*Claims, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrInvalidToken
	}
	sc, err := jwt.Verify(ctx, &jwt.VerifyParams{Token: token})
	if err != nil {
		return nil, ErrInvalidToken
	}
	if c.Issuer != "" && sc.Issuer != c.Issuer {
		return nil, ErrInvalidToken
	}
	exp := time.Time{}
	if sc.Expiry != nil {
		exp = time.Unix(*sc.Expiry, 0)
	}
	return &Claims{
		UserID:    sc.Subject,
		SessionID: sc.SessionID,
		OrgID:     sc.ActiveOrganizationID,
		ExpiresAt: exp,
	}, nil
}

func (c *ClerkProvider) FetchUser(ctx context.Context, userID string) (*ProviderUser, error) {
	u, err := user.Get(ctx, userID)
	if err != nil {
		return nil, mapClerkErr(err)
	}
	email := primaryEmail(u)
	return &ProviderUser{
		ID:        u.ID,
		Email:     email,
		FullName:  fullNameOf(u),
		CreatedAt: time.UnixMilli(u.CreatedAt),
	}, nil
}

func (c *ClerkProvider) FetchOrganization(ctx context.Context, orgID string) (*ProviderOrganization, error) {
	o, err := organization.Get(ctx, orgID)
	if err != nil {
		return nil, mapClerkErr(err)
	}
	return &ProviderOrganization{
		ID:        o.ID,
		Name:      o.Name,
		CreatedAt: time.UnixMilli(o.CreatedAt),
	}, nil
}

func (c *ClerkProvider) CreateOrganization(ctx context.Context, name, creatorUserID string) (*ProviderOrganization, error) {
	o, err := organization.Create(ctx, &organization.CreateParams{
		Name:      &name,
		CreatedBy: &creatorUserID,
	})
	if err != nil {
		return nil, mapClerkErr(err)
	}
	return &ProviderOrganization{
		ID:        o.ID,
		Name:      o.Name,
		CreatedAt: time.UnixMilli(o.CreatedAt),
	}, nil
}

func (c *ClerkProvider) InviteToOrganization(ctx context.Context, orgID, email, role string) (*ProviderInvitation, error) {
	inv, err := organizationinvitation.Create(ctx, &organizationinvitation.CreateParams{
		OrganizationID: orgID,
		EmailAddress:   &email,
		Role:           &role,
	})
	if err != nil {
		return nil, mapClerkErr(err)
	}
	return &ProviderInvitation{
		ID:     inv.ID,
		OrgID:  orgID,
		Email:  inv.EmailAddress,
		Role:   inv.Role,
		Status: inv.Status,
	}, nil
}

// ResendInvitation resends an invitation by revoking the old one and
// creating a fresh invite with the same email and role. The visible
// invite id changes — the listing UI should always render the latest
// pending invite per email rather than tracking a stable id.
func (c *ClerkProvider) ResendInvitation(ctx context.Context, orgID, inviteID string) error {
	inv, err := organizationinvitation.Get(ctx, &organizationinvitation.GetParams{
		OrganizationID: orgID,
		ID:             inviteID,
	})
	if err != nil {
		return mapClerkErr(err)
	}
	if !strings.EqualFold(inv.Status, "pending") {
		return ErrProviderConflict
	}
	if _, err := organizationinvitation.Revoke(ctx, &organizationinvitation.RevokeParams{
		OrganizationID: orgID,
		ID:             inviteID,
	}); err != nil {
		return mapClerkErr(err)
	}
	email := inv.EmailAddress
	role := inv.Role
	if _, err := organizationinvitation.Create(ctx, &organizationinvitation.CreateParams{
		OrganizationID: orgID,
		EmailAddress:   &email,
		Role:           &role,
	}); err != nil {
		return fmt.Errorf("resend recreate: %w", err)
	}
	return nil
}

func (c *ClerkProvider) RevokeInvitation(ctx context.Context, orgID, inviteID string) error {
	if _, err := organizationinvitation.Revoke(ctx, &organizationinvitation.RevokeParams{
		OrganizationID: orgID,
		ID:             inviteID,
	}); err != nil {
		return mapClerkErr(err)
	}
	return nil
}

func (c *ClerkProvider) RemoveMembership(ctx context.Context, orgID, userID string) error {
	if _, err := organizationmembership.Delete(ctx, &organizationmembership.DeleteParams{
		OrganizationID: orgID,
		UserID:         userID,
	}); err != nil {
		return mapClerkErr(err)
	}
	return nil
}

func (c *ClerkProvider) RevokeSession(ctx context.Context, sessionID string) error {
	if _, err := session.Revoke(ctx, &session.RevokeParams{ID: sessionID}); err != nil {
		return mapClerkErr(err)
	}
	return nil
}

// Compile-time assertion.
var _ Provider = (*ClerkProvider)(nil)

// --- helpers ---

// mapClerkErr translates Clerk SDK errors into auth-package errors. The
// SDK exposes APIErrorResponse with HTTP-status semantics; we only
// distinguish 404 (not found) from everything else.
func mapClerkErr(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *clerksdk.APIErrorResponse
	if errors.As(err, &apiErr) {
		switch apiErr.HTTPStatusCode {
		case 404:
			return ErrProviderNotFound
		case 409, 422:
			return ErrProviderConflict
		}
	}
	return err
}

func primaryEmail(u *clerksdk.User) string {
	if u == nil {
		return ""
	}
	primaryID := derefString(u.PrimaryEmailAddressID)
	for _, e := range u.EmailAddresses {
		if e == nil {
			continue
		}
		if e.ID == primaryID || primaryID == "" {
			return e.EmailAddress
		}
	}
	if len(u.EmailAddresses) > 0 && u.EmailAddresses[0] != nil {
		return u.EmailAddresses[0].EmailAddress
	}
	return ""
}

func fullNameOf(u *clerksdk.User) string {
	if u == nil {
		return ""
	}
	first := strings.TrimSpace(derefString(u.FirstName))
	last := strings.TrimSpace(derefString(u.LastName))
	switch {
	case first == "" && last == "":
		return ""
	case first == "":
		return last
	case last == "":
		return first
	default:
		return first + " " + last
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
