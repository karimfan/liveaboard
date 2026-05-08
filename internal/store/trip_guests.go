package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrTripGuestExists = errors.New("store: guest already exists on trip")

type TripGuest struct {
	ID                      uuid.UUID
	OrganizationID          uuid.UUID
	TripID                  uuid.UUID
	GuestUserID             *uuid.UUID
	InvitedByUserID         *uuid.UUID
	FullName                string
	Email                   string
	InviteSendStatus        string
	InviteLastSentAt        *time.Time
	InviteLastError         *string
	AccountCreatedAt        *time.Time
	RegistrationSubmittedAt *time.Time
	RevokedAt               *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type GuestTripInvitation struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	TripID         uuid.UUID
	TripGuestID    uuid.UUID
	Email          string
	ExpiresAt      time.Time
	AcceptedAt     *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type TripGuestManifestRow struct {
	Guest                 *TripGuest
	Status                string
	InviteExpiresAt       *time.Time
	RegistrationStatus    *string
	RegistrationSubmitted *time.Time
	CabinAssignment       *TripCabinAssignment
}

type TripManifestSummary struct {
	TripID         uuid.UUID
	GuestCount     int
	SubmittedCount int
	ExpectedCount  *int
	HasWarning     bool
}

type GuestInviteView struct {
	Invitation       *GuestTripInvitation
	Guest            *TripGuest
	OrganizationName string
	BoatName         string
	TripItinerary    string
	TripStartDate    time.Time
	TripEndDate      time.Time
}

const tripGuestColumns = `id, organization_id, trip_id, guest_user_id, invited_by_user_id, full_name, email,
	invite_send_status, invite_last_sent_at, invite_last_error, account_created_at, registration_submitted_at,
	revoked_at, created_at, updated_at`

const invitationColumnsGuest = `id, organization_id, trip_id, trip_guest_id, email, expires_at, accepted_at, revoked_at, created_at, updated_at`

func scanTripGuest(row interface{ Scan(dest ...any) error }, g *TripGuest) error {
	return row.Scan(
		&g.ID, &g.OrganizationID, &g.TripID, &g.GuestUserID, &g.InvitedByUserID, &g.FullName, &g.Email,
		&g.InviteSendStatus, &g.InviteLastSentAt, &g.InviteLastError, &g.AccountCreatedAt, &g.RegistrationSubmittedAt,
		&g.RevokedAt, &g.CreatedAt, &g.UpdatedAt,
	)
}

func scanGuestInvitation(row interface{ Scan(dest ...any) error }, inv *GuestTripInvitation) error {
	return row.Scan(&inv.ID, &inv.OrganizationID, &inv.TripID, &inv.TripGuestID, &inv.Email, &inv.ExpiresAt, &inv.AcceptedAt, &inv.RevokedAt, &inv.CreatedAt, &inv.UpdatedAt)
}

func (p *Pool) CreateTripGuestInvite(ctx context.Context, orgID, tripID, invitedBy uuid.UUID, fullName, email string, tokenHash []byte, expiresAt time.Time, berthID uuid.UUID) (*TripGuest, *GuestTripInvitation, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM trips WHERE organization_id = $1 AND id = $2)`, orgID, tripID).Scan(&ok); err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, ErrNotFound
	}

	g := &TripGuest{}
	err = scanTripGuest(tx.QueryRow(ctx, `
		INSERT INTO trip_guests (organization_id, trip_id, invited_by_user_id, full_name, email)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (organization_id, trip_id, email) DO UPDATE SET
			full_name = EXCLUDED.full_name,
			invited_by_user_id = EXCLUDED.invited_by_user_id,
			invite_send_status = 'not_sent',
			invite_last_sent_at = NULL,
			invite_last_error = NULL,
			revoked_at = NULL,
			updated_at = now()
		WHERE trip_guests.revoked_at IS NOT NULL
		RETURNING `+tripGuestColumns,
		orgID, tripID, invitedBy, fullName, email,
	), g)
	if isNoRows(err) {
		return nil, nil, ErrTripGuestExists
	}
	if err != nil {
		return nil, nil, err
	}

	inv := &GuestTripInvitation{}
	err = scanGuestInvitation(tx.QueryRow(ctx, `
		INSERT INTO guest_trip_invitations (organization_id, trip_id, trip_guest_id, email, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+invitationColumnsGuest,
		orgID, tripID, g.ID, email, tokenHash, expiresAt,
	), inv)
	if err != nil {
		return nil, nil, err
	}
	if berthID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: berth_id", ErrInvalidInput)
	}
	if _, err := assignTripGuestBerthTx(ctx, tx, orgID, tripID, g.ID, berthID, invitedBy, nil); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return g, inv, nil
}

