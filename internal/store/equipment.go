package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EquipmentAsset is a serialized piece of dive gear owned by an
// org and (optionally) assigned to a boat. required_for_dive
// drives the readiness gate — if an asset is required-for-dive
// and either out_of_service or past service_due_on, the trip
// can't start without an operator override. Sprint 026 Anchor 3.
type EquipmentAsset struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	BoatID          *uuid.UUID
	AssetType       string
	Label           string
	SerialNumber    *string
	Status          string
	ServiceDueOn    *time.Time
	RequiredForDive bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateEquipmentAssetInput struct {
	OrganizationID  uuid.UUID
	BoatID          *uuid.UUID
	AssetType       string
	Label           string
	SerialNumber    *string
	ServiceDueOn    *time.Time
	RequiredForDive bool
}

const equipmentAssetColumns = `
	id, organization_id, boat_id, asset_type, label, serial_number,
	status, service_due_on, required_for_dive, created_at, updated_at
`

var validAssetTypes = map[string]struct{}{
	"bcd": {}, "regulator": {}, "tank": {}, "dive_computer": {},
	"o2_kit": {}, "other": {},
}

var validAssetStatuses = map[string]struct{}{
	"in_service": {}, "out_of_service": {}, "retired": {},
}

func scanEquipmentAsset(row interface{ Scan(dest ...any) error }, e *EquipmentAsset) error {
	return row.Scan(
		&e.ID, &e.OrganizationID, &e.BoatID, &e.AssetType, &e.Label, &e.SerialNumber,
		&e.Status, &e.ServiceDueOn, &e.RequiredForDive, &e.CreatedAt, &e.UpdatedAt,
	)
}

