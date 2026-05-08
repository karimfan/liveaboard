package store

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrCabinAssignmentConflict = errors.New("store: cabin assignment conflict")
	ErrCabinLayoutInUse        = errors.New("store: cabin layout in use")
)

type BoatCabin struct {
	ID             uuid.UUID         `json:"id"`
	OrganizationID uuid.UUID         `json:"organization_id"`
	BoatID         uuid.UUID         `json:"boat_id"`
	Label          string            `json:"label"`
	Deck           *string           `json:"deck"`
	SortOrder      int               `json:"sort_order"`
	Notes          *string           `json:"notes"`
	IsActive       bool              `json:"is_active"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Berths         []*BoatCabinBerth `json:"berths,omitempty"`
}

type BoatCabinBerth struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	BoatID         uuid.UUID `json:"boat_id"`
	CabinID        uuid.UUID `json:"cabin_id"`
	BerthLabel     string    `json:"berth_label"`
	DisplayLabel   string    `json:"display_label"`
	SortOrder      int       `json:"sort_order"`
	Notes          *string   `json:"notes"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CabinLayout struct {
	BoatID           uuid.UUID    `json:"boat_id"`
	Cabins           []*BoatCabin `json:"cabins"`
	ActiveCabinCount int          `json:"active_cabin_count"`
	ActiveBerthCount int          `json:"active_berth_count"`
}

type CabinLayoutInput struct {
	Source string            `json:"source"`
	Ranges []CabinRangeInput `json:"ranges,omitempty"`
	Paste  string            `json:"paste,omitempty"`
	CSV    string            `json:"csv,omitempty"`
	Cabins []CabinInput      `json:"cabins,omitempty"`
}

type CabinRangeInput struct {
	From   int      `json:"from"`
	To     int      `json:"to"`
	Berths []string `json:"berths"`
	Deck   *string  `json:"deck"`
}

type CabinInput struct {
	Label     string       `json:"label"`
	Deck      *string      `json:"deck"`
	SortOrder int          `json:"sort_order"`
	Notes     *string      `json:"notes"`
	Berths    []BerthInput `json:"berths"`
}

type BerthInput struct {
	BerthLabel string  `json:"berth_label"`
	SortOrder  int     `json:"sort_order"`
	Notes      *string `json:"notes"`
}

type CabinLayoutPreview struct {
	Cabins   []CabinInput `json:"cabins"`
	Warnings []string     `json:"warnings"`
}

type TripCabinAssignment struct {
	ID                   uuid.UUID  `json:"id"`
	TripID               uuid.UUID  `json:"trip_id"`
	TripGuestID          uuid.UUID  `json:"trip_guest_id"`
	BoatID               uuid.UUID  `json:"boat_id"`
	BerthID              uuid.UUID  `json:"berth_id"`
	CabinLabelSnapshot   string     `json:"cabin_label"`
	BerthLabelSnapshot   string     `json:"berth_label"`
	DisplayLabelSnapshot string     `json:"display_label"`
	AssignedByUserID     *uuid.UUID `json:"assigned_by_user_id"`
	AssignedAt           time.Time  `json:"assigned_at"`
	UnassignedByUserID   *uuid.UUID `json:"unassigned_by_user_id"`
	UnassignedAt         *time.Time `json:"unassigned_at"`
	Notes                *string    `json:"notes"`
}

type TripCabinBoard struct {
	TripID           uuid.UUID         `json:"trip_id"`
	BoatID           uuid.UUID         `json:"boat_id"`
	Cabins           []*TripCabinCabin `json:"cabins"`
	UnassignedGuests []*TripCabinGuest `json:"unassigned_guests"`
}

type TripCabinCabin struct {
	ID        uuid.UUID         `json:"id"`
	Label     string            `json:"label"`
	Deck      *string           `json:"deck"`
	SortOrder int               `json:"sort_order"`
	Berths    []*TripCabinBerth `json:"berths"`
}

type TripCabinBerth struct {
	ID           uuid.UUID       `json:"id"`
	CabinID      uuid.UUID       `json:"cabin_id"`
	BerthLabel   string          `json:"berth_label"`
	DisplayLabel string          `json:"display_label"`
	SortOrder    int             `json:"sort_order"`
	Guest        *TripCabinGuest `json:"guest"`
	AssignmentID *uuid.UUID      `json:"assignment_id"`
}

type TripCabinGuest struct {
	ID       uuid.UUID `json:"id"`
	FullName string    `json:"full_name"`
	Email    string    `json:"email"`
	Status   string    `json:"status"`
}

const cabinColumns = `id, organization_id, boat_id, label, deck, sort_order, notes, is_active, created_at, updated_at`
const berthColumns = `id, organization_id, boat_id, cabin_id, berth_label, display_label, sort_order, notes, is_active, created_at, updated_at`

func scanCabin(row interface{ Scan(dest ...any) error }, c *BoatCabin) error {
	return row.Scan(&c.ID, &c.OrganizationID, &c.BoatID, &c.Label, &c.Deck, &c.SortOrder, &c.Notes, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
}

func scanBerth(row interface{ Scan(dest ...any) error }, b *BoatCabinBerth) error {
	return row.Scan(&b.ID, &b.OrganizationID, &b.BoatID, &b.CabinID, &b.BerthLabel, &b.DisplayLabel, &b.SortOrder, &b.Notes, &b.IsActive, &b.CreatedAt, &b.UpdatedAt)
}

func (p *Pool) PreviewCabinLayout(ctx context.Context, orgID, boatID uuid.UUID, input CabinLayoutInput) (*CabinLayoutPreview, error) {
	if _, err := p.BoatByID(ctx, orgID, boatID); err != nil {
		return nil, err
	}
	return NormalizeCabinLayoutInput(input)
}

func NormalizeCabinLayoutInput(input CabinLayoutInput) (*CabinLayoutPreview, error) {
	var cabins []CabinInput
	switch input.Source {
	case "", "cabins":
		cabins = input.Cabins
	case "ranges":
		cabins = cabinsFromRanges(input.Ranges)
	case "paste":
		parsed, err := cabinsFromPaste(input.Paste)
		if err != nil {
			return nil, err
		}
		cabins = parsed
	case "csv":
		parsed, err := cabinsFromCSV(input.CSV)
		if err != nil {
			return nil, err
		}
		cabins = parsed
	default:
		return nil, fmt.Errorf("%w: source", ErrInvalidInput)
	}
	cabins, err := normalizeCabins(cabins)
	if err != nil {
		return nil, err
	}
	return &CabinLayoutPreview{Cabins: cabins, Warnings: []string{}}, nil
}

func cabinsFromRanges(ranges []CabinRangeInput) []CabinInput {
	var cabins []CabinInput
	for _, r := range ranges {
		if r.To < r.From {
			continue
		}
		for n := r.From; n <= r.To; n++ {
			c := CabinInput{Label: strconv.Itoa(n), Deck: r.Deck, SortOrder: n * 10}
			for i, label := range r.Berths {
				c.Berths = append(c.Berths, BerthInput{BerthLabel: label, SortOrder: n*10 + i})
			}
			cabins = append(cabins, c)
		}
	}
	return cabins
}

func cabinsFromPaste(raw string) ([]CabinInput, error) {
	var cabins []CabinInput
	for i, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, ",") {
			return nil, fmt.Errorf("%w: row %d must use commas", ErrInvalidInput, i+1)
		}
		parts := splitCSVish(line)
		if len(parts) < 2 {
			return nil, fmt.Errorf("%w: row %d needs a cabin and berth", ErrInvalidInput, i+1)
		}
		c := CabinInput{Label: parts[0], SortOrder: (i + 1) * 10}
		for j, berth := range parts[1:] {
			c.Berths = append(c.Berths, BerthInput{BerthLabel: berth, SortOrder: (i+1)*10 + j})
		}
		cabins = append(cabins, c)
	}
	return cabins, nil
}

