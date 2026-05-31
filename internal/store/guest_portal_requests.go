package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// GuestPortalRequest is a guest-supplied equipment-rental or
// dietary form. Payload is application-shaped JSON; the schema
// doesn't constrain it. Sprint 026 Anchor 2.
type GuestPortalRequest struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	TripGuestID    uuid.UUID
	RequestType    string
	Payload        json.RawMessage
	UpdatedAt      time.Time
}

type UpsertGuestPortalRequestInput struct {
	OrganizationID uuid.UUID
	TripGuestID    uuid.UUID
	RequestType    string
	Payload        json.RawMessage
}

const guestPortalRequestColumns = `
	id, organization_id, trip_guest_id, request_type, payload, updated_at
`

func scanGuestPortalRequest(row interface{ Scan(dest ...any) error }, r *GuestPortalRequest) error {
	return row.Scan(&r.ID, &r.OrganizationID, &r.TripGuestID, &r.RequestType, &r.Payload, &r.UpdatedAt)
}

func (p *Pool) UpsertGuestPortalRequest(ctx context.Context, in UpsertGuestPortalRequestInput) (*GuestPortalRequest, error) {
	switch in.RequestType {
	case "equipment", "dietary":
	default:
		return nil, errors.New("portal_request: type must be equipment|dietary")
	}
	if len(in.Payload) == 0 {
		in.Payload = json.RawMessage(`{}`)
	}
	out := &GuestPortalRequest{}
	err := scanGuestPortalRequest(p.QueryRow(ctx, `
		INSERT INTO guest_portal_requests (organization_id, trip_guest_id, request_type, payload)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (trip_guest_id, request_type) DO UPDATE
		SET payload    = EXCLUDED.payload,
		    updated_at = now()
		RETURNING `+guestPortalRequestColumns,
		in.OrganizationID, in.TripGuestID, in.RequestType, in.Payload,
	), out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pool) ListGuestPortalRequests(ctx context.Context, orgID, tripGuestID uuid.UUID) ([]*GuestPortalRequest, error) {
	rows, err := p.Query(ctx, `
		SELECT `+guestPortalRequestColumns+`
		FROM guest_portal_requests
		WHERE organization_id = $1 AND trip_guest_id = $2
		ORDER BY request_type
	`, orgID, tripGuestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*GuestPortalRequest
	for rows.Next() {
		r := &GuestPortalRequest{}
		if err := scanGuestPortalRequest(rows, r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
