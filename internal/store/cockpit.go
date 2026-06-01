package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Cockpit struct {
	Identity        CockpitIdentity       `json:"identity"`
	AdminCockpit    *AdminCockpitPulse    `json:"admin_cockpit,omitempty"`
	DirectorCockpit *DirectorCockpitPulse `json:"director_cockpit,omitempty"`
	VoyageLanes     []CockpitVoyage       `json:"voyage_lanes"`
	Blockers        []CockpitSignal       `json:"blockers"`
	Money           CockpitMoney          `json:"money"`
	Inventory       []CockpitInventory    `json:"inventory"`
	Activity        []CockpitActivity     `json:"activity"`
	Commands        []CockpitCommand      `json:"commands"`
	FailedSections  []string              `json:"failed_sections,omitempty"`
}

type CockpitIdentity struct {
	UserID          uuid.UUID `json:"user_id"`
	OrganizationID  uuid.UUID `json:"organization_id"`
	Role            string    `json:"role"`
	AssignedVoyages int       `json:"assigned_voyages"`
	GeneratedAt     time.Time `json:"generated_at"`
}

type AdminCockpitPulse struct {
	SetupPercent    int `json:"setup_percent"`
	BoatCount       int `json:"boat_count"`
	TripCount       int `json:"trip_count"`
	DirectorCount   int `json:"director_count"`
	LeadCount       int `json:"lead_count"`
	QuoteCount      int `json:"quote_count"`
	ActiveVoyages   int `json:"active_voyages"`
	UpcomingVoyages int `json:"upcoming_voyages"`
}

type DirectorCockpitPulse struct {
	AssignedVoyages int `json:"assigned_voyages"`
	ActiveVoyages   int `json:"active_voyages"`
	UpcomingVoyages int `json:"upcoming_voyages"`
	OpenFolios      int `json:"open_folios"`
	Blockers        int `json:"blockers"`
}

type CockpitVoyage struct {
	ID                  uuid.UUID `json:"id"`
	BoatID              uuid.UUID `json:"boat_id"`
	BoatName            string    `json:"boat_name"`
	Itinerary           string    `json:"itinerary"`
	StartDate           time.Time `json:"start_date"`
	EndDate             time.Time `json:"end_date"`
	Status              string    `json:"status"`
	ExpectedGuests      *int      `json:"expected_guests,omitempty"`
	GuestCount          int       `json:"guest_count"`
	SubmittedCount      int       `json:"submitted_count"`
	CabinAssignments    int       `json:"cabin_assignments"`
	DirectorCount       int       `json:"director_count"`
	OpenFolioCount      int       `json:"open_folio_count"`
	OutstandingUSDCents int64     `json:"outstanding_usd_cents"`
	BlockerCount        int       `json:"blocker_count"`
}

type CockpitSignal struct {
	Kind     string     `json:"kind"`
	Severity string     `json:"severity"`
	Label    string     `json:"label"`
	Detail   string     `json:"detail"`
	TripID   *uuid.UUID `json:"trip_id,omitempty"`
	BoatID   *uuid.UUID `json:"boat_id,omitempty"`
	EntityID *uuid.UUID `json:"entity_id,omitempty"`
	Route    string     `json:"route"`
}

type CockpitMoney struct {
	OpenFolioCount         int   `json:"open_folio_count"`
	ClosedFolioCount       int   `json:"closed_folio_count"`
	OutstandingUSDCents    int64 `json:"outstanding_usd_cents"`
	SettledUSDCents        int64 `json:"settled_usd_cents"`
	DepositPendingQuotes   int   `json:"deposit_pending_quotes"`
	HeldQuotes             int   `json:"held_quotes"`
	OfflineDepositUSDCents int64 `json:"offline_deposit_usd_cents"`
	OfflineRefundUSDCents  int64 `json:"offline_refund_usd_cents"`
}