func (p *Pool) ResendTripGuestInvite(ctx context.Context, orgID, tripID, guestID uuid.UUID, tokenHash []byte, expiresAt time.Time) (*TripGuest, *GuestTripInvitation, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	g := &TripGuest{}
	err = scanTripGuest(tx.QueryRow(ctx, `
		UPDATE trip_guests
		SET invite_send_status = 'not_sent', invite_last_sent_at = NULL, invite_last_error = NULL, revoked_at = NULL, updated_at = now()
		WHERE organization_id = $1 AND trip_id = $2 AND id = $3
		RETURNING `+tripGuestColumns,
		orgID, tripID, guestID,
	), g)
	if isNoRows(err) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE guest_trip_invitations
		SET revoked_at = now(), updated_at = now()
		WHERE trip_guest_id = $1 AND accepted_at IS NULL AND revoked_at IS NULL
	`, guestID); err != nil {
		return nil, nil, err
	}
	inv := &GuestTripInvitation{}
	err = scanGuestInvitation(tx.QueryRow(ctx, `
		INSERT INTO guest_trip_invitations (organization_id, trip_id, trip_guest_id, email, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+invitationColumnsGuest,
		orgID, tripID, guestID, g.Email, tokenHash, expiresAt,
	), inv)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return g, inv, nil
}

func (p *Pool) RevokeTripGuestInvite(ctx context.Context, orgID, tripID, guestID uuid.UUID, at time.Time) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE trip_guests
		SET revoked_at = $4, updated_at = now()
		WHERE organization_id = $1 AND trip_id = $2 AND id = $3
	`, orgID, tripID, guestID, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `
		UPDATE guest_trip_invitations
		SET revoked_at = $2, updated_at = now()
		WHERE trip_guest_id = $1 AND accepted_at IS NULL AND revoked_at IS NULL
	`, guestID, at); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE trip_cabin_assignments
		SET unassigned_at = $4, unassigned_by_user_id = NULL, updated_at = now()
		WHERE organization_id = $1 AND trip_id = $2 AND trip_guest_id = $3 AND unassigned_at IS NULL
	`, orgID, tripID, guestID, at); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Pool) MarkTripGuestInviteSent(ctx context.Context, guestID uuid.UUID, at time.Time) error {
	_, err := p.Exec(ctx, `
		UPDATE trip_guests SET invite_send_status = 'sent', invite_last_sent_at = $2, invite_last_error = NULL, updated_at = now()
		WHERE id = $1
	`, guestID, at)
	return err
}

func (p *Pool) MarkTripGuestInviteFailed(ctx context.Context, guestID uuid.UUID, message string) error {
	_, err := p.Exec(ctx, `
		UPDATE trip_guests SET invite_send_status = 'failed', invite_last_error = $2, updated_at = now()
		WHERE id = $1
	`, guestID, message)
	return err
}

func (p *Pool) GuestInviteByTokenHash(ctx context.Context, tokenHash []byte) (*GuestInviteView, error) {
	view := &GuestInviteView{Invitation: &GuestTripInvitation{}, Guest: &TripGuest{}}
	err := p.QueryRow(ctx, `
		SELECT
			i.id, i.organization_id, i.trip_id, i.trip_guest_id, i.email, i.expires_at, i.accepted_at, i.revoked_at, i.created_at, i.updated_at,
			g.id, g.organization_id, g.trip_id, g.guest_user_id, g.invited_by_user_id, g.full_name, g.email,
			g.invite_send_status, g.invite_last_sent_at, g.invite_last_error, g.account_created_at, g.registration_submitted_at,
			g.revoked_at, g.created_at, g.updated_at,
			o.name, b.display_name, t.itinerary, t.start_date, t.end_date
		FROM guest_trip_invitations i
		JOIN trip_guests g ON g.id = i.trip_guest_id
		JOIN organizations o ON o.id = i.organization_id
		JOIN trips t ON t.id = i.trip_id AND t.organization_id = i.organization_id
		JOIN boats b ON b.id = t.boat_id AND b.organization_id = i.organization_id
		WHERE i.token_hash = $1
	`, tokenHash).Scan(
		&view.Invitation.ID, &view.Invitation.OrganizationID, &view.Invitation.TripID, &view.Invitation.TripGuestID, &view.Invitation.Email, &view.Invitation.ExpiresAt, &view.Invitation.AcceptedAt, &view.Invitation.RevokedAt, &view.Invitation.CreatedAt, &view.Invitation.UpdatedAt,
		&view.Guest.ID, &view.Guest.OrganizationID, &view.Guest.TripID, &view.Guest.GuestUserID, &view.Guest.InvitedByUserID, &view.Guest.FullName, &view.Guest.Email,
		&view.Guest.InviteSendStatus, &view.Guest.InviteLastSentAt, &view.Guest.InviteLastError, &view.Guest.AccountCreatedAt, &view.Guest.RegistrationSubmittedAt,
		&view.Guest.RevokedAt, &view.Guest.CreatedAt, &view.Guest.UpdatedAt,
		&view.OrganizationName, &view.BoatName, &view.TripItinerary, &view.TripStartDate, &view.TripEndDate,
	)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return view, nil
}