func (p *Pool) CreateEquipmentAsset(ctx context.Context, in CreateEquipmentAssetInput) (*EquipmentAsset, error) {
	if in.OrganizationID == uuid.Nil {
		return nil, errors.New("equipment: organization_id required")
	}
	if _, ok := validAssetTypes[in.AssetType]; !ok {
		return nil, errors.New("equipment: invalid asset_type")
	}
	label := strings.TrimSpace(in.Label)
	if label == "" {
		return nil, errors.New("equipment: label required")
	}
	out := &EquipmentAsset{}
	err := scanEquipmentAsset(p.QueryRow(ctx, `
		INSERT INTO equipment_assets (
			organization_id, boat_id, asset_type, label, serial_number,
			service_due_on, required_for_dive
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING `+equipmentAssetColumns,
		in.OrganizationID, in.BoatID, in.AssetType, label, in.SerialNumber,
		in.ServiceDueOn, in.RequiredForDive,
	), out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pool) GetEquipmentAsset(ctx context.Context, orgID, id uuid.UUID) (*EquipmentAsset, error) {
	out := &EquipmentAsset{}
	err := scanEquipmentAsset(p.QueryRow(ctx, `
		SELECT `+equipmentAssetColumns+`
		FROM equipment_assets
		WHERE organization_id = $1 AND id = $2
	`, orgID, id), out)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pool) ListEquipmentAssets(ctx context.Context, orgID uuid.UUID, boatID *uuid.UUID, includeRetired bool) ([]*EquipmentAsset, error) {
	q := `
		SELECT ` + equipmentAssetColumns + `
		FROM equipment_assets
		WHERE organization_id = $1
	`
	args := []any{orgID}
	if boatID != nil {
		q += " AND boat_id = $2"
		args = append(args, *boatID)
	}
	if !includeRetired {
		q += " AND status != 'retired'"
	}
	q += " ORDER BY asset_type, label"
	rows, err := p.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*EquipmentAsset
	for rows.Next() {
		e := &EquipmentAsset{}
		if err := scanEquipmentAsset(rows, e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type UpdateEquipmentAssetInput struct {
	BoatID          *uuid.UUID
	UnassignBoat    bool // when true, set boat_id = NULL
	Status          *string
	ServiceDueOn    *time.Time
	ClearServiceDue bool
	RequiredForDive *bool
	Label           *string
}

func (p *Pool) UpdateEquipmentAsset(ctx context.Context, orgID, id uuid.UUID, in UpdateEquipmentAssetInput) (*EquipmentAsset, error) {
	if in.Status != nil {
		if _, ok := validAssetStatuses[*in.Status]; !ok {
			return nil, errors.New("equipment: invalid status")
		}
	}
	out := &EquipmentAsset{}
	err := scanEquipmentAsset(p.QueryRow(ctx, `
		UPDATE equipment_assets
		SET boat_id            = CASE WHEN $1::boolean THEN NULL ELSE COALESCE($2, boat_id) END,
		    status             = COALESCE($3, status),
		    service_due_on     = CASE WHEN $4::boolean THEN NULL ELSE COALESCE($5, service_due_on) END,
		    required_for_dive  = COALESCE($6, required_for_dive),
		    label              = COALESCE($7, label),
		    updated_at         = now()
		WHERE organization_id = $8 AND id = $9
		RETURNING `+equipmentAssetColumns,
		in.UnassignBoat, in.BoatID, in.Status,
		in.ClearServiceDue, in.ServiceDueOn,
		in.RequiredForDive, in.Label,
		orgID, id,
	), out)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------
// Equipment service events
// ---------------------------------------------------------------

type EquipmentServiceEvent struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	EquipmentAssetID uuid.UUID
	EventType        string
	EventOn          time.Time
	Notes            *string
	RecordedByUserID uuid.UUID
	CreatedAt        time.Time
}

type RecordEquipmentServiceEventInput struct {
	OrganizationID   uuid.UUID
	EquipmentAssetID uuid.UUID
	EventType        string
	EventOn          time.Time
	Notes            *string
	RecordedByUserID uuid.UUID
}

var validServiceEventTypes = map[string]struct{}{
	"annual_service": {}, "vip_inspection": {}, "repair": {}, "retirement": {},
}

func (p *Pool) RecordEquipmentServiceEvent(ctx context.Context, in RecordEquipmentServiceEventInput) (*EquipmentServiceEvent, error) {
	if _, ok := validServiceEventTypes[in.EventType]; !ok {
		return nil, errors.New("service_event: invalid event_type")
	}
	if in.EventOn.IsZero() {
		return nil, errors.New("service_event: event_on required")
	}
	out := &EquipmentServiceEvent{}
	err := p.QueryRow(ctx, `
		INSERT INTO equipment_service_events (
			organization_id, equipment_asset_id, event_type, event_on, notes, recorded_by_user_id
		)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, organization_id, equipment_asset_id, event_type, event_on, notes, recorded_by_user_id, created_at
	`,
		in.OrganizationID, in.EquipmentAssetID, in.EventType, in.EventOn, in.Notes, in.RecordedByUserID,
	).Scan(
		&out.ID, &out.OrganizationID, &out.EquipmentAssetID, &out.EventType, &out.EventOn,
		&out.Notes, &out.RecordedByUserID, &out.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pool) ListEquipmentServiceEvents(ctx context.Context, orgID, assetID uuid.UUID) ([]*EquipmentServiceEvent, error) {
	rows, err := p.Query(ctx, `
		SELECT id, organization_id, equipment_asset_id, event_type, event_on, notes, recorded_by_user_id, created_at
		FROM equipment_service_events
		WHERE organization_id = $1 AND equipment_asset_id = $2
		ORDER BY event_on DESC
	`, orgID, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*EquipmentServiceEvent
	for rows.Next() {
		e := &EquipmentServiceEvent{}
		if err := rows.Scan(&e.ID, &e.OrganizationID, &e.EquipmentAssetID, &e.EventType, &e.EventOn,
			&e.Notes, &e.RecordedByUserID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