func cabinsFromCSV(raw string) ([]CabinInput, error) {
	r := csv.NewReader(strings.NewReader(raw))
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("%w: csv header", ErrInvalidInput)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	cabinIdx, okC := idx["cabin_label"]
	berthIdx, okB := idx["berth_label"]
	if !okC || !okB {
		return nil, fmt.Errorf("%w: csv requires cabin_label and berth_label", ErrInvalidInput)
	}
	byLabel := map[string]*CabinInput{}
	order := []string{}
	rowNum := 1
	for {
		rowNum++
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: csv row %d", ErrInvalidInput, rowNum)
		}
		cabinLabel := csvVal(row, cabinIdx)
		berthLabel := csvVal(row, berthIdx)
		if cabinLabel == "" || berthLabel == "" {
			return nil, fmt.Errorf("%w: csv row %d needs cabin_label and berth_label", ErrInvalidInput, rowNum)
		}
		c, ok := byLabel[cabinLabel]
		if !ok {
			c = &CabinInput{Label: cabinLabel, SortOrder: len(order) * 10}
			if i, ok := idx["deck"]; ok {
				c.Deck = nullableString(csvVal(row, i))
			}
			if i, ok := idx["notes"]; ok {
				c.Notes = nullableString(csvVal(row, i))
			}
			order = append(order, cabinLabel)
			byLabel[cabinLabel] = c
		}
		sortOrder := len(c.Berths)
		if i, ok := idx["sort_order"]; ok && csvVal(row, i) != "" {
			n, err := strconv.Atoi(csvVal(row, i))
			if err != nil {
				return nil, fmt.Errorf("%w: csv row %d sort_order", ErrInvalidInput, rowNum)
			}
			sortOrder = n
		}
		c.Berths = append(c.Berths, BerthInput{BerthLabel: berthLabel, SortOrder: sortOrder})
	}
	cabins := make([]CabinInput, 0, len(order))
	for _, label := range order {
		cabins = append(cabins, *byLabel[label])
	}
	return cabins, nil
}

