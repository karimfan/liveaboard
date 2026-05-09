package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	TripStatusPlanned   = "planned"
	TripStatusActive    = "active"
	TripStatusCompleted = "completed"
	TripStatusCancelled = "cancelled"
)

var ErrTripTransition = errors.New("store: invalid trip transition")

type TripReadinessIssue struct {
	Code        string    `json:"code"`
	Severity    string    `json:"severity"`
	Message     string    `json:"message"`
	TripGuestID uuid.UUID `json:"trip_guest_id,omitempty"`
}

type TripReadinessGuest struct {
	TripGuestID          uuid.UUID `json:"trip_guest_id"`
	FullName             string    `json:"full_name"`
	Email                string    `json:"email"`
	RegistrationStatus   *string   `json:"registration_status"`
	DocumentCount        int       `json:"document_count"`
	HasCabinAssignment   bool      `json:"has_cabin_assignment"`
	FolioStatus          *string   `json:"folio_status"`
	RegistrationComplete bool      `json:"registration_complete"`
}

type TripLifecycleReadiness struct {
	TripID      uuid.UUID            `json:"trip_id"`
	Status      string               `json:"status"`
	CanStart    bool                 `json:"can_start"`
	CanComplete bool                 `json:"can_complete"`
	Blockers    []TripReadinessIssue `json:"blockers"`
	Warnings    []TripReadinessIssue `json:"warnings"`
	Guests      []TripReadinessGuest `json:"guests"`
}

type TripLifecycleTransitionInput struct {
	ActorUserID          uuid.UUID
	OverrideUsed         bool
	OverrideRole         string
	Reason               string
	AcknowledgedWarnings []string
	Now                  time.Time
}

