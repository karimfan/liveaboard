package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TripDayPlan is one day's schedule and dive plan for a trip.
// schedule_json + dive_plan_json are application-shaped JSON
// arrays; the schema doesn't constrain their contents.
// Sprint 026 Anchor 2.
type TripDayPlan struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	TripID          uuid.UUID
	DayDate         time.Time
	Title           string
	Schedule        json.RawMessage
	DivePlan        json.RawMessage
	UpdatedByUserID uuid.UUID
	UpdatedAt       time.Time
}

type UpsertTripDayPlanInput struct {
	OrganizationID  uuid.UUID
	TripID          uuid.UUID
	DayDate         time.Time
	Title           string
	Schedule        json.RawMessage
	DivePlan        json.RawMessage
	UpdatedByUserID uuid.UUID
}

const tripDayPlanColumns = `
	id, organization_id, trip_id, day_date, title,
	schedule_json, dive_plan_json, updated_by_user_id, updated_at
`

func scanTripDayPlan(row interface{ Scan(dest ...any) error }, d *TripDayPlan) error {
	return row.Scan(
		&d.ID, &d.OrganizationID, &d.TripID, &d.DayDate, &d.Title,
		&d.Schedule, &d.DivePlan, &d.UpdatedByUserID, &d.UpdatedAt,
	)
}

func (p *Pool) UpsertTripDayPlan(ctx context.Context, in UpsertTripDayPlanInput) (*TripDayPlan, error) {
	if in.OrganizationID == uuid.Nil || in.TripID == uuid.Nil || in.UpdatedByUserID == uuid.Nil {
		return nil, errors.New("day_plan: ids required")
	}
	title := strings.TrimSpace(in.Title)
	if len(in.Schedule) == 0 {
		in.Schedule = json.RawMessage(`[]`)
	}
	if len(in.DivePlan) == 0 {
		in.DivePlan = json.RawMessage(`[]`)
	}
	out := &TripDayPlan{}
	err := scanTripDayPlan(p.QueryRow(ctx, `
		INSERT INTO trip_day_plans (
			organization_id, trip_id, day_date, title,
			schedule_json, dive_plan_json, updated_by_user_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (trip_id, day_date) DO UPDATE
		SET title              = EXCLUDED.title,
		    schedule_json      = EXCLUDED.schedule_json,
		    dive_plan_json     = EXCLUDED.dive_plan_json,
		    updated_by_user_id = EXCLUDED.updated_by_user_id,
		    updated_at         = now()
		RETURNING `+tripDayPlanColumns,
		in.OrganizationID, in.TripID, in.DayDate, title,
		in.Schedule, in.DivePlan, in.UpdatedByUserID,
	), out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pool) ListTripDayPlans(ctx context.Context, orgID, tripID uuid.UUID) ([]*TripDayPlan, error) {
	rows, err := p.Query(ctx, `
		SELECT `+tripDayPlanColumns+`
		FROM trip_day_plans
		WHERE organization_id = $1 AND trip_id = $2
		ORDER BY day_date ASC
	`, orgID, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*TripDayPlan
	for rows.Next() {
		d := &TripDayPlan{}
		if err := scanTripDayPlan(rows, d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (p *Pool) GetTripDayPlanForDate(ctx context.Context, orgID, tripID uuid.UUID, day time.Time) (*TripDayPlan, error) {
	out := &TripDayPlan{}
	err := scanTripDayPlan(p.QueryRow(ctx, `
		SELECT `+tripDayPlanColumns+`
		FROM trip_day_plans
		WHERE organization_id = $1 AND trip_id = $2 AND day_date = $3
	`, orgID, tripID, day), out)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}
