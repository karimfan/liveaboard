package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CrewMember is one staff member on an org's roster — Captain,
// Dive Master, Chef, etc. Sprint 026 Anchor 3.
type CrewMember struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	UserID         *uuid.UUID
	Name           string
	Email          *string
	Role           string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateCrewMemberInput struct {
	OrganizationID uuid.UUID
	UserID         *uuid.UUID
	Name           string
	Email          *string
	Role           string
}

const crewMemberColumns = `
	id, organization_id, user_id, name, email, role, status, created_at, updated_at
`

func scanCrewMember(row interface{ Scan(dest ...any) error }, c *CrewMember) error {
	return row.Scan(
		&c.ID, &c.OrganizationID, &c.UserID, &c.Name, &c.Email, &c.Role, &c.Status,
		&c.CreatedAt, &c.UpdatedAt,
	)
}

var validCrewRoles = map[string]struct{}{
	"captain": {}, "dive_master": {}, "chef": {}, "deckhand": {},
	"engineer": {}, "cruise_director": {}, "other": {},
}

func (p *Pool) CreateCrewMember(ctx context.Context, in CreateCrewMemberInput) (*CrewMember, error) {
	if in.OrganizationID == uuid.Nil {
		return nil, errors.New("crew: organization_id required")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errors.New("crew: name required")
	}
	if _, ok := validCrewRoles[in.Role]; !ok {
		return nil, errors.New("crew: invalid role")
	}
	out := &CrewMember{}
	err := scanCrewMember(p.QueryRow(ctx, `
		INSERT INTO crew_members (organization_id, user_id, name, email, role)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING `+crewMemberColumns,
		in.OrganizationID, in.UserID, name, in.Email, in.Role,
	), out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pool) GetCrewMember(ctx context.Context, orgID, id uuid.UUID) (*CrewMember, error) {
	out := &CrewMember{}
	err := scanCrewMember(p.QueryRow(ctx, `
		SELECT `+crewMemberColumns+`
		FROM crew_members
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

func (p *Pool) ListCrewMembers(ctx context.Context, orgID uuid.UUID, includeInactive bool) ([]*CrewMember, error) {
	q := `
		SELECT ` + crewMemberColumns + `
		FROM crew_members
		WHERE organization_id = $1
	`
	if !includeInactive {
		q += " AND status = 'active'"
	}
	q += " ORDER BY name ASC"
	rows, err := p.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CrewMember
	for rows.Next() {
		c := &CrewMember{}
		if err := scanCrewMember(rows, c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type UpdateCrewMemberInput struct {
	Name   *string
	Email  *string
	Role   *string
	Status *string
}

func (p *Pool) UpdateCrewMember(ctx context.Context, orgID, id uuid.UUID, in UpdateCrewMemberInput) (*CrewMember, error) {
	if in.Role != nil {
		if _, ok := validCrewRoles[*in.Role]; !ok {
			return nil, errors.New("crew: invalid role")
		}
	}
	if in.Status != nil && *in.Status != "active" && *in.Status != "inactive" {
		return nil, errors.New("crew: invalid status")
	}
	out := &CrewMember{}
	err := scanCrewMember(p.QueryRow(ctx, `
		UPDATE crew_members
		SET name       = COALESCE($1, name),
		    email      = COALESCE($2, email),
		    role       = COALESCE($3, role),
		    status     = COALESCE($4, status),
		    updated_at = now()
		WHERE organization_id = $5 AND id = $6
		RETURNING `+crewMemberColumns,
		in.Name, in.Email, in.Role, in.Status, orgID, id,
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
// Crew certifications
// ---------------------------------------------------------------

type CrewCertification struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
	CrewMemberID        uuid.UUID
	CertificationType   string
	Issuer              *string
	CertificationNumber *string
	ExpiresOn           *time.Time
	RequiredForRoles    []string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CreateCrewCertificationInput struct {
	OrganizationID      uuid.UUID
	CrewMemberID        uuid.UUID
	CertificationType   string
	Issuer              *string
	CertificationNumber *string
	ExpiresOn           *time.Time
	RequiredForRoles    []string
}

const crewCertificationColumns = `
	id, organization_id, crew_member_id, certification_type,
	issuer, certification_number, expires_on, required_for_roles,
	created_at, updated_at
`

func scanCrewCertification(row interface{ Scan(dest ...any) error }, c *CrewCertification) error {
	return row.Scan(
		&c.ID, &c.OrganizationID, &c.CrewMemberID, &c.CertificationType,
		&c.Issuer, &c.CertificationNumber, &c.ExpiresOn, &c.RequiredForRoles,
		&c.CreatedAt, &c.UpdatedAt,
	)
}

func (p *Pool) CreateCrewCertification(ctx context.Context, in CreateCrewCertificationInput) (*CrewCertification, error) {
	if in.OrganizationID == uuid.Nil || in.CrewMemberID == uuid.Nil {
		return nil, errors.New("cert: ids required")
	}
	t := strings.TrimSpace(in.CertificationType)
	if t == "" {
		return nil, errors.New("cert: certification_type required")
	}
	if in.RequiredForRoles == nil {
		in.RequiredForRoles = []string{}
	}
	out := &CrewCertification{}
	err := scanCrewCertification(p.QueryRow(ctx, `
		INSERT INTO crew_certifications (
			organization_id, crew_member_id, certification_type,
			issuer, certification_number, expires_on, required_for_roles
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING `+crewCertificationColumns,
		in.OrganizationID, in.CrewMemberID, t,
		in.Issuer, in.CertificationNumber, in.ExpiresOn, in.RequiredForRoles,
	), out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pool) ListCrewCertifications(ctx context.Context, orgID, crewMemberID uuid.UUID) ([]*CrewCertification, error) {
	rows, err := p.Query(ctx, `
		SELECT `+crewCertificationColumns+`
		FROM crew_certifications
		WHERE organization_id = $1 AND crew_member_id = $2
		ORDER BY expires_on ASC NULLS LAST
	`, orgID, crewMemberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CrewCertification
	for rows.Next() {
		c := &CrewCertification{}
		if err := scanCrewCertification(rows, c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListAllCertsForOrg returns every cert for the org so callers
// can compute expiry status (the Crew page renders the whole
// org's roster + certs at once). Joined to crew_members in
// memory by the caller.
func (p *Pool) ListAllCrewCertificationsForOrg(ctx context.Context, orgID uuid.UUID) ([]*CrewCertification, error) {
	rows, err := p.Query(ctx, `
		SELECT `+crewCertificationColumns+`
		FROM crew_certifications
		WHERE organization_id = $1
		ORDER BY expires_on ASC NULLS LAST
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CrewCertification
	for rows.Next() {
		c := &CrewCertification{}
		if err := scanCrewCertification(rows, c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (p *Pool) DeleteCrewCertification(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := p.Exec(ctx, `
		DELETE FROM crew_certifications
		WHERE organization_id = $1 AND id = $2
	`, orgID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------
// Trip crew assignments
// ---------------------------------------------------------------

type TripCrewAssignment struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	TripID         uuid.UUID
	CrewMemberID   uuid.UUID
	Role           string
	AssignedAt     time.Time
}

func (p *Pool) AssignCrewToTrip(ctx context.Context, orgID, tripID, crewID uuid.UUID, role string) (*TripCrewAssignment, error) {
	role = strings.TrimSpace(role)
	if role == "" {
		return nil, errors.New("assignment: role required")
	}
	out := &TripCrewAssignment{}
	err := p.QueryRow(ctx, `
		INSERT INTO trip_crew_assignments (organization_id, trip_id, crew_member_id, role)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (trip_id, crew_member_id) DO UPDATE
		SET role = EXCLUDED.role
		RETURNING id, organization_id, trip_id, crew_member_id, role, assigned_at
	`, orgID, tripID, crewID, role).Scan(
		&out.ID, &out.OrganizationID, &out.TripID, &out.CrewMemberID, &out.Role, &out.AssignedAt,
	)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pool) UnassignCrewFromTrip(ctx context.Context, orgID, tripID, crewID uuid.UUID) error {
	tag, err := p.Exec(ctx, `
		DELETE FROM trip_crew_assignments
		WHERE organization_id = $1 AND trip_id = $2 AND crew_member_id = $3
	`, orgID, tripID, crewID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Pool) ListTripCrewAssignments(ctx context.Context, orgID, tripID uuid.UUID) ([]*TripCrewAssignment, error) {
	rows, err := p.Query(ctx, `
		SELECT id, organization_id, trip_id, crew_member_id, role, assigned_at
		FROM trip_crew_assignments
		WHERE organization_id = $1 AND trip_id = $2
		ORDER BY role
	`, orgID, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*TripCrewAssignment
	for rows.Next() {
		a := &TripCrewAssignment{}
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.TripID, &a.CrewMemberID, &a.Role, &a.AssignedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