func (p *Pool) TripReadiness(ctx context.Context, orgID, tripID uuid.UUID, now time.Time) (*TripLifecycleReadiness, error) {
	trip, err := p.TripByID(ctx, orgID, tripID)
	if err != nil {
		return nil, err
	}
	out := &TripLifecycleReadiness{TripID: tripID, Status: trip.Status}
	rows, err := p.Query(ctx, `
		SELECT
			g.id, g.full_name, g.email,
			r.status,
			COALESCE(d.document_count, 0)::int,
			(a.id IS NOT NULL) AS has_assignment,
			f.status
		FROM trip_guests g
		LEFT JOIN guest_trip_registrations r ON r.trip_guest_id = g.id
		LEFT JOIN LATERAL (
			SELECT count(*) AS document_count
			FROM guest_documents gd
			WHERE gd.organization_id = g.organization_id
			  AND gd.trip_id = g.trip_id
			  AND gd.trip_guest_id = g.id
			  AND gd.archived_at IS NULL
		) d ON true
		LEFT JOIN trip_cabin_assignments a ON a.trip_guest_id = g.id AND a.unassigned_at IS NULL
		LEFT JOIN guest_folios f ON f.trip_guest_id = g.id
		WHERE g.organization_id = $1 AND g.trip_id = $2 AND g.revoked_at IS NULL
		ORDER BY lower(g.full_name), g.created_at
	`, orgID, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var g TripReadinessGuest
		if err := rows.Scan(&g.TripGuestID, &g.FullName, &g.Email, &g.RegistrationStatus, &g.DocumentCount, &g.HasCabinAssignment, &g.FolioStatus); err != nil {
			return nil, err
		}
		g.RegistrationComplete = g.RegistrationStatus != nil && *g.RegistrationStatus == "submitted"
		if !g.HasCabinAssignment {
			out.Blockers = append(out.Blockers, TripReadinessIssue{Code: "missing_berth", Severity: "blocker", Message: "Guest needs a cabin berth.", TripGuestID: g.TripGuestID})
		}
		if !g.RegistrationComplete {
			out.Blockers = append(out.Blockers, TripReadinessIssue{Code: "registration_not_submitted", Severity: "blocker", Message: "Guest registration is not submitted.", TripGuestID: g.TripGuestID})
		}
		if g.DocumentCount == 0 {
			out.Warnings = append(out.Warnings, TripReadinessIssue{Code: "missing_documents", Severity: "warning", Message: "Guest has no uploaded documents.", TripGuestID: g.TripGuestID})
		}
		if g.FolioStatus == nil || *g.FolioStatus != FolioStatusClosed {
			out.Warnings = append(out.Warnings, TripReadinessIssue{Code: "open_folios", Severity: "warning", Message: "Guest folio is open or missing.", TripGuestID: g.TripGuestID})
		}
		out.Guests = append(out.Guests, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out.Guests) == 0 {
		out.Warnings = append(out.Warnings, TripReadinessIssue{Code: "no_active_guests", Severity: "warning", Message: "Trip has no active guests."})
	}
	out.CanStart = trip.Status == TripStatusPlanned && !hasBlockers(out.Blockers)
	out.CanComplete = trip.Status == TripStatusActive
	return out, nil
}

func (p *Pool) StartTrip(ctx context.Context, orgID, tripID uuid.UUID, in TripLifecycleTransitionInput) (*Trip, error) {
	if in.Now.IsZero() {
		in.Now = time.Now().UTC()
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	t := &Trip{}
	if err := scanTrip(tx.QueryRow(ctx, `SELECT `+tripColumns+` FROM trips WHERE organization_id = $1 AND id = $2 FOR UPDATE`, orgID, tripID), t); err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if t.Status != TripStatusPlanned {
		return nil, ErrTripTransition
	}
	readiness, err := p.TripReadiness(ctx, orgID, tripID, in.Now)
	if err != nil {
		return nil, err
	}
	if hasBlockers(readiness.Blockers) {
		return nil, ErrTripTransition
	}
	if err := validateTransitionAcknowledgement(in, readiness.Warnings); err != nil {
		return nil, err
	}
	err = scanTrip(tx.QueryRow(ctx, `
		UPDATE trips
		SET status = 'active', started_at = $3, started_by_user_id = $4, updated_at = now()
		WHERE organization_id = $1 AND id = $2 AND status = 'planned'
		RETURNING `+tripColumns,
		orgID, tripID, in.Now, in.ActorUserID), t)
	if err != nil {
		return nil, err
	}
	_, err = p.RecordAuditEventTx(ctx, tx, AuditEventInput{
		OrganizationID: orgID,
		Actor:          StaffAuditActor(in.ActorUserID),
		Action:         "trip.started",
		EntityType:     "trip",
		EntityID:       &tripID,
		TripID:         &tripID,
		Metadata:       lifecycleMetadata("planned", "active", in, readiness.Warnings),
	})
	if err != nil {
		return nil, err
	}
	return t, tx.Commit(ctx)
}

func (p *Pool) CompleteTrip(ctx context.Context, orgID, tripID uuid.UUID, in TripLifecycleTransitionInput) (*Trip, error) {
	if in.Now.IsZero() {
		in.Now = time.Now().UTC()
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	t := &Trip{}
	if err := scanTrip(tx.QueryRow(ctx, `SELECT `+tripColumns+` FROM trips WHERE organization_id = $1 AND id = $2 FOR UPDATE`, orgID, tripID), t); err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if t.Status != TripStatusActive {
		return nil, ErrTripTransition
	}
	readiness, err := p.TripReadiness(ctx, orgID, tripID, in.Now)
	if err != nil {
		return nil, err
	}
	if err := validateTransitionAcknowledgement(in, readiness.Warnings); err != nil {
		return nil, err
	}
	err = scanTrip(tx.QueryRow(ctx, `
		UPDATE trips
		SET status = 'completed', completed_at = $3, completed_by_user_id = $4, updated_at = now()
		WHERE organization_id = $1 AND id = $2 AND status = 'active'
		RETURNING `+tripColumns,
		orgID, tripID, in.Now, in.ActorUserID), t)
	if err != nil {
		return nil, err
	}
	meta := lifecycleMetadata("active", "completed", in, readiness.Warnings)
	meta["open_folio_count"] = countWarning(readiness.Warnings, "open_folios")
	_, err = p.RecordAuditEventTx(ctx, tx, AuditEventInput{
		OrganizationID: orgID,
		Actor:          StaffAuditActor(in.ActorUserID),
		Action:         "trip.completed",
		EntityType:     "trip",
		EntityID:       &tripID,
		TripID:         &tripID,
		Metadata:       meta,
	})
	if err != nil {
		return nil, err
	}
	return t, tx.Commit(ctx)
}

func (p *Pool) CancelTrip(ctx context.Context, orgID, tripID, actorID uuid.UUID, reason string, now time.Time) (*Trip, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 {
		return nil, ErrInvalidInput
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	t := &Trip{}
	err = scanTrip(tx.QueryRow(ctx, `
		UPDATE trips
		SET status = 'cancelled', cancelled_at = $3, cancelled_by_user_id = $4, cancellation_reason = $5, updated_at = now()
		WHERE organization_id = $1 AND id = $2 AND status = 'planned'
		RETURNING `+tripColumns,
		orgID, tripID, now, actorID, reason), t)
	if isNoRows(err) {
		return nil, ErrTripTransition
	}
	if err != nil {
		return nil, err
	}
	_, err = p.RecordAuditEventTx(ctx, tx, AuditEventInput{
		OrganizationID: orgID,
		Actor:          StaffAuditActor(actorID),
		Action:         "trip.cancelled",
		EntityType:     "trip",
		EntityID:       &tripID,
		TripID:         &tripID,
		Metadata:       map[string]any{"previous_status": "planned", "new_status": "cancelled", "reason": reason},
	})
	if err != nil {
		return nil, err
	}
	return t, tx.Commit(ctx)
}

func validateTransitionAcknowledgement(in TripLifecycleTransitionInput, warnings []TripReadinessIssue) error {
	if in.OverrideUsed && strings.TrimSpace(in.Reason) == "" {
		return ErrInvalidInput
	}
	if len(warnings) > 0 && strings.TrimSpace(in.Reason) == "" {
		return ErrInvalidInput
	}
	if len(in.Reason) > 500 {
		return ErrInvalidInput
	}
	return nil
}

func hasBlockers(issues []TripReadinessIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "blocker" {
			return true
		}
	}
	return false
}

func lifecycleMetadata(prev, next string, in TripLifecycleTransitionInput, warnings []TripReadinessIssue) map[string]any {
	return map[string]any{
		"previous_status":       prev,
		"new_status":            next,
		"warning_codes":         warningCodes(warnings),
		"acknowledged_warnings": in.AcknowledgedWarnings,
		"override_used":         in.OverrideUsed,
		"override_role":         in.OverrideRole,
		"reason":                strings.TrimSpace(in.Reason),
	}
}

func warningCodes(warnings []TripReadinessIssue) []string {
	seen := map[string]bool{}
	var out []string
	for _, warning := range warnings {
		if !seen[warning.Code] {
			seen[warning.Code] = true
			out = append(out, warning.Code)
		}
	}
	return out
}

func countWarning(warnings []TripReadinessIssue, code string) int {
	n := 0
	for _, warning := range warnings {
		if warning.Code == code {
			n++
		}
	}
	return n
}
