package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/karimfan/liveaboard/internal/store"
)

// --- payload shapes ---
//
// These mirror the relevant subset of Clerk webhook payloads. Fields we
// don't use are dropped — this is intentional, the full Clerk schema is
// large and would create maintenance noise. See:
// https://clerk.com/docs/integrations/webhooks/sync-data

type clerkUserData struct {
	ID                    string                  `json:"id"`
	FirstName             *string                 `json:"first_name"`
	LastName              *string                 `json:"last_name"`
	PrimaryEmailAddressID *string                 `json:"primary_email_address_id"`
	EmailAddresses        []clerkUserEmailAddress `json:"email_addresses"`
}

type clerkUserEmailAddress struct {
	ID           string `json:"id"`
	EmailAddress string `json:"email_address"`
}

func (u *clerkUserData) primaryEmail() string {
	if u == nil {
		return ""
	}
	primary := ""
	if u.PrimaryEmailAddressID != nil {
		primary = *u.PrimaryEmailAddressID
	}
	for _, e := range u.EmailAddresses {
		if e.ID == primary || primary == "" {
			return e.EmailAddress
		}
	}
	if len(u.EmailAddresses) > 0 {
		return u.EmailAddresses[0].EmailAddress
	}
	return ""
}

func (u *clerkUserData) fullName() string {
	if u == nil {
		return ""
	}
	first := ""
	last := ""
	if u.FirstName != nil {
		first = strings.TrimSpace(*u.FirstName)
	}
	if u.LastName != nil {
		last = strings.TrimSpace(*u.LastName)
	}
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

type clerkOrganizationData struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type clerkMembershipData struct {
	ID             string                        `json:"id"`
	Role           string                        `json:"role"`
	Organization   clerkOrganizationData         `json:"organization"`
	PublicUserData clerkMembershipPublicUserData `json:"public_user_data"`
}

type clerkMembershipPublicUserData struct {
	UserID         string  `json:"user_id"`
	FirstName      *string `json:"first_name"`
	LastName       *string `json:"last_name"`
	IdentifierType string  `json:"identifier_type"`
	Identifier     string  `json:"identifier"` // primary email when type=email_address
}

type deletedResource struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}

// --- user.* handlers ---

// handleUserCreated is intentionally a no-op. A Clerk user has no org
// context yet; the local users row is created when /api/signup-complete
// runs (org bootstrap) or when organizationMembership.created fires
// (invitation acceptance). Logging here for visibility only.
func (r *WebhookReceiver) handleUserCreated(_ context.Context, _ json.RawMessage) error {
	return nil
}

// handleUserUpdated mirrors identity changes (email, name) from Clerk
// into our users row when the row exists. role and organization_id are
// app-level and never touched by this path.
func (r *WebhookReceiver) handleUserUpdated(ctx context.Context, raw json.RawMessage) error {
	var u clerkUserData
	if err := json.Unmarshal(raw, &u); err != nil {
		return err
	}
	if u.ID == "" {
		return errors.New("user.updated: missing id")
	}
	// UpdateExternalUser is a WHERE clerk_user_id = $1 update; if the
	// local row doesn't exist yet (race with creation) it's a no-op.
	return r.Store.UpdateExternalUser(ctx, u.ID, u.primaryEmail(), u.fullName())
}

