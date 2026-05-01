package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
)

func TestStubVerifyJWTHappyPath(t *testing.T) {
	s := auth.NewStubProvider()
	user := s.NewUser("owner@acme.test", "Acme Owner")
	org := s.NewOrganization("Acme Diving")
	token, sessionID := s.NewSession(user.ID, org.ID, time.Hour)

	c, err := s.VerifyJWT(context.Background(), token)
	if err != nil {
		t.Fatalf("VerifyJWT: %v", err)
	}
	if c.UserID != user.ID {
		t.Errorf("UserID: got %q want %q", c.UserID, user.ID)
	}
	if c.SessionID != sessionID {
		t.Errorf("SessionID: got %q want %q", c.SessionID, sessionID)
	}
	if c.OrgID != org.ID {
		t.Errorf("OrgID: got %q want %q", c.OrgID, org.ID)
	}
	if !c.ExpiresAt.After(time.Now()) {
		t.Errorf("ExpiresAt %v is not in the future", c.ExpiresAt)
	}
}

func TestStubVerifyJWTRejectsUnknownToken(t *testing.T) {
	s := auth.NewStubProvider()
	_, err := s.VerifyJWT(context.Background(), "bogus")
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("err: %v want ErrInvalidToken", err)
	}
}

func TestStubVerifyJWTRejectsExpiredToken(t *testing.T) {
	s := auth.NewStubProvider()
	user := s.NewUser("a@b.test", "A B")
	org := s.NewOrganization("Org")
	token, _ := s.NewSession(user.ID, org.ID, time.Hour)
	s.ExpireToken(token)

	_, err := s.VerifyJWT(context.Background(), token)
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("err: %v want ErrInvalidToken", err)
	}
}

func TestStubVerifyJWTRejectsRevokedSession(t *testing.T) {
	s := auth.NewStubProvider()
	user := s.NewUser("a@b.test", "A B")
	org := s.NewOrganization("Org")
	token, sessionID := s.NewSession(user.ID, org.ID, time.Hour)

	if err := s.RevokeSession(context.Background(), sessionID); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	_, err := s.VerifyJWT(context.Background(), token)
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("err: %v want ErrInvalidToken", err)
	}
}

func TestStubFetchUserAndOrganization(t *testing.T) {
	s := auth.NewStubProvider()
	u := s.NewUser("e@x.test", "E X")
	o := s.NewOrganization("Org")

	got, err := s.FetchUser(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("FetchUser: %v", err)
	}
	if got.Email != "e@x.test" {
		t.Errorf("got Email %q", got.Email)
	}

	gotOrg, err := s.FetchOrganization(context.Background(), o.ID)
	if err != nil {
		t.Fatalf("FetchOrganization: %v", err)
	}
	if gotOrg.Name != "Org" {
		t.Errorf("got Name %q", gotOrg.Name)
	}

	if _, err := s.FetchUser(context.Background(), "user_missing"); !errors.Is(err, auth.ErrProviderNotFound) {
		t.Errorf("FetchUser missing: %v want ErrProviderNotFound", err)
	}
	if _, err := s.FetchOrganization(context.Background(), "org_missing"); !errors.Is(err, auth.ErrProviderNotFound) {
		t.Errorf("FetchOrganization missing: %v want ErrProviderNotFound", err)
	}
}

func TestStubCreateOrganizationRequiresKnownCreator(t *testing.T) {
	s := auth.NewStubProvider()
	if _, err := s.CreateOrganization(context.Background(), "Org", "user_unknown"); !errors.Is(err, auth.ErrProviderNotFound) {
		t.Fatalf("err: %v want ErrProviderNotFound", err)
	}
	u := s.NewUser("a@b.test", "A B")
	got, err := s.CreateOrganization(context.Background(), "Acme", u.ID)
	if err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}
	if got.Name != "Acme" {
		t.Errorf("Name: got %q", got.Name)
	}
}

func TestStubInvitationLifecycle(t *testing.T) {
	s := auth.NewStubProvider()
	o := s.NewOrganization("Org")

	inv, err := s.InviteToOrganization(context.Background(), o.ID, "site@x.test", auth.RoleSiteDirector)
	if err != nil {
		t.Fatalf("InviteToOrganization: %v", err)
	}
	if inv.Status != "pending" || inv.Role != auth.RoleSiteDirector || inv.Email != "site@x.test" {
		t.Errorf("invite: %+v", inv)
	}

	// Duplicate pending invite for same email is a conflict.
	if _, err := s.InviteToOrganization(context.Background(), o.ID, "site@x.test", auth.RoleSiteDirector); !errors.Is(err, auth.ErrProviderConflict) {
		t.Errorf("duplicate invite: %v want ErrProviderConflict", err)
	}

	if err := s.ResendInvitation(context.Background(), o.ID, inv.ID); err != nil {
		t.Errorf("ResendInvitation: %v", err)
	}
	if err := s.RevokeInvitation(context.Background(), o.ID, inv.ID); err != nil {
		t.Errorf("RevokeInvitation: %v", err)
	}
	// Resend after revoke should conflict.
	if err := s.ResendInvitation(context.Background(), o.ID, inv.ID); !errors.Is(err, auth.ErrProviderConflict) {
		t.Errorf("resend after revoke: %v want ErrProviderConflict", err)
	}

	if err := s.ResendInvitation(context.Background(), "org_missing", inv.ID); !errors.Is(err, auth.ErrProviderNotFound) {
		t.Errorf("resend wrong org: %v want ErrProviderNotFound", err)
	}
	if err := s.RevokeInvitation(context.Background(), o.ID, "inv_missing"); !errors.Is(err, auth.ErrProviderNotFound) {
		t.Errorf("revoke missing: %v want ErrProviderNotFound", err)
	}
}

func TestStubRemoveMembershipRevokesUserSessions(t *testing.T) {
	s := auth.NewStubProvider()
	u := s.NewUser("u@x.test", "U X")
	o := s.NewOrganization("Org")
	token, _ := s.NewSession(u.ID, o.ID, time.Hour)

	if err := s.RemoveMembership(context.Background(), o.ID, u.ID); err != nil {
		t.Fatalf("RemoveMembership: %v", err)
	}
	if _, err := s.VerifyJWT(context.Background(), token); !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("token survived membership removal: %v", err)
	}
}

func TestStubRevokeSessionUnknownIsNotFound(t *testing.T) {
	s := auth.NewStubProvider()
	if err := s.RevokeSession(context.Background(), "sess_missing"); !errors.Is(err, auth.ErrProviderNotFound) {
		t.Fatalf("err: %v want ErrProviderNotFound", err)
	}
}
