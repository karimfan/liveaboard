package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ReadinessFailure is one structured reason a trip can't start.
// The handler returns a slice of these as the 409 body so the UI
// can render the exact resolution path per failure.
type ReadinessFailure struct {
	Kind              string     // crew_role_unfilled | crew_cert_expired | equipment_out_of_service | equipment_service_due
	CrewMemberID      *uuid.UUID `json:",omitempty"`
	CrewMemberName    string     `json:",omitempty"`
	CertificationType string     `json:",omitempty"`
	ExpiresOn         *time.Time `json:",omitempty"`
	AssetID           *uuid.UUID `json:",omitempty"`
	AssetLabel        string     `json:",omitempty"`
	AssetStatus       string     `json:",omitempty"`
	Detail            string     `json:",omitempty"`
}

// ComputeTripReadiness runs the supply-side readiness check for a
// trip. Returns the list of failures; an empty slice means the
// trip is ready to start. Computation is server-side authoritative
// — the handler invokes it inside the start-transition transaction
// so a concurrent change can't sneak past the gate.
//
// On `start`, the failures are:
//
//  1. crew_cert_expired: any cert with expires_on < startDate
//     AND required_for_roles intersects an assigned crew member's
//     role (or the certified member is themselves the assigned
//     crew for that role).
//  2. equipment_out_of_service: assets on the trip's boat that
//     are required_for_dive AND status != in_service.
//  3. equipment_service_due: assets on the trip's boat that are
//     required_for_dive AND service_due_on < startDate.
//
// The "required crew role unfilled" check is deferred to a
// follow-up sprint — it needs a per-org role-requirements config
// that doesn't ship in Sprint 026.
func (p *Pool) ComputeTripReadiness(ctx context.Context, orgID, tripID uuid.UUID, startDate time.Time) ([]ReadinessFailure, error) {
	out := []ReadinessFailure{}

	// --- 1. Crew cert expiry: certs whose required_for_roles
	//        contains the role the crew is assigned to on this
	//        trip, and that expire before the trip start date.
	rows, err := p.Query(ctx, `
		SELECT cm.id, cm.name, cc.certification_type, cc.expires_on
		FROM trip_crew_assignments tca
		JOIN crew_members cm  ON cm.id = tca.crew_member_id
		JOIN crew_certifications cc ON cc.crew_member_id = cm.id
		WHERE tca.organization_id = $1 AND tca.trip_id = $2
		  AND cc.required_for_roles && ARRAY[tca.role]::text[]
		  AND cc.expires_on IS NOT NULL
		  AND cc.expires_on < $3
	`, orgID, tripID, startDate)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var crewID uuid.UUID
		var name, certType string
		var expiresOn *time.Time
		if err := rows.Scan(&crewID, &name, &certType, &expiresOn); err != nil {
			rows.Close()
			return nil, err
		}
		f := ReadinessFailure{
			Kind:              "crew_cert_expired",
			CrewMemberID:      &crewID,
			CrewMemberName:    name,
			CertificationType: certType,
			ExpiresOn:         expiresOn,
		}
		out = append(out, f)
	}
	rows.Close()

	// --- 2 + 3. Equipment on this trip's boat, required for
	//            dives, that's out of service OR past service due.
	rows, err = p.Query(ctx, `
		SELECT ea.id, ea.label, ea.status, ea.service_due_on
		FROM trips t
		JOIN equipment_assets ea ON ea.boat_id = t.boat_id
		WHERE t.organization_id = $1 AND t.id = $2
		  AND ea.organization_id = $1
		  AND ea.required_for_dive = true
		  AND (
		        ea.status != 'in_service'
		     OR (ea.service_due_on IS NOT NULL AND ea.service_due_on < $3)
		  )
	`, orgID, tripID, startDate)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		var label, status string
		var serviceDue *time.Time
		if err := rows.Scan(&id, &label, &status, &serviceDue); err != nil {
			rows.Close()
			return nil, err
		}
		assetID := id
		f := ReadinessFailure{
			AssetID:     &assetID,
			AssetLabel:  label,
			AssetStatus: status,
		}
		if status != "in_service" {
			f.Kind = "equipment_out_of_service"
		} else {
			f.Kind = "equipment_service_due"
			f.ExpiresOn = serviceDue
		}
		out = append(out, f)
	}
	rows.Close()

	return out, nil
}