// handleUserDeleted deactivates the local users row. We do not hard
// delete because the user may be referenced by future trip/ledger rows
// and we want soft-delete semantics for the user-facing app.
func (r *WebhookReceiver) handleUserDeleted(ctx context.Context, raw json.RawMessage) error {
	var d deletedResource
	if err := json.Unmarshal(raw, &d); err != nil {
		return err
	}
	if d.ID == "" {
		return errors.New("user.deleted: missing id")
	}
	user, err := r.Store.UserByClerkID(ctx, d.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := r.Store.DeactivateUser(ctx, user.ID); err != nil {
		return err
	}
	if err := r.Store.DeleteAppSessionsForUser(ctx, user.ID); err != nil {
		return err
	}
	return nil
}

// --- organization.* handlers ---

// handleOrganizationCreated is idempotent: when a new Clerk org is
// created and we don't already have it locally, log and continue. Local
// rows are created by /api/signup-complete (which is the only place
// orgs come from in this product); Clerk creating an org we don't know
// about is unexpected but harmless.
func (r *WebhookReceiver) handleOrganizationCreated(ctx context.Context, raw json.RawMessage) error {
	var o clerkOrganizationData
	if err := json.Unmarshal(raw, &o); err != nil {
		return err
	}
	if o.ID == "" {
		return errors.New("organization.created: missing id")
	}
	if _, err := r.Store.OrganizationByClerkID(ctx, o.ID); err == nil {
		return nil // already linked, no-op
	}
	r.Log.Warn("clerk webhook: organization.created for an org we don't know locally",
		"clerk_org_id", o.ID, "name", o.Name)
	return nil
}

// handleOrganizationUpdated mirrors a Clerk-side rename into the local
// row. No-op if the org isn't linked locally.
func (r *WebhookReceiver) handleOrganizationUpdated(ctx context.Context, raw json.RawMessage) error {
	var o clerkOrganizationData
	if err := json.Unmarshal(raw, &o); err != nil {
		return err
	}
	if o.ID == "" {
		return errors.New("organization.updated: missing id")
	}
	return r.Store.UpdateOrganizationName(ctx, o.ID, o.Name, r.Now())
}

// handleOrganizationDeleted: org delete from the Clerk side. We do
// nothing destructive locally for now — preserving rows until a
// product-level decision exists. Logged for ops visibility.
func (r *WebhookReceiver) handleOrganizationDeleted(_ context.Context, _ json.RawMessage) error {
	return nil
}

// --- organizationMembership.* handlers ---

// handleMembershipCreated is the invitation-acceptance write path.
// When an invitee accepts a Clerk org invite, this fires. We:
//
//  1. Look up the local organization by clerk_org_id.
//  2. If we already have a local users row for this clerk_user_id,
//     no-op (this is the org-admin-created-their-own-org case where
//     /api/signup-complete already wrote the row).
//  3. Otherwise create a local users row with role mapped from the
//     Clerk membership role.
func (r *WebhookReceiver) handleMembershipCreated(ctx context.Context, raw json.RawMessage) error {
	var m clerkMembershipData
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	if m.Organization.ID == "" || m.PublicUserData.UserID == "" {
		return errors.New("organizationMembership.created: missing identifiers")
	}

	org, err := r.Store.OrganizationByClerkID(ctx, m.Organization.ID)
	if errors.Is(err, store.ErrNotFound) {
		r.Log.Warn("clerk webhook: membership for unknown local org",
			"clerk_org_id", m.Organization.ID, "clerk_user_id", m.PublicUserData.UserID)
		return nil
	}
	if err != nil {
		return err
	}

	if _, err := r.Store.UserByClerkID(ctx, m.PublicUserData.UserID); err == nil {
		// Already linked — nothing to do. This is the signup-complete path.
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}

	role := mapClerkRoleToAppRole(m.Role)
	email := m.PublicUserData.Identifier
	if email == "" {
		// Fall back to the Provider's user-detail endpoint if the
		// payload didn't include the email (older Clerk versions).
		pUser, err := r.Provider.FetchUser(ctx, m.PublicUserData.UserID)
		if err != nil {
			return err
		}
		email = pUser.Email
	}
	fullName := membershipFullName(m.PublicUserData)
	if fullName == "" {
		// Fall back as above.
		pUser, err := r.Provider.FetchUser(ctx, m.PublicUserData.UserID)
		if err == nil {
			fullName = pUser.FullName
		}
	}
	if fullName == "" {
		fullName = email // last-resort placeholder; user can change later
	}

	if _, err := r.Store.CreateExternalUser(ctx, org.ID, m.PublicUserData.UserID, email, fullName, role); err != nil {
		if errors.Is(err, store.ErrUserClerkIDTaken) {
			return nil // race: another path created the row
		}
		return err
	}
	return nil
}

// handleMembershipUpdated mirrors a role change from Clerk. We DO NOT
// flip our app-level role from a webhook by default — users.role is
// authoritative — so this handler currently logs and no-ops. If a real
// product need emerges (admin promotes/demotes via Clerk dashboard),
// uncomment the role update.
func (r *WebhookReceiver) handleMembershipUpdated(_ context.Context, raw json.RawMessage) error {
	var m clerkMembershipData
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	r.Log.Info("clerk webhook: organizationMembership.updated (no-op; users.role is authoritative)",
		"clerk_org_id", m.Organization.ID, "clerk_user_id", m.PublicUserData.UserID, "clerk_role", m.Role)
	return nil
}

// handleMembershipDeleted is the app-level deactivation pathway when the
// removal originates from the Clerk dashboard or another channel
// (sibling to /api/users/{id}/deactivate calling RemoveMembership).
func (r *WebhookReceiver) handleMembershipDeleted(ctx context.Context, raw json.RawMessage) error {
	var m clerkMembershipData
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	if m.PublicUserData.UserID == "" {
		return errors.New("organizationMembership.deleted: missing user id")
	}

	user, err := r.Store.UserByClerkID(ctx, m.PublicUserData.UserID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := r.Store.DeactivateUser(ctx, user.ID); err != nil {
		return err
	}
	if err := r.Store.DeleteAppSessionsForUser(ctx, user.ID); err != nil {
		return err
	}
	return nil
}

// --- helpers ---

// mapClerkRoleToAppRole translates Clerk's organization role slug into
// the canonical app role. Clerk's default org roles are "admin" and
// "basic_member"; we configure custom slugs (org_admin, site_director)
// to make the mapping identity. Anything we don't recognize falls back
// to RoleSiteDirector.
func mapClerkRoleToAppRole(clerkRole string) string {
	switch strings.ToLower(strings.TrimPrefix(clerkRole, "org:")) {
	case RoleOrgAdmin, "admin":
		return RoleOrgAdmin
	case RoleSiteDirector, "director", "basic_member", "member":
		return RoleSiteDirector
	default:
		return RoleSiteDirector
	}
}

func membershipFullName(p clerkMembershipPublicUserData) string {
	first := ""
	last := ""
	if p.FirstName != nil {
		first = strings.TrimSpace(*p.FirstName)
	}
	if p.LastName != nil {
		last = strings.TrimSpace(*p.LastName)
	}
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