func splitCSVish(line string) []string {
	raw := strings.Split(line, ",")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func csvVal(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func normalizeCabins(cabins []CabinInput) ([]CabinInput, error) {
	if len(cabins) == 0 {
		return nil, fmt.Errorf("%w: cabins", ErrInvalidInput)
	}
	seenCabins := map[string]bool{}
	for i := range cabins {
		cabins[i].Label = strings.TrimSpace(cabins[i].Label)
		if cabins[i].Label == "" {
			return nil, fmt.Errorf("%w: cabin label", ErrInvalidInput)
		}
		key := strings.ToLower(cabins[i].Label)
		if seenCabins[key] {
			return nil, fmt.Errorf("%w: duplicate cabin %s", ErrInvalidInput, cabins[i].Label)
		}
		seenCabins[key] = true
		if len(cabins[i].Berths) == 0 {
			return nil, fmt.Errorf("%w: cabin %s has no berths", ErrInvalidInput, cabins[i].Label)
		}
		seenBerths := map[string]bool{}
		for j := range cabins[i].Berths {
			cabins[i].Berths[j].BerthLabel = strings.TrimSpace(cabins[i].Berths[j].BerthLabel)
			if cabins[i].Berths[j].BerthLabel == "" {
				return nil, fmt.Errorf("%w: berth label", ErrInvalidInput)
			}
			if len(cabins[i].Berths[j].BerthLabel) > 40 {
				return nil, fmt.Errorf("%w: berth label too long", ErrInvalidInput)
			}
			bkey := strings.ToLower(cabins[i].Berths[j].BerthLabel)
			if seenBerths[bkey] {
				return nil, fmt.Errorf("%w: duplicate berth %s", ErrInvalidInput, cabins[i].Berths[j].BerthLabel)
			}
			seenBerths[bkey] = true
		}
	}
	return cabins, nil
}

func (p *Pool) BoatCabinLayout(ctx context.Context, orgID, boatID uuid.UUID) (*CabinLayout, error) {
	if _, err := p.BoatByID(ctx, orgID, boatID); err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, `SELECT `+cabinColumns+` FROM boat_cabins WHERE organization_id = $1 AND boat_id = $2 ORDER BY sort_order, label`, orgID, boatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	layout := &CabinLayout{BoatID: boatID}
	cabinsByID := map[uuid.UUID]*BoatCabin{}
	for rows.Next() {
		c := &BoatCabin{}
		if err := scanCabin(rows, c); err != nil {
			return nil, err
		}
		if c.IsActive {
			layout.ActiveCabinCount++
		}
		layout.Cabins = append(layout.Cabins, c)
		cabinsByID[c.ID] = c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	brows, err := p.Query(ctx, `SELECT `+berthColumns+` FROM boat_cabin_berths WHERE organization_id = $1 AND boat_id = $2 ORDER BY sort_order, display_label`, orgID, boatID)
	if err != nil {
		return nil, err
	}
	defer brows.Close()
	for brows.Next() {
		b := &BoatCabinBerth{}
		if err := scanBerth(brows, b); err != nil {
			return nil, err
		}
		if b.IsActive {
			layout.ActiveBerthCount++
		}
		if c := cabinsByID[b.CabinID]; c != nil {
			c.Berths = append(c.Berths, b)
		}
	}
	return layout, brows.Err()
}

func (p *Pool) ReplaceBoatCabinLayout(ctx context.Context, orgID, boatID, actorID uuid.UUID, input CabinLayoutInput) (*CabinLayout, error) {
	preview, err := p.PreviewCabinLayout(ctx, orgID, boatID, input)
	if err != nil {
		return nil, err
	}
	var inUse bool
	if err := p.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM trip_cabin_assignments a
			JOIN boat_cabin_berths b ON b.id = a.berth_id
			WHERE b.organization_id = $1 AND b.boat_id = $2 AND a.unassigned_at IS NULL
		)
	`, orgID, boatID).Scan(&inUse); err != nil {
		return nil, err
	}
	if inUse {
		return nil, ErrCabinLayoutInUse
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM boat_cabin_berths WHERE organization_id = $1 AND boat_id = $2`, orgID, boatID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM boat_cabins WHERE organization_id = $1 AND boat_id = $2`, orgID, boatID); err != nil {
		return nil, err
	}
	for i, c := range preview.Cabins {
		if c.SortOrder == 0 {
			c.SortOrder = i * 10
		}
		cabin := &BoatCabin{}
		if err := scanCabin(tx.QueryRow(ctx, `
			INSERT INTO boat_cabins (organization_id, boat_id, label, deck, sort_order, notes)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING `+cabinColumns,
			orgID, boatID, c.Label, c.Deck, c.SortOrder, c.Notes,
		), cabin); err != nil {
			return nil, err
		}
		for j, b := range c.Berths {
			if b.SortOrder == 0 {
				b.SortOrder = c.SortOrder + j
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO boat_cabin_berths (organization_id, boat_id, cabin_id, berth_label, display_label, sort_order, notes)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, orgID, boatID, cabin.ID, b.BerthLabel, displayLabel(c.Label, b.BerthLabel), b.SortOrder, b.Notes); err != nil {
				return nil, err
			}
		}
	}
	_ = actorID
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return p.BoatCabinLayout(ctx, orgID, boatID)
}

func displayLabel(cabin, berth string) string {
	if len([]rune(berth)) == 1 {
		return cabin + berth
	}
	return cabin + " " + berth
}

func (p *Pool) UpdateBoatCabin(ctx context.Context, orgID, boatID, cabinID, actorID uuid.UUID, in CabinInput) (*BoatCabin, error) {
	label := strings.TrimSpace(in.Label)
	if label == "" {
		return nil, fmt.Errorf("%w: label", ErrInvalidInput)
	}
	c := &BoatCabin{}
	err := scanCabin(p.QueryRow(ctx, `
		UPDATE boat_cabins
		SET label = $4, deck = $5, sort_order = $6, notes = $7, updated_at = now()
		WHERE organization_id = $1 AND boat_id = $2 AND id = $3
		RETURNING `+cabinColumns,
		orgID, boatID, cabinID, label, in.Deck, in.SortOrder, in.Notes,
	), c)
	_ = actorID
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return c, err
}

func (p *Pool) UpdateBoatBerth(ctx context.Context, orgID, boatID, berthID, actorID uuid.UUID, in BerthInput) (*BoatCabinBerth, error) {
	label := strings.TrimSpace(in.BerthLabel)
	if label == "" {
		return nil, fmt.Errorf("%w: berth_label", ErrInvalidInput)
	}
	b := &BoatCabinBerth{}
	err := scanBerth(p.QueryRow(ctx, `
		UPDATE boat_cabin_berths b
		SET berth_label = $4,
		    display_label = concat(c.label, CASE WHEN length($4) = 1 THEN '' ELSE ' ' END, $4),
		    sort_order = $5,
		    notes = $6,
		    updated_at = now()
		FROM boat_cabins c
		WHERE b.cabin_id = c.id AND b.organization_id = $1 AND b.boat_id = $2 AND b.id = $3
		RETURNING b.`+strings.ReplaceAll(berthColumns, ", ", ", b."),
		orgID, boatID, berthID, label, in.SortOrder, in.Notes,
	), b)
	_ = actorID
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return b, err
}

func (p *Pool) DeactivateBoatCabin(ctx context.Context, orgID, boatID, cabinID, actorID uuid.UUID) error {
	var inUse bool
	if err := p.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM trip_cabin_assignments a
			JOIN boat_cabin_berths b ON b.id = a.berth_id
			WHERE b.cabin_id = $1 AND b.organization_id = $2 AND b.boat_id = $3 AND a.unassigned_at IS NULL
		)
	`, cabinID, orgID, boatID).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return ErrCabinLayoutInUse
	}
	tag, err := p.Exec(ctx, `UPDATE boat_cabins SET is_active = false, updated_at = now() WHERE organization_id = $1 AND boat_id = $2 AND id = $3`, orgID, boatID, cabinID)
	_ = actorID
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = p.Exec(ctx, `UPDATE boat_cabin_berths SET is_active = false, updated_at = now() WHERE organization_id = $1 AND boat_id = $2 AND cabin_id = $3`, orgID, boatID, cabinID)
	return err
}