type CockpitInventory struct {
	BoatID   uuid.UUID `json:"boat_id"`
	BoatName string    `json:"boat_name"`
	ItemName string    `json:"item_name"`
	Status   string    `json:"status"`
	Quantity int       `json:"quantity"`
	Route    string    `json:"route"`
}

type CockpitActivity struct {
	ID        uuid.UUID  `json:"id"`
	Action    string     `json:"action"`
	Entity    string     `json:"entity"`
	TripID    *uuid.UUID `json:"trip_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type CockpitCommand struct {
	Label string `json:"label"`
	Route string `json:"route"`
	Kind  string `json:"kind"`
}

func (p *Pool) Cockpit(ctx context.Context, orgID, userID uuid.UUID, role string, now time.Time) (*Cockpit, error) {
	c := &Cockpit{
		Identity:    CockpitIdentity{UserID: userID, OrganizationID: orgID, Role: role, GeneratedAt: now},
		VoyageLanes: []CockpitVoyage{},
		Blockers:    []CockpitSignal{},
		Inventory:   []CockpitInventory{},
		Activity:    []CockpitActivity{},
		Commands:    cockpitCommands(role),
	}
	var err error
	c.Identity.AssignedVoyages, err = p.assignedVoyageCount(ctx, orgID, userID)
	if err != nil {
		if cockpitFatalErr(err) {
			return nil, err
		}
		c.FailedSections = append(c.FailedSections, "assigned_voyages")
	}
	c.VoyageLanes, err = p.cockpitVoyages(ctx, orgID, userID, role, now)
	if err != nil {
		if cockpitFatalErr(err) {
			return nil, err
		}
		c.VoyageLanes = []CockpitVoyage{}
		c.FailedSections = append(c.FailedSections, "voyage_lanes")
	}
	c.Blockers, err = p.cockpitBlockers(ctx, orgID, userID, role, now)
	if err != nil {
		if cockpitFatalErr(err) {
			return nil, err
		}
		c.Blockers = []CockpitSignal{}
		c.FailedSections = append(c.FailedSections, "blockers")
	}
	c.Money, err = p.cockpitMoney(ctx, orgID, userID, role)
	if err != nil {
		if cockpitFatalErr(err) {
			return nil, err
		}
		c.FailedSections = append(c.FailedSections, "money")
	}
	if role == RoleOrgAdmin {
		setupPercent := 0
		setup, err := p.SetupCompleteness(ctx, orgID)
		if err != nil {
			if cockpitFatalErr(err) {
				return nil, err
			}
			c.FailedSections = append(c.FailedSections, "setup")
		} else {
			setupPercent = setup.Percent
		}
		c.AdminCockpit, err = p.adminCockpitPulse(ctx, orgID, setupPercent, now)
		if err != nil {
			if cockpitFatalErr(err) {
				return nil, err
			}
			c.AdminCockpit = &AdminCockpitPulse{SetupPercent: setupPercent}
			c.FailedSections = append(c.FailedSections, "admin_cockpit")
		}
		c.Inventory, err = p.cockpitInventory(ctx, orgID)
		if err != nil {
			if cockpitFatalErr(err) {
				return nil, err
			}
			c.Inventory = []CockpitInventory{}
			c.FailedSections = append(c.FailedSections, "inventory")
		}
	} else {
		c.DirectorCockpit = directorPulse(c.VoyageLanes, c.Blockers, c.Money)
	}
	c.Activity, err = p.cockpitActivity(ctx, orgID, userID, role)
	if err != nil {
		if cockpitFatalErr(err) {
			return nil, err
		}
		c.Activity = []CockpitActivity{}
		c.FailedSections = append(c.FailedSections, "activity")
	}
	return c, nil
}

func cockpitFatalErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (p *Pool) assignedVoyageCount(ctx context.Context, orgID, userID uuid.UUID) (int, error) {
	var n int
	err := p.QueryRow(ctx, `
		SELECT count(*)::int
		FROM trips t
		JOIN trip_cruise_directors d ON d.trip_id = t.id
		WHERE t.organization_id = $1 AND d.user_id = $2
		  AND t.status IN ('planned','active')
		  AND t.removed_from_source_at IS NULL
	`, orgID, userID).Scan(&n)
	return n, err
}

func (p *Pool) cockpitVoyages(ctx context.Context, orgID, userID uuid.UUID, role string, now time.Time) ([]CockpitVoyage, error) {
	args := []any{orgID, now.AddDate(0, 0, -7), now.AddDate(0, 0, 180)}
	assignment := ""
	if role != RoleOrgAdmin {
		args = append(args, userID)
		assignment = " AND EXISTS (SELECT 1 FROM trip_cruise_directors cd WHERE cd.trip_id = t.id AND cd.user_id = $4)"
	}
	rows, err := p.Query(ctx, `
		SELECT
			t.id, t.boat_id, b.display_name, t.itinerary, t.start_date, t.end_date,
			t.status, t.num_guests,
			(SELECT count(*)::int
			   FROM trip_guests g
			  WHERE g.organization_id = t.organization_id AND g.trip_id = t.id AND g.revoked_at IS NULL) AS guest_count,
			(SELECT count(*)::int
			   FROM guest_trip_registrations r
			   JOIN trip_guests g ON g.id = r.trip_guest_id
			  WHERE g.organization_id = t.organization_id AND g.trip_id = t.id
			    AND g.revoked_at IS NULL AND r.status = 'submitted') AS submitted_count,
			(SELECT count(*)::int
			   FROM trip_cabin_assignments ca
			  WHERE ca.organization_id = t.organization_id AND ca.trip_id = t.id AND ca.unassigned_at IS NULL) AS cabin_assignments,
			(SELECT count(*)::int
			   FROM trip_cruise_directors d
			  WHERE d.trip_id = t.id) AS director_count,
			(SELECT count(*)::int
			   FROM guest_folios f
			  WHERE f.organization_id = t.organization_id AND f.trip_id = t.id AND f.status = 'open') AS open_folio_count,
			(SELECT coalesce(sum(f.total_usd_cents), 0)::bigint
			   FROM guest_folios f
			  WHERE f.organization_id = t.organization_id AND f.trip_id = t.id AND f.status = 'open') AS outstanding_usd_cents
		FROM trips t
		JOIN boats b ON b.id = t.boat_id
		WHERE t.organization_id = $1
		  AND t.removed_from_source_at IS NULL
		  AND t.status IN ('planned','active')
		  AND t.start_date BETWEEN $2 AND $3
		`+assignment+`
		ORDER BY CASE WHEN t.status = 'active' THEN 0 ELSE 1 END, t.start_date
		LIMIT 10
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CockpitVoyage{}
	for rows.Next() {
		var v CockpitVoyage
		if err := rows.Scan(&v.ID, &v.BoatID, &v.BoatName, &v.Itinerary, &v.StartDate, &v.EndDate,
			&v.Status, &v.ExpectedGuests, &v.GuestCount, &v.SubmittedCount, &v.CabinAssignments,
			&v.DirectorCount, &v.OpenFolioCount, &v.OutstandingUSDCents); err != nil {
			return nil, err
		}
		if v.DirectorCount == 0 {
			v.BlockerCount++
		}
		if v.ExpectedGuests != nil && v.GuestCount < *v.ExpectedGuests {
			v.BlockerCount++
		}
		if v.GuestCount > 0 && v.CabinAssignments < v.GuestCount {
			v.BlockerCount++
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (p *Pool) cockpitBlockers(ctx context.Context, orgID, userID uuid.UUID, role string, now time.Time) ([]CockpitSignal, error) {
	args := []any{orgID, now, now.AddDate(0, 0, 90)}
	assignment := ""
	if role != RoleOrgAdmin {
		args = append(args, userID)
		assignment = " AND EXISTS (SELECT 1 FROM trip_cruise_directors cd WHERE cd.trip_id = t.id AND cd.user_id = $4)"
	}
	rows, err := p.Query(ctx, `
		SELECT t.id, t.boat_id, b.display_name, t.itinerary, t.start_date,
		       count(DISTINCT d.user_id)::int,
		       count(DISTINCT g.id) FILTER (WHERE g.revoked_at IS NULL)::int,
		       count(DISTINCT ca.id) FILTER (WHERE ca.unassigned_at IS NULL)::int
		FROM trips t
		JOIN boats b ON b.id = t.boat_id
		LEFT JOIN trip_cruise_directors d ON d.trip_id = t.id
		LEFT JOIN trip_guests g ON g.trip_id = t.id AND g.organization_id = t.organization_id
		LEFT JOIN trip_cabin_assignments ca ON ca.trip_id = t.id AND ca.trip_guest_id = g.id AND ca.organization_id = t.organization_id AND ca.unassigned_at IS NULL
		WHERE t.organization_id = $1 AND t.status = 'planned'
		  AND t.removed_from_source_at IS NULL
		  AND t.start_date BETWEEN $2 AND $3
		`+assignment+`
		GROUP BY t.id, t.boat_id, b.display_name, t.itinerary, t.start_date
		ORDER BY t.start_date
		LIMIT 8
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CockpitSignal{}
	for rows.Next() {
		var tripID, boatID uuid.UUID
		var boatName, itinerary string
		var start time.Time
		var directors, guests, cabins int
		if err := rows.Scan(&tripID, &boatID, &boatName, &itinerary, &start, &directors, &guests, &cabins); err != nil {
			return nil, err
		}
		route := "/admin/trips/" + tripID.String() + "/dashboard"
		if directors == 0 {
			out = append(out, CockpitSignal{Kind: "director", Severity: "blocker", Label: "Director unassigned", Detail: boatName + " · " + itinerary, TripID: &tripID, BoatID: &boatID, Route: "/admin/trips"})
		}
		if guests > 0 && cabins < guests {
			out = append(out, CockpitSignal{Kind: "cabins", Severity: "warning", Label: "Cabins incomplete", Detail: boatName + " · " + itinerary, TripID: &tripID, BoatID: &boatID, Route: route})
		}
	}
	if role == RoleOrgAdmin {
		more, err := p.supplyBlockers(ctx, orgID, now)
		if err != nil && cockpitFatalErr(err) {
			return nil, err
		}
		if err == nil {
			out = append(out, more...)
		}
	}
	if len(out) > 12 {
		out = out[:12]
	}
	return out, nil
}

func (p *Pool) supplyBlockers(ctx context.Context, orgID uuid.UUID, now time.Time) ([]CockpitSignal, error) {
	rows, err := p.Query(ctx, `
		SELECT id, name, certification_type, expires_on
		FROM (
			SELECT cm.id, cm.name, cc.certification_type, cc.expires_on
			FROM crew_members cm
			JOIN crew_certifications cc ON cc.crew_member_id = cm.id AND cc.organization_id = cm.organization_id
			WHERE cm.organization_id = $1 AND cm.status = 'active'
			  AND cc.expires_on IS NOT NULL AND cc.expires_on <= $2 + INTERVAL '30 days'
			ORDER BY cc.expires_on
			LIMIT 4
		) x
	`, orgID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CockpitSignal{}
	for rows.Next() {
		var id uuid.UUID
		var name, cert string
		var expires time.Time
		if err := rows.Scan(&id, &name, &cert, &expires); err != nil {
			return nil, err
		}
		severity := "warning"
		if expires.Before(now) {
			severity = "blocker"
		}
		out = append(out, CockpitSignal{Kind: "crew_cert", Severity: severity, Label: "Crew certification", Detail: name + " · " + cert, EntityID: &id, Route: "/admin/users"})
	}
	rows.Close()
	rows, err = p.Query(ctx, `
		SELECT ea.id, ea.boat_id, coalesce(b.display_name, 'Unassigned'), ea.label, ea.status
		FROM equipment_assets ea
		LEFT JOIN boats b ON b.id = ea.boat_id
		WHERE ea.organization_id = $1
		  AND ea.required_for_dive = true
		  AND (ea.status != 'in_service' OR (ea.service_due_on IS NOT NULL AND ea.service_due_on <= $2 + INTERVAL '30 days'))
		ORDER BY ea.status, ea.service_due_on NULLS LAST, ea.label
		LIMIT 4
	`, orgID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var boatID *uuid.UUID
		var boat, label, status string
		if err := rows.Scan(&id, &boatID, &boat, &label, &status); err != nil {
			return nil, err
		}
		severity := "warning"
		if status != "in_service" {
			severity = "blocker"
		}
		out = append(out, CockpitSignal{Kind: "equipment", Severity: severity, Label: "Equipment readiness", Detail: boat + " · " + label, EntityID: &id, BoatID: boatID, Route: "/admin/fleet"})
	}
	return out, rows.Err()
}

func (p *Pool) cockpitMoney(ctx context.Context, orgID, userID uuid.UUID, role string) (CockpitMoney, error) {
	args := []any{orgID}
	assignment := ""
	if role != RoleOrgAdmin {
		args = append(args, userID)
		assignment = " AND EXISTS (SELECT 1 FROM trip_cruise_directors cd WHERE cd.trip_id = f.trip_id AND cd.user_id = $2)"
	}
	var m CockpitMoney
	if err := p.QueryRow(ctx, `
		SELECT
			coalesce(sum(CASE WHEN f.status = 'open' THEN 1 ELSE 0 END), 0)::int,
			coalesce(sum(CASE WHEN f.status = 'closed' THEN 1 ELSE 0 END), 0)::int,
			coalesce(sum(CASE WHEN f.status = 'open' THEN f.total_usd_cents ELSE 0 END), 0)::bigint,
			coalesce(sum(CASE WHEN f.status = 'closed' THEN f.total_usd_cents ELSE 0 END), 0)::bigint
		FROM guest_folios f
		WHERE f.organization_id = $1
		`+assignment, args...).Scan(&m.OpenFolioCount, &m.ClosedFolioCount, &m.OutstandingUSDCents, &m.SettledUSDCents); err != nil {
		return m, err
	}
	if role == RoleOrgAdmin {
		_ = p.QueryRow(ctx, `
			SELECT
				count(*) FILTER (WHERE status = 'deposit_pending')::int,
				count(*) FILTER (WHERE status = 'held')::int
			FROM booking_quotes
			WHERE organization_id = $1
		`, orgID).Scan(&m.DepositPendingQuotes, &m.HeldQuotes)
		_ = p.QueryRow(ctx, `
			SELECT
				coalesce(sum(amount_cents) FILTER (WHERE direction = 'deposit'), 0)::bigint,
				coalesce(sum(amount_cents) FILTER (WHERE direction = 'refund'), 0)::bigint
			FROM offline_payments
			WHERE organization_id = $1 AND currency = 'USD'
		`, orgID).Scan(&m.OfflineDepositUSDCents, &m.OfflineRefundUSDCents)
	}
	return m, nil
}

func (p *Pool) adminCockpitPulse(ctx context.Context, orgID uuid.UUID, setupPercent int, now time.Time) (*AdminCockpitPulse, error) {
	pulse := &AdminCockpitPulse{SetupPercent: setupPercent}
	var err error
	if pulse.BoatCount, err = p.BoatCountForOrg(ctx, orgID); err != nil {
		return nil, err
	}
	if pulse.TripCount, err = p.TripCountForOrg(ctx, orgID); err != nil {
		return nil, err
	}
	if pulse.DirectorCount, err = p.CountActiveUsersByRole(ctx, orgID, RoleCruiseDirector); err != nil {
		return nil, err
	}
	_ = p.QueryRow(ctx, `SELECT count(*)::int FROM leads WHERE organization_id = $1 AND status IN ('new','contacted')`, orgID).Scan(&pulse.LeadCount)
	_ = p.QueryRow(ctx, `SELECT count(*)::int FROM booking_quotes WHERE organization_id = $1 AND status IN ('sent','deposit_pending','held')`, orgID).Scan(&pulse.QuoteCount)
	if err := p.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status = 'active')::int,
			count(*) FILTER (WHERE status = 'planned' AND start_date BETWEEN $2 AND $2 + INTERVAL '90 days')::int
		FROM trips
		WHERE organization_id = $1 AND removed_from_source_at IS NULL
	`, orgID, now).Scan(&pulse.ActiveVoyages, &pulse.UpcomingVoyages); err != nil {
		return nil, err
	}
	return pulse, nil
}

func (p *Pool) cockpitInventory(ctx context.Context, orgID uuid.UUID) ([]CockpitInventory, error) {
	rows, err := p.Query(ctx, `
		SELECT bi.boat_id, b.display_name, i.name, bi.quantity_on_hand,
		       CASE WHEN bi.quantity_on_hand <= 0 THEN 'out' ELSE 'low' END
		FROM boat_inventory_items bi
		JOIN boats b ON b.id = bi.boat_id
		JOIN catalog_items i ON i.id = bi.catalog_item_id
		WHERE bi.organization_id = $1
		  AND (
		      bi.quantity_on_hand <= 0
		      OR (bi.reorder_level IS NOT NULL AND bi.quantity_on_hand <= bi.reorder_level)
		  )
		ORDER BY bi.quantity_on_hand, b.display_name, i.name
		LIMIT 8
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CockpitInventory{}
	for rows.Next() {
		var it CockpitInventory
		if err := rows.Scan(&it.BoatID, &it.BoatName, &it.ItemName, &it.Quantity, &it.Status); err != nil {
			return nil, err
		}
		it.Route = "/admin/inventory"
		out = append(out, it)
	}
	return out, rows.Err()
}

func (p *Pool) cockpitActivity(ctx context.Context, orgID, userID uuid.UUID, role string) ([]CockpitActivity, error) {
	f := AuditEventFilters{Limit: 8}
	if role != RoleOrgAdmin {
		f.AssignedTo = &userID
	}
	events, err := p.AuditEvents(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	out := make([]CockpitActivity, 0, len(events))
	for _, ev := range events {
		out = append(out, CockpitActivity{
			ID: ev.ID, Action: ev.Action, Entity: ev.EntityType, TripID: ev.TripID, CreatedAt: ev.CreatedAt,
		})
	}
	return out, nil
}

func directorPulse(voyages []CockpitVoyage, blockers []CockpitSignal, money CockpitMoney) *DirectorCockpitPulse {
	p := &DirectorCockpitPulse{AssignedVoyages: len(voyages), OpenFolios: money.OpenFolioCount, Blockers: len(blockers)}
	for _, v := range voyages {
		switch v.Status {
		case "active":
			p.ActiveVoyages++
		case "planned":
			p.UpcomingVoyages++
		}
	}
	return p
}

func cockpitCommands(role string) []CockpitCommand {
	base := []CockpitCommand{
		{Label: "Today cockpit", Route: "/admin", Kind: "core"},
		{Label: "Trips", Route: "/admin/trips", Kind: "operations"},
		{Label: "Audit", Route: "/admin/audit", Kind: "insights"},
	}
	if role != RoleOrgAdmin {
		return append(base,
			CockpitCommand{Label: "My account", Route: "/admin/account", Kind: "profile"},
		)
	}
	return append(base,
		CockpitCommand{Label: "Import trips", Route: "/admin/import", Kind: "operations"},
		CockpitCommand{Label: "Inventory", Route: "/admin/inventory", Kind: "operations"},
		CockpitCommand{Label: "Fleet", Route: "/admin/fleet", Kind: "configuration"},
		CockpitCommand{Label: "Users", Route: "/admin/users", Kind: "configuration"},
		CockpitCommand{Label: "Payments", Route: "/admin/organization/payments", Kind: "configuration"},
		CockpitCommand{Label: "Reports", Route: "/admin/reports", Kind: "insights"},
	)
}
