package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/karimfan/liveaboard/internal/email"
	"github.com/karimfan/liveaboard/internal/store"
)

type InviteTripGuestInput struct {
	TripID   uuid.UUID
	ActorID  uuid.UUID
	BerthID  uuid.UUID
	FullName string
	Email    string
}

type GuestInviteLookup struct {
	Token            string
	TripGuestID      uuid.UUID
	Email            string
	FullName         string
	OrganizationName string
	BoatName         string
	Itinerary        string
	StartDate        time.Time
	EndDate          time.Time
	ExpiresAt        time.Time
}

type AcceptGuestInviteResult struct {
	Guest     *store.GuestUser
	TripGuest *store.TripGuest
	Token     string
	ExpiresAt time.Time
}

func (s *Service) InviteTripGuest(ctx context.Context, orgID uuid.UUID, in InviteTripGuestInput) (*store.TripGuest, error) {
	em := normalizeEmail(in.Email)
	if !looksLikeEmail(em) {
		return nil, fmt.Errorf("%w: email", ErrInvalidInput)
	}
	full := strings.TrimSpace(in.FullName)
	if full == "" {
		return nil, fmt.Errorf("%w: full_name", ErrInvalidInput)
	}
	rawToken, tokenHash, err := NewToken()
	if err != nil {
		return nil, err
	}
	g, inv, err := s.Store.CreateTripGuestInvite(ctx, orgID, in.TripID, in.ActorID, full, em, tokenHash, s.now().Add(s.InvitationDuration), in.BerthID)
	if err != nil {
		return nil, err
	}
	if err := s.sendGuestInviteEmail(ctx, orgID, g, inv, rawToken); err != nil {
		_ = s.Store.MarkTripGuestInviteFailed(ctx, g.ID, err.Error())
		s.Log.Error("guest invite: send email", "err", err, "trip_guest_id", g.ID)
		return nil, err
	}
	_ = s.Store.MarkTripGuestInviteSent(ctx, g.ID, s.now())
	return g, nil
}

func (s *Service) ResendTripGuestInvite(ctx context.Context, orgID, tripID, guestID uuid.UUID) (*store.TripGuest, error) {
	rawToken, tokenHash, err := NewToken()
	if err != nil {
		return nil, err
	}
	g, inv, err := s.Store.ResendTripGuestInvite(ctx, orgID, tripID, guestID, tokenHash, s.now().Add(s.InvitationDuration))
	if err != nil {
		return nil, err
	}
	if err := s.sendGuestInviteEmail(ctx, orgID, g, inv, rawToken); err != nil {
		_ = s.Store.MarkTripGuestInviteFailed(ctx, g.ID, err.Error())
		s.Log.Error("guest invite resend: send email", "err", err, "trip_guest_id", g.ID)
		return nil, err
	}
	_ = s.Store.MarkTripGuestInviteSent(ctx, g.ID, s.now())
	return g, nil
}

func (s *Service) LookupGuestInvite(ctx context.Context, rawToken string) (*GuestInviteLookup, error) {
	view, err := s.lookupGuestInviteView(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	return &GuestInviteLookup{
		Token:            rawToken,
		TripGuestID:      view.Guest.ID,
		Email:            view.Guest.Email,
		FullName:         view.Guest.FullName,
		OrganizationName: view.OrganizationName,
		BoatName:         view.BoatName,
		Itinerary:        view.TripItinerary,
		StartDate:        view.TripStartDate,
		EndDate:          view.TripEndDate,
		ExpiresAt:        view.Invitation.ExpiresAt,
	}, nil
}

func (s *Service) AcceptGuestInvite(ctx context.Context, rawToken, password string) (*AcceptGuestInviteResult, error) {
	view, err := s.lookupGuestInviteView(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	email := normalizeEmail(view.Guest.Email)
	if err := s.Throttle.Check(ctx, email); err != nil {
		return nil, err
	}
	if strings.TrimSpace(password) == "" {
		_ = s.Throttle.RecordFailure(ctx, email)
		return nil, ErrInvalidCredentials
	}

	guest, err := s.Store.GuestUserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		if err := ValidatePassword(password); err != nil {
			_ = s.Throttle.RecordFailure(ctx, email)
			return nil, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), s.BcryptCost)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		guest, err = s.Store.CreateGuestUser(ctx, email, hash, s.now())
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		if err := bcrypt.CompareHashAndPassword(guest.PasswordHash, []byte(password)); err != nil {
			_ = s.Throttle.RecordFailure(ctx, email)
			return nil, ErrInvalidCredentials
		}
	}
	if !guest.IsActive {
		_ = s.Throttle.RecordFailure(ctx, email)
		return nil, ErrInvalidCredentials
	}

	tripGuest, err := s.Store.AcceptGuestInvite(ctx, view.Invitation.ID, view.Guest.ID, guest.ID, s.now())
	if err != nil {
		return nil, err
	}
	rawCookie, sess, err := MintGuestSession(ctx, s.Store, guest, s.now(), s.GuestSessionDuration)
	if err != nil {
		return nil, err
	}
	_ = s.Throttle.Clear(ctx, email)
	return &AcceptGuestInviteResult{Guest: guest, TripGuest: tripGuest, Token: rawCookie, ExpiresAt: sess.ExpiresAt}, nil
}

func (s *Service) LogoutGuest(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.Store.RevokeGuestSessionByTokenHash(ctx, HashToken(rawToken), s.now())
}

func (s *Service) lookupGuestInviteView(ctx context.Context, rawToken string) (*store.GuestInviteView, error) {
	if rawToken == "" {
		return nil, ErrTokenInvalid
	}
	view, err := s.Store.GuestInviteByTokenHash(ctx, HashToken(rawToken))
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrTokenInvalid
	}
	if err != nil {
		return nil, err
	}
	if view.Invitation.AcceptedAt != nil || view.Invitation.RevokedAt != nil || view.Guest.RevokedAt != nil || view.Invitation.ExpiresAt.Before(s.now()) {
		return nil, ErrTokenInvalid
	}
	return view, nil
}

func (s *Service) sendGuestInviteEmail(ctx context.Context, orgID uuid.UUID, g *store.TripGuest, inv *store.GuestTripInvitation, rawToken string) error {
	view, err := s.Store.GuestInviteByTokenHash(ctx, HashToken(rawToken))
	if err != nil {
		// Token hash was just inserted. Fall back to direct context if
		// lookup fails unexpectedly so the caller gets the real error.
		return err
	}
	link := fmt.Sprintf("%s/guest/invitations/%s", strings.TrimRight(s.AppBaseURL, "/"), rawToken)
	msg, err := email.Render(email.KindGuestRegistrationInvite, email.Vars{
		AppName:          "Liveaboard",
		OrganizationName: view.OrganizationName,
		RecipientName:    g.FullName,
		RecipientEmail:   g.Email,
		ActionURL:        link,
		ExpiresAt:        inv.ExpiresAt,
		TripBoatName:     view.BoatName,
		TripItinerary:    view.TripItinerary,
		TripStartDate:    view.TripStartDate,
		TripEndDate:      view.TripEndDate,
	})
	if err != nil {
		return err
	}
	msg.From = s.SenderFrom
	msg.To = g.Email
	_ = orgID
	return s.Email.Send(ctx, msg)
}