func (p *Pool) DeactivateBoatBerth(ctx context.Context, orgID, boatID, berthID, actorID uuid.UUID) error {
	var inUse bool
	if err := p.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM trip_cabin_assignments WHERE berth_id = $1 AND unassigned_at IS NULL)`, berthID).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return ErrCabinLayoutInUse
	}
	tag, err := p.Exec(ctx, `UPDATE boat_cabin_berths SET is_active = false, updated_at = now() WHERE organization_id = $1 AND boat_id = $2 AND id = $3`, orgID, boatID, berthID)
	_ = actorID
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Pool) AssignTripGuestBerth(ctx context.Context, orgID, tripID, tripGuestID, berthID, actorID uuid.UUID, notes *string) (*TripCabinAssignment, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	a, err := assignTripGuestBerthTx(ctx, tx, orgID, tripID, tripGuestID, berthID, actorID, notes)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

func assignTripGuestBerthTx(ctx context.Context, tx pgx.Tx, orgID, tripID, tripGuestID, berthID, actorID uuid.UUID, notes *string) (*TripCabinAssignment, error) {
	var tripBoatID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT boat_id FROM trips WHERE organization_id = $1 AND id = $2`, orgID, tripID).Scan(&tripBoatID); err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var guestRevoked *time.Time
	if err := tx.QueryRow(ctx, `SELECT revoked_at FROM trip_guests WHERE organization_id = $1 AND trip_id = $2 AND id = $3`, orgID, tripID, tripGuestID).Scan(&guestRevoked); err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if guestRevoked != nil {
		return nil, fmt.Errorf("%w: revoked guest", ErrInvalidInput)
	}
	var cabinLabel, berthLabel, display string
	var berthBoatID uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT b.boat_id, c.label, b.berth_label, b.display_label
		FROM boat_cabin_berths b
		JOIN boat_cabins c ON c.id = b.cabin_id
		WHERE b.organization_id = $1 AND b.id = $2 AND b.is_active AND c.is_active
	`, orgID, berthID).Scan(&berthBoatID, &cabinLabel, &berthLabel, &display); err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if berthBoatID != tripBoatID {
		return nil, fmt.Errorf("%w: berth boat mismatch", ErrInvalidInput)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE trip_cabin_assignments
		SET unassigned_at = now(), unassigned_by_user_id = $5, updated_at = now()
		WHERE organization_id = $1 AND trip_id = $2 AND trip_guest_id = $3 AND unassigned_at IS NULL AND berth_id <> $4
	`, orgID, tripID, tripGuestID, berthID, actorID); err != nil {
		return nil, err
	}
	a := &TripCabinAssignment{}
	err := tx.QueryRow(ctx, `
		INSERT INTO trip_cabin_assignments (
			organization_id, trip_id, trip_guest_id, boat_id, berth_id,
			cabin_label_snapshot, berth_label_snapshot, display_label_snapshot,
			assigned_by_user_id, notes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (trip_guest_id) WHERE unassigned_at IS NULL DO UPDATE SET
			berth_id = EXCLUDED.berth_id,
			cabin_label_snapshot = EXCLUDED.cabin_label_snapshot,
			berth_label_snapshot = EXCLUDED.berth_label_snapshot,
			display_label_snapshot = EXCLUDED.display_label_snapshot,
			assigned_by_user_id = EXCLUDED.assigned_by_user_id,
			assigned_at = now(),
			notes = EXCLUDED.notes,
			updated_at = now()
		RETURNING id, trip_id, trip_guest_id, boat_id, berth_id, cabin_label_snapshot, berth_label_snapshot,
			display_label_snapshot, assigned_by_user_id, assigned_at, unassigned_by_user_id, unassigned_at, notes
	`, orgID, tripID, tripGuestID, tripBoatID, berthID, cabinLabel, berthLabel, display, actorID, notes).Scan(
		&a.ID, &a.TripID, &a.TripGuestID, &a.BoatID, &a.BerthID, &a.CabinLabelSnapshot, &a.BerthLabelSnapshot,
		&a.DisplayLabelSnapshot, &a.AssignedByUserID, &a.AssignedAt, &a.UnassignedByUserID, &a.UnassignedAt, &a.Notes,
	)
	if isUniqueViolation(err, "trip_cabin_assignments_one_active_berth_per_trip_idx") {
		return nil, ErrCabinAssignmentConflict
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (p *Pool) UnassignTripGuestBerth(ctx context.Context, orgID, tripID, tripGuestID, actorID uuid.UUID) error {
	tag, err := p.Exec(ctx, `
		UPDATE trip_cabin_assignments
		SET unassigned_at = now(), unassigned_by_user_id = $4, updated_at = now()
		WHERE organization_id = $1 AND trip_id = $2 AND trip_guest_id = $3 AND unassigned_at IS NULL
	`, orgID, tripID, tripGuestID, actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	return nil
}

func (p *Pool) TripCabinBoard(ctx context.Context, orgID, tripID uuid.UUID, now time.Time) (*TripCabinBoard, error) {
	trip, err := p.TripByID(ctx, orgID, tripID)
	if err != nil {
		return nil, err
	}
	layout, err := p.BoatCabinLayout(ctx, orgID, trip.BoatID)
	if err != nil {
		return nil, err
	}
	board := &TripCabinBoard{TripID: tripID, BoatID: trip.BoatID}
	berthsByID := map[uuid.UUID]*TripCabinBerth{}
	for _, c := range layout.Cabins {
		if !c.IsActive {
			continue
		}
		tc := &TripCabinCabin{ID: c.ID, Label: c.Label, Deck: c.Deck, SortOrder: c.SortOrder}
		for _, b := range c.Berths {
			if !b.IsActive {
				continue
			}
			tb := &TripCabinBerth{ID: b.ID, CabinID: c.ID, BerthLabel: b.BerthLabel, DisplayLabel: b.DisplayLabel, SortOrder: b.SortOrder}
			tc.Berths = append(tc.Berths, tb)
			berthsByID[b.ID] = tb
		}
		board.Cabins = append(board.Cabins, tc)
	}
	rows, err := p.Query(ctx, `
		SELECT g.id, g.full_name, g.email, g.revoked_at, a.id, a.berth_id
		FROM trip_guests g
		LEFT JOIN trip_cabin_assignments a ON a.trip_guest_id = g.id AND a.unassigned_at IS NULL
		WHERE g.organization_id = $1 AND g.trip_id = $2 AND g.revoked_at IS NULL
		ORDER BY lower(g.full_name), g.created_at
	`, orgID, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var g TripCabinGuest
		var revoked *time.Time
		var assignmentID, berthID *uuid.UUID
		if err := rows.Scan(&g.ID, &g.FullName, &g.Email, &revoked, &assignmentID, &berthID); err != nil {
			return nil, err
		}
		g.Status = "active"
		if berthID != nil {
			if b := berthsByID[*berthID]; b != nil {
				copyG := g
				b.Guest = &copyG
				b.AssignmentID = assignmentID
				continue
			}
		}
		board.UnassignedGuests = append(board.UnassignedGuests, &g)
	}
	sort.Slice(board.Cabins, func(i, j int) bool { return board.Cabins[i].SortOrder < board.Cabins[j].SortOrder })
	_ = now
	return board, rows.Err()
}

func (p *Pool) BoatActiveBerthCount(ctx context.Context, orgID, boatID uuid.UUID) (int, error) {
	var n int
	err := p.QueryRow(ctx, `SELECT count(*) FROM boat_cabin_berths WHERE organization_id = $1 AND boat_id = $2 AND is_active`, orgID, boatID).Scan(&n)
	return n, err
}

type BoatLayoutSummary struct {
	BoatID           uuid.UUID `json:"boat_id"`
	BoatName         string    `json:"boat_name"`
	ActiveBerthCount int       `json:"active_berth_count"`
}

func (p *Pool) UnconfiguredBoats(ctx context.Context, orgID uuid.UUID) ([]BoatLayoutSummary, error) {
	rows, err := p.Query(ctx, `
		SELECT b.id, b.display_name, count(bb.id) FILTER (WHERE bb.is_active)::int
		FROM boats b
		LEFT JOIN boat_cabin_berths bb ON bb.boat_id = b.id AND bb.organization_id = b.organization_id
		WHERE b.organization_id = $1
		GROUP BY b.id, b.display_name
		HAVING count(bb.id) FILTER (WHERE bb.is_active) = 0
		ORDER BY b.display_name
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BoatLayoutSummary
	for rows.Next() {
		var s BoatLayoutSummary
		if err := rows.Scan(&s.BoatID, &s.BoatName, &s.ActiveBerthCount); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
