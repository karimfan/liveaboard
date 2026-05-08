package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AuditActor struct {
	Type        string
	UserID      *uuid.UUID
	GuestUserID *uuid.UUID
}

type AuditEventInput struct {
	OrganizationID uuid.UUID
	Actor          AuditActor
	Action         string
	EntityType     string
	EntityID       *uuid.UUID
	TripID         *uuid.UUID
	TripGuestID    *uuid.UUID
	Metadata       map[string]any
}

type AuditEventFilters struct {
	Action      string
	EntityType  string
	TripID      *uuid.UUID
	TripGuestID *uuid.UUID
	ActorType   string
	DateFrom    *time.Time
	DateTo      *time.Time
	Limit       int
	AssignedTo  *uuid.UUID
}

type AuditEvent struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	ActorType        string
	ActorUserID      *uuid.UUID
	ActorGuestUserID *uuid.UUID
	Action           string
	EntityType       string
	EntityID         *uuid.UUID
	TripID           *uuid.UUID
	TripGuestID      *uuid.UUID
	Metadata         json.RawMessage
	CreatedAt        time.Time
}

const auditEventColumns = `id, organization_id, actor_type, actor_user_id, actor_guest_user_id,
	action, entity_type, entity_id, trip_id, trip_guest_id, metadata, created_at`

func StaffAuditActor(userID uuid.UUID) AuditActor {
	return AuditActor{Type: "staff", UserID: &userID}
}

func GuestAuditActor(guestUserID uuid.UUID) AuditActor {
	return AuditActor{Type: "guest", GuestUserID: &guestUserID}
}

func SystemAuditActor() AuditActor {
	return AuditActor{Type: "system"}
}

func (p *Pool) RecordAuditEvent(ctx context.Context, in AuditEventInput) (*AuditEvent, error) {
	return recordAuditEvent(ctx, p, in)
}

func (p *Pool) RecordAuditEventTx(ctx context.Context, tx pgx.Tx, in AuditEventInput) (*AuditEvent, error) {
	return recordAuditEvent(ctx, tx, in)
}

func recordAuditEvent(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, in AuditEventInput) (*AuditEvent, error) {
	if in.OrganizationID == uuid.Nil || in.Actor.Type == "" || in.Action == "" || in.EntityType == "" {
		return nil, ErrInvalidInput
	}
	meta := in.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	ev := &AuditEvent{}
	err = scanAuditEvent(q.QueryRow(ctx, `
		INSERT INTO audit_events (
			organization_id, actor_type, actor_user_id, actor_guest_user_id,
			action, entity_type, entity_id, trip_id, trip_guest_id, metadata
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING `+auditEventColumns,
		in.OrganizationID, in.Actor.Type, in.Actor.UserID, in.Actor.GuestUserID,
		in.Action, in.EntityType, in.EntityID, in.TripID, in.TripGuestID, raw,
	), ev)
	if err != nil {
		return nil, err
	}
	return ev, nil
}

func (p *Pool) AuditEvents(ctx context.Context, orgID uuid.UUID, f AuditEventFilters) ([]*AuditEvent, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	args := []any{orgID}
	sql := `SELECT ` + auditEventColumns + ` FROM audit_events e WHERE e.organization_id = $1`
	if f.Action != "" {
		args = append(args, f.Action)
		sql += ` AND e.action = $` + itoa(len(args))
	}
	if f.EntityType != "" {
		args = append(args, f.EntityType)
		sql += ` AND e.entity_type = $` + itoa(len(args))
	}
	if f.TripID != nil {
		args = append(args, *f.TripID)
		sql += ` AND e.trip_id = $` + itoa(len(args))
	}
	if f.TripGuestID != nil {
		args = append(args, *f.TripGuestID)
		sql += ` AND e.trip_guest_id = $` + itoa(len(args))
	}
	if f.ActorType != "" {
		args = append(args, f.ActorType)
		sql += ` AND e.actor_type = $` + itoa(len(args))
	}
	if f.DateFrom != nil {
		args = append(args, *f.DateFrom)
		sql += ` AND e.created_at >= $` + itoa(len(args))
	}
	if f.DateTo != nil {
		args = append(args, *f.DateTo)
		sql += ` AND e.created_at <= $` + itoa(len(args))
	}
	if f.AssignedTo != nil {
		args = append(args, *f.AssignedTo)
		sql += ` AND (e.trip_id IS NULL OR EXISTS (SELECT 1 FROM trip_cruise_directors d WHERE d.trip_id = e.trip_id AND d.user_id = $` + itoa(len(args)) + `))`
	}
	args = append(args, limit)
	sql += ` ORDER BY e.created_at DESC LIMIT $` + itoa(len(args))
	return scanAuditEvents(p.Query(ctx, sql, args...))
}

func (p *Pool) AuditEventsForTripGuest(ctx context.Context, orgID, tripID, tripGuestID uuid.UUID, limit int) ([]*AuditEvent, error) {
	return p.AuditEvents(ctx, orgID, AuditEventFilters{TripID: &tripID, TripGuestID: &tripGuestID, Limit: limit})
}

func scanAuditEvents(rows pgx.Rows, err error) ([]*AuditEvent, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AuditEvent
	for rows.Next() {
		ev := &AuditEvent{}
		if err := scanAuditEvent(rows, ev); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func scanAuditEvent(row interface{ Scan(dest ...any) error }, ev *AuditEvent) error {
	return row.Scan(
		&ev.ID, &ev.OrganizationID, &ev.ActorType, &ev.ActorUserID, &ev.ActorGuestUserID,
		&ev.Action, &ev.EntityType, &ev.EntityID, &ev.TripID, &ev.TripGuestID, &ev.Metadata, &ev.CreatedAt,
	)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
