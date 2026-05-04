package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// StubProvider is an in-memory Provider used by tests. It is intentionally
// simple: it does not pretend to be cryptographically real, it just keeps
// track of which token strings are valid and what they map to.
//
// Production refuses to start with a StubProvider in use; the wiring layer
// rejects auth.AuthStubEnabled=true outside of test mode.
//
// Tests build a StubProvider, populate it via the New* helpers, and pass
// it to handlers as the Provider. The stub also implements every method
// of Provider so callers do not need a separate fake.
type StubProvider struct {
	mu sync.Mutex

	now func() time.Time

	users    map[string]*ProviderUser
	orgs     map[string]*ProviderOrganization
	tokens   map[string]*Claims             // token -> claims
	invites  map[string]*ProviderInvitation // invite_id -> invite
	sessions map[string]bool                // active session ids
}

// NewStubProvider returns a fresh stub. Tests usually want to immediately
// call NewUser / NewOrganization / NewSession.
func NewStubProvider() *StubProvider {
	return &StubProvider{
		now:      time.Now,
		users:    map[string]*ProviderUser{},
		orgs:     map[string]*ProviderOrganization{},
		tokens:   map[string]*Claims{},
		invites:  map[string]*ProviderInvitation{},
		sessions: map[string]bool{},
	}
}

// SetClock overrides the stub's clock; used by tests that need
// deterministic timestamps or expired-token paths.
func (s *StubProvider) SetClock(now func() time.Time) { s.now = now }

// OverrideUser replaces the stored ProviderUser entry. Lets tests model
// "the user updated their email/name in Clerk" without recreating the
// session.
func (s *StubProvider) OverrideUser(u *ProviderUser) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *u
	s.users[u.ID] = &cp
}

// NewUser inserts a user into the stub and returns it.
func (s *StubProvider) NewUser(email, fullName string) *ProviderUser {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := "user_" + randID()
	u := &ProviderUser{
		ID:        id,
		Email:     email,
		FullName:  fullName,
		CreatedAt: s.now(),
	}
	s.users[id] = u
	return u
}

// NewOrganization inserts an org into the stub.
func (s *StubProvider) NewOrganization(name string) *ProviderOrganization {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.newOrgLocked(name)
}

func (s *StubProvider) newOrgLocked(name string) *ProviderOrganization {
	id := "org_" + randID()
	o := &ProviderOrganization{
		ID:        id,
		Name:      name,
		CreatedAt: s.now(),
	}
	s.orgs[id] = o
	return o
}

// NewSession creates a (token, session_id) pair and registers it as valid.
// The returned token is what callers send as the Clerk JWT in
// /api/auth/exchange.
func (s *StubProvider) NewSession(userID, orgID string, ttl time.Duration) (token, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token = "stub_" + randID()
	sessionID = "sess_" + randID()
	s.tokens[token] = &Claims{
		UserID:    userID,
		SessionID: sessionID,
		OrgID:     orgID,
		ExpiresAt: s.now().Add(ttl),
	}
	s.sessions[sessionID] = true
	return token, sessionID
}

// ExpireToken marks an already-issued token as expired by rewinding its
// ExpiresAt. Lets tests exercise the ErrInvalidToken path without sleeping.
func (s *StubProvider) ExpireToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.tokens[token]; ok {
		c.ExpiresAt = s.now().Add(-time.Hour)
	}
}

// --- Provider implementation ---

func (s *StubProvider) VerifyJWT(ctx context.Context, token string) (*Claims, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.tokens[token]
	if !ok {
		return nil, ErrInvalidToken
	}
	if !c.ExpiresAt.After(s.now()) {
		return nil, ErrInvalidToken
	}
	if !s.sessions[c.SessionID] {
		return nil, ErrInvalidToken
	}
	cp := *c
	return &cp, nil
}

func (s *StubProvider) FetchUser(ctx context.Context, userID string) (*ProviderUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[userID]
	if !ok {
		return nil, ErrProviderNotFound
	}
	cp := *u
	return &cp, nil
}

func (s *StubProvider) FetchOrganization(ctx context.Context, orgID string) (*ProviderOrganization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orgs[orgID]
	if !ok {
		return nil, ErrProviderNotFound
	}
	cp := *o
	return &cp, nil
}

func (s *StubProvider) CreateOrganization(ctx context.Context, name, creatorUserID string) (*ProviderOrganization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[creatorUserID]; !ok {
		return nil, ErrProviderNotFound
	}
	o := s.newOrgLocked(name)
	cp := *o
	return &cp, nil
}

func (s *StubProvider) InviteToOrganization(ctx context.Context, orgID, email, role string) (*ProviderInvitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.orgs[orgID]; !ok {
		return nil, ErrProviderNotFound
	}
	for _, inv := range s.invites {
		if inv.OrgID == orgID && inv.Email == email && inv.Status == "pending" {
			return nil, ErrProviderConflict
		}
	}
	inv := &ProviderInvitation{
		ID:     "inv_" + randID(),
		OrgID:  orgID,
		Email:  email,
		Role:   role,
		Status: "pending",
	}
	s.invites[inv.ID] = inv
	cp := *inv
	return &cp, nil
}

func (s *StubProvider) ResendInvitation(ctx context.Context, orgID, inviteID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invites[inviteID]
	if !ok || inv.OrgID != orgID {
		return ErrProviderNotFound
	}
	if inv.Status != "pending" {
		return ErrProviderConflict
	}
	return nil
}

func (s *StubProvider) RevokeInvitation(ctx context.Context, orgID, inviteID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invites[inviteID]
	if !ok || inv.OrgID != orgID {
		return ErrProviderNotFound
	}
	inv.Status = "revoked"
	return nil
}

func (s *StubProvider) RemoveMembership(ctx context.Context, orgID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.orgs[orgID]; !ok {
		return ErrProviderNotFound
	}
	if _, ok := s.users[userID]; !ok {
		return ErrProviderNotFound
	}
	// Stub does not model memberships; revoke any active sessions for this user.
	for token, c := range s.tokens {
		if c.UserID == userID {
			delete(s.tokens, token)
			delete(s.sessions, c.SessionID)
		}
	}
	return nil
}

func (s *StubProvider) RevokeSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.sessions[sessionID] {
		return ErrProviderNotFound
	}
	delete(s.sessions, sessionID)
	for token, c := range s.tokens {
		if c.SessionID == sessionID {
			delete(s.tokens, token)
		}
	}
	return nil
}

// Compile-time assertion that StubProvider implements Provider.
var _ Provider = (*StubProvider)(nil)

// randID returns a short random hex id suitable for stub object ids.
// Cryptographic strength is not required — tests just need uniqueness.
func randID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("stub: rand: %w", err))
	}
	return hex.EncodeToString(b)
}