func (p *Pool) AcceptGuestInvite(ctx context.Context, inviteID, tripGuestID, guestUserID uuid.UUID, at time.Time) (*TripGuest, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE guest_trip_invitations
		SET accepted_at = $2, updated_at = now()
		WHERE id = $1 AND accepted_at IS NULL AND revoked_at IS NULL
	`, inviteID, at)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	g := &TripGuest{}
	err = scanTripGuest(tx.QueryRow(ctx, `
		UPDATE trip_guests
		SET guest_user_id = $2, account_created_at = COALESCE(account_created_at, $3), updated_at = now()
		WHERE id = $1 AND revoked_at IS NULL
		RETURNING `+tripGuestColumns,
		tripGuestID, guestUserID, at,
	), g)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return g, nil
}

func (p *Pool) TripManifest(ctx context.Context, orgID, tripID uuid.UUID, now time.Time) ([]*TripGuestManifestRow, error) {
	rows, err := p.Query(ctx, `
		SELECT
			g.id, g.organization_id, g.trip_id, g.guest_user_id, g.invited_by_user_id, g.full_name, g.email,
			g.invite_send_status, g.invite_last_sent_at, g.invite_last_error, g.account_created_at, g.registration_submitted_at,
			g.revoked_at, g.created_at, g.updated_at,
			i.expires_at,
			r.status,
			r.submitted_at,
			a.id, a.boat_id, a.berth_id, a.cabin_label_snapshot, a.berth_label_snapshot,
			a.display_label_snapshot, a.assigned_by_user_id, a.assigned_at, a.unassigned_by_user_id, a.unassigned_at, a.notes
		FROM trip_guests g
		LEFT JOIN LATERAL (
			SELECT expires_at, accepted_at, revoked_at
			FROM guest_trip_invitations
			WHERE trip_guest_id = g.id
			ORDER BY created_at DESC
			LIMIT 1
		) i ON true
		LEFT JOIN guest_trip_registrations r ON r.trip_guest_id = g.id
		LEFT JOIN trip_cabin_assignments a ON a.trip_guest_id = g.id AND a.unassigned_at IS NULL
		WHERE g.organization_id = $1 AND g.trip_id = $2
		ORDER BY lower(g.full_name), g.created_at
	`, orgID, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*TripGuestManifestRow{}
	for rows.Next() {
		g := &TripGuest{}
		row := &TripGuestManifestRow{Guest: g}
		var assignmentID, assignmentBoatID, berthID *uuid.UUID
		var cabinLabel, berthLabel, displayLabel *string
		var assignedBy, unassignedBy *uuid.UUID
		var assignedAt, unassignedAt *time.Time
		var notes *string
		if err := rows.Scan(
			&g.ID, &g.OrganizationID, &g.TripID, &g.GuestUserID, &g.InvitedByUserID, &g.FullName, &g.Email,
			&g.InviteSendStatus, &g.InviteLastSentAt, &g.InviteLastError, &g.AccountCreatedAt, &g.RegistrationSubmittedAt,
			&g.RevokedAt, &g.CreatedAt, &g.UpdatedAt,
			&row.InviteExpiresAt, &row.RegistrationStatus, &row.RegistrationSubmitted,
			&assignmentID, &assignmentBoatID, &berthID, &cabinLabel, &berthLabel, &displayLabel, &assignedBy, &assignedAt, &unassignedBy, &unassignedAt, &notes,
		); err != nil {
			return nil, err
		}
		if assignmentID != nil && assignmentBoatID != nil && berthID != nil && cabinLabel != nil && berthLabel != nil && displayLabel != nil && assignedAt != nil {
			row.CabinAssignment = &TripCabinAssignment{
				ID:                   *assignmentID,
				TripID:               tripID,
				TripGuestID:          g.ID,
				BoatID:               *assignmentBoatID,
				BerthID:              *berthID,
				CabinLabelSnapshot:   *cabinLabel,
				BerthLabelSnapshot:   *berthLabel,
				DisplayLabelSnapshot: *displayLabel,
				AssignedByUserID:     assignedBy,
				AssignedAt:           *assignedAt,
				UnassignedByUserID:   unassignedBy,
				UnassignedAt:         unassignedAt,
				Notes:                notes,
			}
		}
		row.Status = computedManifestStatus(g, row.InviteExpiresAt, row.RegistrationStatus, now)
		if g.RevokedAt == nil && row.CabinAssignment == nil {
			row.Status = "needs_cabin"
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (p *Pool) TripManifestSummaries(ctx context.Context, orgID uuid.UUID, tripIDs []uuid.UUID, now time.Time) (map[uuid.UUID]TripManifestSummary, error) {
	out := map[uuid.UUID]TripManifestSummary{}
	if len(tripIDs) == 0 {
		return out, nil
	}
	rows, err := p.Query(ctx, `
		SELECT t.id, t.num_guests, count(g.id)::int,
		       count(r.id) FILTER (WHERE r.status = 'submitted')::int
		FROM trips t
		LEFT JOIN trip_guests g ON g.trip_id = t.id AND g.revoked_at IS NULL
		LEFT JOIN guest_trip_registrations r ON r.trip_guest_id = g.id
		WHERE t.organization_id = $1 AND t.id = ANY($2::uuid[])
		GROUP BY t.id, t.num_guests
	`, orgID, tripIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s TripManifestSummary
		if err := rows.Scan(&s.TripID, &s.ExpectedCount, &s.GuestCount, &s.SubmittedCount); err != nil {
			return nil, err
		}
		s.HasWarning = s.ExpectedCount != nil && s.GuestCount > *s.ExpectedCount
		out[s.TripID] = s
	}
	return out, rows.Err()
}

func computedManifestStatus(g *TripGuest, inviteExpiresAt *time.Time, regStatus *string, now time.Time) string {
	if g.RevokedAt != nil {
		return "revoked"
	}
	if regStatus != nil {
		if *regStatus == "submitted" {
			return "submitted"
		}
		return "registration_draft"
	}
	if g.GuestUserID != nil {
		return "account_created"
	}
	if g.InviteSendStatus == "failed" {
		return "invite_failed"
	}
	if inviteExpiresAt != nil && inviteExpiresAt.Before(now) {
		return "expired"
	}
	if g.InviteSendStatus == "sent" {
		return "invited"
	}
	return "invite_not_sent"
}

func (p *Pool) AssertTripGuestAccess(ctx context.Context, guestUserID, tripGuestID uuid.UUID) (*TripGuest, error) {
	g := &TripGuest{}
	err := scanTripGuest(p.QueryRow(ctx, `
		SELECT `+tripGuestColumns+`
		FROM trip_guests
		WHERE id = $1 AND guest_user_id = $2 AND revoked_at IS NULL
	`, tripGuestID, guestUserID), g)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (p *Pool) UserAssignedToTrip(ctx context.Context, tripID, userID uuid.UUID) (bool, error) {
	var ok bool
	err := p.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM trip_cruise_directors WHERE trip_id = $1 AND user_id = $2)`, tripID, userID).Scan(&ok)
	return ok, err
}

func (p *Pool) UserAssignedToBoat(ctx context.Context, orgID, boatID, userID uuid.UUID) (bool, error) {
	var ok bool
	err := p.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM trips t
			JOIN trip_cruise_directors d ON d.trip_id = t.id
			WHERE t.organization_id = $1 AND t.boat_id = $2 AND d.user_id = $3
		)
	`, orgID, boatID, userID).Scan(&ok)
	return ok, err
}

func (s TripManifestSummary) Label() string {
	if s.ExpectedCount == nil {
		return fmt.Sprintf("%d guests", s.GuestCount)
	}
	return fmt.Sprintf("%d / %d", s.GuestCount, *s.ExpectedCount)
}
