package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrFolioExists               = errors.New("store: folio already exists")
	ErrFolioClosed               = errors.New("store: folio is closed")
	ErrTripNotActive             = errors.New("store: trip is not active")
	errDuplicateFolioLineRequest = errors.New("store: duplicate folio line request")
)

const (
	FolioStatusOpen        = "open"
	FolioStatusClosed      = "closed"
	FolioLineCatalogItem   = "catalog_item"
	FolioLineCrewTip       = "crew_tip"
	FolioEmailStatusSent   = "sent"
	FolioEmailStatusFailed = "failed"
)

type GuestFolio struct {
	ID                   uuid.UUID
	OrganizationID       uuid.UUID
	TripID               uuid.UUID
	TripGuestID          uuid.UUID
	GuestUserID          *uuid.UUID
	Status               string
	OpenedByUserID       uuid.UUID
	ClosedByUserID       *uuid.UUID
	ClosedAt             *time.Time
	SubtotalUSDCents     int64
	CardFeeUSDCents      int64
	TotalUSDCents        int64
	SettlementCurrency   *string
	SettlementTotalMinor *int64
	CurrencyExponent     *int
	RateProvider         *string
	RateNumerator        *int64
	RateDenominator      *int64
	RateAsOf             *time.Time
	PaymentMethod        *string
	CardFeeBasisPoints   int
	EmailSendStatus      string
	EmailLastSentAt      *time.Time
	EmailLastError       *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Lines                []*GuestFolioLine
}

type GuestFolioLine struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	FolioID           uuid.UUID
	TripGuestID       uuid.UUID
	CatalogItemID     *uuid.UUID
	LineType          string
	ItemName          string
	Quantity          int
	UnitPriceUSDCents int64
	LineTotalUSDCents int64
	StockMode         string
	SortOrder         int
	CreatedByUserID   *uuid.UUID
	StockPostedAt     *time.Time
	VoidedAt          *time.Time
	VoidedByUserID    *uuid.UUID
	VoidReason        *string
	ClientRequestID   *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type FolioWarning struct {
	Code           string
	Message        string
	CatalogItemID  *uuid.UUID
	QuantityOnHand *int
}

type GuestFolioView struct {
	Folio            *GuestFolio
	OrganizationName string
	BoatName         string
	TripItinerary    string
	TripStartDate    time.Time
	TripEndDate      time.Time
	GuestFullName    string
	GuestEmail       string
	Warnings         []FolioWarning
}

type TripLedgerGuest struct {
	TripGuestID      uuid.UUID
	FullName         string
	Email            string
	FolioID          *uuid.UUID
	FolioStatus      *string
	LineCount        int
	SubtotalUSDCents int64
}

type TripLedgerLine struct {
	ID                uuid.UUID
	TripGuestID       uuid.UUID
	GuestFullName     string
	ItemName          string
	Quantity          int
	LineTotalUSDCents int64
	StockMode         string
	CreatedAt         time.Time
}

type TripLedgerView struct {
	Trip      *Trip
	Guests    []TripLedgerGuest
	Catalog   []*CatalogItem
	Inventory []*BoatInventoryItem
	Recent    []TripLedgerLine
}

type AddFolioLineInput struct {
	ActorUserID     uuid.UUID
	LineType        string
	CatalogItemID   uuid.UUID
	Quantity        int
	TipUSDCents     int64
	ClientRequestID string
}

type UpdateFolioLineInput struct {
	ActorUserID uuid.UUID
	Quantity    *int
	TipUSDCents *int64
}

type CloseGuestFolioInput struct {
	ActorUserID        uuid.UUID
	PaymentMethod      string
	SettlementCurrency string
	Now                time.Time
}

const guestFolioColumns = `id, organization_id, trip_id, trip_guest_id, guest_user_id, status,
	opened_by_user_id, closed_by_user_id, closed_at, subtotal_usd_cents, card_fee_usd_cents,
	total_usd_cents, settlement_currency, settlement_total_minor, currency_exponent,
	rate_provider, rate_numerator, rate_denominator, rate_as_of, payment_method,
	card_fee_basis_points, email_send_status, email_last_sent_at, email_last_error,
	created_at, updated_at`

const guestFolioLineColumns = `id, organization_id, folio_id, catalog_item_id, line_type,
	item_name, quantity, unit_price_usd_cents, line_total_usd_cents, stock_mode,
	sort_order, created_by_user_id, trip_guest_id, stock_posted_at, voided_at,
	voided_by_user_id, void_reason, client_request_id, created_at, updated_at`

func scanGuestFolio(row interface{ Scan(dest ...any) error }, f *GuestFolio) error {
	return row.Scan(&f.ID, &f.OrganizationID, &f.TripID, &f.TripGuestID, &f.GuestUserID, &f.Status,
		&f.OpenedByUserID, &f.ClosedByUserID, &f.ClosedAt, &f.SubtotalUSDCents, &f.CardFeeUSDCents,
		&f.TotalUSDCents, &f.SettlementCurrency, &f.SettlementTotalMinor, &f.CurrencyExponent,
		&f.RateProvider, &f.RateNumerator, &f.RateDenominator, &f.RateAsOf, &f.PaymentMethod,
		&f.CardFeeBasisPoints, &f.EmailSendStatus, &f.EmailLastSentAt, &f.EmailLastError,
		&f.CreatedAt, &f.UpdatedAt)
}

func scanGuestFolioLine(row interface{ Scan(dest ...any) error }, l *GuestFolioLine) error {
	return row.Scan(&l.ID, &l.OrganizationID, &l.FolioID, &l.CatalogItemID, &l.LineType,
		&l.ItemName, &l.Quantity, &l.UnitPriceUSDCents, &l.LineTotalUSDCents, &l.StockMode,
		&l.SortOrder, &l.CreatedByUserID, &l.TripGuestID, &l.StockPostedAt, &l.VoidedAt,
		&l.VoidedByUserID, &l.VoidReason, &l.ClientRequestID, &l.CreatedAt, &l.UpdatedAt)
}

func (p *Pool) OpenGuestFolio(ctx context.Context, orgID, tripID, tripGuestID, actorID uuid.UUID) (*GuestFolioView, error) {
	g, err := p.tripGuestForFolio(ctx, orgID, tripID, tripGuestID)
	if err != nil {
		return nil, err
	}
	f := &GuestFolio{}
	err = scanGuestFolio(p.QueryRow(ctx, `
		INSERT INTO guest_folios (organization_id, trip_id, trip_guest_id, guest_user_id, status, opened_by_user_id)
		VALUES ($1,$2,$3,$4,'open',$5)
		RETURNING `+guestFolioColumns,
		orgID, tripID, tripGuestID, g.GuestUserID, actorID), f)
	if err != nil {
		// Unique violation is surfaced by pgx as a generic error string
		// through PostgreSQL; returning a domain error keeps handlers simple.
		if isUniqueViolation(err, "guest_folios_one_per_trip_guest_idx") {
			return nil, ErrFolioExists
		}
		return nil, err
	}
	f.Lines = []*GuestFolioLine{}
	return p.guestFolioView(ctx, f)
}

func (p *Pool) GuestFolioByTripGuest(ctx context.Context, orgID, tripID, tripGuestID uuid.UUID) (*GuestFolioView, error) {
	f := &GuestFolio{}
	err := scanGuestFolio(p.QueryRow(ctx, `
		SELECT `+guestFolioColumns+`
		FROM guest_folios
		WHERE organization_id = $1 AND trip_id = $2 AND trip_guest_id = $3
	`, orgID, tripID, tripGuestID), f)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	lines, err := p.GuestFolioLines(ctx, orgID, f.ID)
	if err != nil {
		return nil, err
	}
	f.Lines = lines
	return p.guestFolioView(ctx, f)
}

func (p *Pool) GuestFolioLines(ctx context.Context, orgID, folioID uuid.UUID) ([]*GuestFolioLine, error) {
	rows, err := p.Query(ctx, `
		SELECT `+guestFolioLineColumns+`
		FROM guest_folio_lines
		WHERE organization_id = $1 AND folio_id = $2 AND voided_at IS NULL
		ORDER BY sort_order, created_at
	`, orgID, folioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*GuestFolioLine{}
	for rows.Next() {
		l := &GuestFolioLine{}
		if err := scanGuestFolioLine(rows, l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (p *Pool) TripConsumptionLedger(ctx context.Context, orgID, tripID uuid.UUID) (*TripLedgerView, error) {
	trip, err := p.TripByID(ctx, orgID, tripID)
	if err != nil {
		return nil, err
	}
	catalog, err := p.CatalogItems(ctx, orgID)
	if err != nil {
		return nil, err
	}
	inventory, err := p.BoatInventory(ctx, orgID, trip.BoatID)
	if err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, `
		SELECT g.id, g.full_name, g.email, f.id, f.status,
		       count(l.id)::int,
		       COALESCE(sum(l.line_total_usd_cents), 0)::bigint
		FROM trip_guests g
		LEFT JOIN guest_folios f ON f.trip_guest_id = g.id AND f.organization_id = g.organization_id
		LEFT JOIN guest_folio_lines l ON l.folio_id = f.id AND l.voided_at IS NULL
		WHERE g.organization_id = $1 AND g.trip_id = $2 AND g.revoked_at IS NULL
		GROUP BY g.id, g.full_name, g.email, f.id, f.status
		ORDER BY lower(g.full_name), g.created_at
	`, orgID, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	guests := []TripLedgerGuest{}
	for rows.Next() {
		var g TripLedgerGuest
		if err := rows.Scan(&g.TripGuestID, &g.FullName, &g.Email, &g.FolioID, &g.FolioStatus, &g.LineCount, &g.SubtotalUSDCents); err != nil {
			return nil, err
		}
		guests = append(guests, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	recentRows, err := p.Query(ctx, `
		SELECT l.id, l.trip_guest_id, g.full_name, l.item_name, l.quantity,
		       l.line_total_usd_cents, l.stock_mode, l.created_at
		FROM guest_folio_lines l
		JOIN guest_folios f ON f.id = l.folio_id AND f.organization_id = l.organization_id
		JOIN trip_guests g ON g.id = l.trip_guest_id AND g.organization_id = l.organization_id
		WHERE l.organization_id = $1 AND f.trip_id = $2 AND l.voided_at IS NULL
		ORDER BY l.created_at DESC
		LIMIT 50
	`, orgID, tripID)
	if err != nil {
		return nil, err
	}
	defer recentRows.Close()
	recent := []TripLedgerLine{}
	for recentRows.Next() {
		var line TripLedgerLine
		if err := recentRows.Scan(&line.ID, &line.TripGuestID, &line.GuestFullName, &line.ItemName, &line.Quantity, &line.LineTotalUSDCents, &line.StockMode, &line.CreatedAt); err != nil {
			return nil, err
		}
		recent = append(recent, line)
	}
	if err := recentRows.Err(); err != nil {
		return nil, err
	}
	return &TripLedgerView{Trip: trip, Guests: guests, Catalog: catalog, Inventory: inventory, Recent: recent}, nil
}

func (p *Pool) AddGuestFolioLine(ctx context.Context, orgID, tripID, tripGuestID uuid.UUID, in AddFolioLineInput) (*GuestFolioView, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	f, boatID, err := p.folioForActiveMutationTx(ctx, tx, orgID, tripID, tripGuestID, in.ActorUserID, true)
	if err != nil {
		return nil, err
	}
	var warnings []FolioWarning
	lineType := in.LineType
	if lineType == "" {
		lineType = FolioLineCatalogItem
	}
	var line *GuestFolioLine
	switch lineType {
	case FolioLineCatalogItem:
		if in.Quantity <= 0 {
			return nil, errors.New("quantity must be positive")
		}
		if len(in.ClientRequestID) > 64 {
			return nil, errors.New("client_request_id must be 64 characters or fewer")
		}
		if in.ClientRequestID != "" {
			existing, err := folioLineByClientRequestTx(ctx, tx, orgID, tripGuestID, in.ClientRequestID)
			if err == nil {
				if err := tx.Commit(ctx); err != nil {
					return nil, err
				}
				view, err := p.GuestFolioByTripGuest(ctx, orgID, tripID, tripGuestID)
				if err == nil {
					view.Warnings = warnings
					_ = existing
				}
				return view, err
			}
			if err != ErrNotFound {
				return nil, err
			}
		}
		item, err := catalogItemByIDTx(ctx, tx, orgID, in.CatalogItemID)
		if err != nil {
			return nil, err
		}
		if !item.IsActive || item.ArchivedAt != nil {
			return nil, ErrNotFound
		}
		line, err = insertFolioLineTx(ctx, tx, f, &in.ActorUserID, &in.CatalogItemID, FolioLineCatalogItem, item.Name, in.Quantity, item.PriceUSDCents, item.StockMode, in.ClientRequestID)
		if err != nil {
			if errors.Is(err, errDuplicateFolioLineRequest) {
				if err := tx.Commit(ctx); err != nil {
					return nil, err
				}
				return p.GuestFolioByTripGuest(ctx, orgID, tripID, tripGuestID)
			}
			return nil, err
		}
		if item.StockMode == StockModeCounted {
			after, err := postFolioStockMovementTx(ctx, tx, orgID, boatID, item.ID, line.ID, in.ActorUserID, MovementFolioCharge, -in.Quantity)
			if err != nil {
				return nil, err
			}
			if _, err := tx.Exec(ctx, `UPDATE guest_folio_lines SET stock_posted_at = now(), updated_at = now() WHERE id = $1`, line.ID); err != nil {
				return nil, err
			}
			if after < 0 {
				warnings = append(warnings, negativeStockWarning(item.ID, item.Name, after))
			}
		}
	case FolioLineCrewTip:
		if in.TipUSDCents < 0 {
			return nil, errors.New("tip_usd_cents must be non-negative")
		}
		if _, err := upsertTipLineTx(ctx, tx, f, &in.ActorUserID, in.TipUSDCents); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported line_type")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	view, err := p.GuestFolioByTripGuest(ctx, orgID, tripID, tripGuestID)
	if err == nil {
		view.Warnings = warnings
	}
	return view, err
}

func (p *Pool) UpdateGuestFolioLine(ctx context.Context, orgID, tripID, tripGuestID, lineID uuid.UUID, in UpdateFolioLineInput) (*GuestFolioView, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	f, boatID, err := p.folioForActiveMutationTx(ctx, tx, orgID, tripID, tripGuestID, in.ActorUserID, false)
	if err != nil {
		return nil, err
	}
	line := &GuestFolioLine{}
	err = scanGuestFolioLine(tx.QueryRow(ctx, `
		SELECT `+guestFolioLineColumns+`
		FROM guest_folio_lines
		WHERE organization_id = $1 AND folio_id = $2 AND id = $3 AND voided_at IS NULL
		FOR UPDATE
	`, orgID, f.ID, lineID), line)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var warnings []FolioWarning
	switch line.LineType {
	case FolioLineCatalogItem:
		if in.Quantity == nil || *in.Quantity <= 0 {
			return nil, errors.New("quantity must be positive")
		}
		delta := *in.Quantity - line.Quantity
		if delta != 0 && line.StockMode == StockModeCounted && line.StockPostedAt != nil {
			if line.CatalogItemID == nil {
				return nil, errors.New("counted folio line missing catalog item")
			}
			movementType := MovementFolioCharge
			stockDelta := -delta
			if delta < 0 {
				movementType = MovementFolioVoid
				stockDelta = -delta
			}
			after, err := postFolioStockMovementTx(ctx, tx, orgID, boatID, *line.CatalogItemID, line.ID, in.ActorUserID, movementType, stockDelta)
			if err != nil {
				return nil, err
			}
			if after < 0 {
				warnings = append(warnings, negativeStockWarning(*line.CatalogItemID, line.ItemName, after))
			}
		}
		_, err = tx.Exec(ctx, `
			UPDATE guest_folio_lines
			SET quantity = $4, line_total_usd_cents = unit_price_usd_cents * $4, updated_at = now()
			WHERE organization_id = $1 AND folio_id = $2 AND id = $3
		`, orgID, f.ID, lineID, *in.Quantity)
	case FolioLineCrewTip:
		if in.TipUSDCents == nil || *in.TipUSDCents < 0 {
			return nil, errors.New("tip_usd_cents must be non-negative")
		}
		_, err = tx.Exec(ctx, `
			UPDATE guest_folio_lines
			SET unit_price_usd_cents = $4, line_total_usd_cents = $4, updated_at = now()
			WHERE organization_id = $1 AND folio_id = $2 AND id = $3
		`, orgID, f.ID, lineID, *in.TipUSDCents)
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	view, err := p.GuestFolioByTripGuest(ctx, orgID, tripID, tripGuestID)
	if err == nil {
		view.Warnings = warnings
	}
	return view, err
}

func (p *Pool) DeleteGuestFolioLine(ctx context.Context, orgID, tripID, tripGuestID, lineID, actorID uuid.UUID) (*GuestFolioView, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	f, boatID, err := p.folioForActiveMutationTx(ctx, tx, orgID, tripID, tripGuestID, actorID, false)
	if err != nil {
		return nil, err
	}
	line := &GuestFolioLine{}
	err = scanGuestFolioLine(tx.QueryRow(ctx, `
		SELECT `+guestFolioLineColumns+`
		FROM guest_folio_lines
		WHERE organization_id = $1 AND folio_id = $2 AND id = $3 AND voided_at IS NULL
		FOR UPDATE
	`, orgID, f.ID, lineID), line)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if line.StockMode == StockModeCounted && line.StockPostedAt != nil {
		if line.CatalogItemID == nil {
			return nil, errors.New("counted folio line missing catalog item")
		}
		if _, err := postFolioStockMovementTx(ctx, tx, orgID, boatID, *line.CatalogItemID, line.ID, actorID, MovementFolioVoid, line.Quantity); err != nil {
			return nil, err
		}
	}
	tag, err := tx.Exec(ctx, `
		UPDATE guest_folio_lines
		SET voided_at = now(), voided_by_user_id = $4, updated_at = now()
		WHERE organization_id = $1 AND folio_id = $2 AND id = $3 AND voided_at IS NULL
	`, orgID, f.ID, lineID, actorID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return p.GuestFolioByTripGuest(ctx, orgID, tripID, tripGuestID)
}

func (p *Pool) CloseGuestFolio(ctx context.Context, orgID, tripID, tripGuestID uuid.UUID, in CloseGuestFolioInput) (*GuestFolioView, error) {
	if in.Now.IsZero() {
		in.Now = time.Now().UTC()
	}
	if err := p.EnsurePaymentSettings(ctx, orgID); err != nil {
		return nil, err
	}
	method, err := validatePaymentMethod(in.PaymentMethod)
	if err != nil {
		return nil, err
	}
	settlementCurrency, err := NormalizeCurrency(in.SettlementCurrency)
	if err != nil {
		return nil, err
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	f := &GuestFolio{}
	err = scanGuestFolio(tx.QueryRow(ctx, `
		SELECT `+guestFolioColumns+`
		FROM guest_folios
		WHERE organization_id = $1 AND trip_id = $2 AND trip_guest_id = $3
		FOR UPDATE
	`, orgID, tripID, tripGuestID), f)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if f.Status == FolioStatusClosed {
		return nil, ErrFolioClosed
	}

	var boatID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT t.boat_id
		FROM trips t
		JOIN trip_guests g ON g.trip_id = t.id AND g.organization_id = t.organization_id
		WHERE t.organization_id = $1 AND t.id = $2 AND g.id = $3 AND g.revoked_at IS NULL
		FOR UPDATE OF t
	`, orgID, tripID, tripGuestID).Scan(&boatID)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	settings, err := paymentSettingsTx(ctx, tx, orgID)
	if err != nil {
		return nil, err
	}
	if !hasString(settings.EnabledPaymentMethods, method) {
		return nil, errors.New("payment method is not enabled")
	}
	if !hasString(settings.SupportedCurrencies, settlementCurrency) {
		return nil, errors.New("settlement currency is not enabled")
	}

	lines, err := folioLinesTx(ctx, tx, orgID, f.ID)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, errors.New("cannot close empty folio")
	}
	var subtotal int64
	for _, line := range lines {
		subtotal += line.LineTotalUSDCents
	}
	cardFee := int64(0)
	if method == PaymentMethodCard {
		cardFee = (subtotal*int64(settings.CardFeeBasisPoints) + 5000) / 10000
	}
	total := subtotal + cardFee
	rate, err := latestExchangeRateTx(ctx, tx, "USD", settlementCurrency, in.Now)
	if err != nil {
		return nil, err
	}
	settlementTotal, exp, err := ConvertUSDCentsToMinor(total, settlementCurrency, rate)
	if err != nil {
		return nil, err
	}

	for _, line := range lines {
		if line.StockMode != StockModeCounted || line.StockPostedAt != nil {
			continue
		}
		if line.CatalogItemID == nil {
			return nil, errors.New("counted folio line missing catalog item")
		}
		if _, err := postFolioStockMovementTx(ctx, tx, orgID, boatID, *line.CatalogItemID, line.ID, in.ActorUserID, MovementFolioCharge, -line.Quantity); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE guest_folio_lines SET stock_posted_at = now(), updated_at = now() WHERE id = $1`, line.ID); err != nil {
			return nil, err
		}
	}

	err = scanGuestFolio(tx.QueryRow(ctx, `
		UPDATE guest_folios
		SET status = 'closed',
		    closed_by_user_id = $4,
		    closed_at = $5,
		    subtotal_usd_cents = $6,
		    card_fee_usd_cents = $7,
		    total_usd_cents = $8,
		    settlement_currency = $9,
		    settlement_total_minor = $10,
		    currency_exponent = $11,
		    rate_provider = $12,
		    rate_numerator = $13,
		    rate_denominator = $14,
		    rate_as_of = $15,
		    payment_method = $16,
		    card_fee_basis_points = $17,
		    updated_at = now()
		WHERE organization_id = $1 AND trip_id = $2 AND trip_guest_id = $3
		RETURNING `+guestFolioColumns,
		orgID, tripID, tripGuestID, in.ActorUserID, in.Now, subtotal, cardFee, total,
		settlementCurrency, settlementTotal, exp, rate.Provider, rate.RateNumerator, rate.RateDenominator,
		rate.AsOf, method, settings.CardFeeBasisPoints), f)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	f.Lines = lines
	return p.guestFolioView(ctx, f)
}

func (p *Pool) MarkGuestFolioEmailSent(ctx context.Context, folioID uuid.UUID, at time.Time) error {
	_, err := p.Exec(ctx, `
		UPDATE guest_folios
		SET email_send_status = 'sent', email_last_sent_at = $2, email_last_error = NULL, updated_at = now()
		WHERE id = $1
	`, folioID, at)
	return err
}

func (p *Pool) MarkGuestFolioEmailFailed(ctx context.Context, folioID uuid.UUID, message string) error {
	_, err := p.Exec(ctx, `
		UPDATE guest_folios
		SET email_send_status = 'failed', email_last_error = $2, updated_at = now()
		WHERE id = $1
	`, folioID, message)
	return err
}

func (p *Pool) tripGuestForFolio(ctx context.Context, orgID, tripID, tripGuestID uuid.UUID) (*TripGuest, error) {
	g := &TripGuest{}
	err := scanTripGuest(p.QueryRow(ctx, `
		SELECT `+tripGuestColumns+`
		FROM trip_guests
		WHERE organization_id = $1 AND trip_id = $2 AND id = $3 AND revoked_at IS NULL
	`, orgID, tripID, tripGuestID), g)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return g, err
}

func (p *Pool) folioForMutation(ctx context.Context, orgID, tripID, tripGuestID uuid.UUID) (*GuestFolio, error) {
	f := &GuestFolio{}
	err := scanGuestFolio(p.QueryRow(ctx, `
		SELECT `+guestFolioColumns+`
		FROM guest_folios
		WHERE organization_id = $1 AND trip_id = $2 AND trip_guest_id = $3
	`, orgID, tripID, tripGuestID), f)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if f.Status == FolioStatusClosed {
		return nil, ErrFolioClosed
	}
	return f, nil
}

func (p *Pool) folioForActiveMutationTx(ctx context.Context, tx pgx.Tx, orgID, tripID, tripGuestID, actorID uuid.UUID, lazyOpen bool) (*GuestFolio, uuid.UUID, error) {
	var boatID uuid.UUID
	var status string
	err := tx.QueryRow(ctx, `
		SELECT t.boat_id, t.status
		FROM trips t
		JOIN trip_guests g ON g.trip_id = t.id AND g.organization_id = t.organization_id
		WHERE t.organization_id = $1 AND t.id = $2 AND g.id = $3 AND g.revoked_at IS NULL
		FOR UPDATE OF t, g
	`, orgID, tripID, tripGuestID).Scan(&boatID, &status)
	if isNoRows(err) {
		return nil, uuid.Nil, ErrNotFound
	}
	if err != nil {
		return nil, uuid.Nil, err
	}
	if status != TripStatusActive {
		return nil, uuid.Nil, ErrTripNotActive
	}
	f := &GuestFolio{}
	err = scanGuestFolio(tx.QueryRow(ctx, `
		SELECT `+guestFolioColumns+`
		FROM guest_folios
		WHERE organization_id = $1 AND trip_id = $2 AND trip_guest_id = $3
		FOR UPDATE
	`, orgID, tripID, tripGuestID), f)
	if err != nil && !isNoRows(err) {
		return nil, uuid.Nil, err
	}
	if err != nil && !lazyOpen {
		return nil, uuid.Nil, ErrNotFound
	}
	if err != nil {
		if actorID == uuid.Nil {
			return nil, uuid.Nil, ErrNotFound
		}
		g, err := tripGuestForFolioTx(ctx, tx, orgID, tripID, tripGuestID)
		if err != nil {
			return nil, uuid.Nil, err
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO guest_folios (organization_id, trip_id, trip_guest_id, guest_user_id, status, opened_by_user_id)
			VALUES ($1,$2,$3,$4,'open',$5)
		`, orgID, tripID, tripGuestID, g.GuestUserID, actorID)
		if err != nil && !isUniqueViolation(err, "guest_folios_one_per_trip_guest_idx") {
			return nil, uuid.Nil, err
		}
		if err == nil && tag.RowsAffected() == 0 {
			return nil, uuid.Nil, ErrNotFound
		}
		var folioID uuid.UUID
		if idErr := tx.QueryRow(ctx, `SELECT id FROM guest_folios WHERE organization_id = $1 AND trip_guest_id = $2`, orgID, tripGuestID).Scan(&folioID); idErr != nil {
			return nil, uuid.Nil, idErr
		}
		f = &GuestFolio{
			ID:             folioID,
			OrganizationID: orgID,
			TripID:         tripID,
			TripGuestID:    tripGuestID,
			Status:         FolioStatusOpen,
		}
	}
	if f.Status == FolioStatusClosed {
		return nil, uuid.Nil, ErrFolioClosed
	}
	return f, boatID, nil
}

func (p *Pool) insertFolioLine(ctx context.Context, f *GuestFolio, actorID *uuid.UUID, itemID *uuid.UUID, lineType, name string, qty int, unitCents int64, stockMode string) (*GuestFolioLine, error) {
	var sortOrder int
	_ = p.QueryRow(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM guest_folio_lines WHERE folio_id = $1`, f.ID).Scan(&sortOrder)
	l := &GuestFolioLine{}
	err := scanGuestFolioLine(p.QueryRow(ctx, `
		INSERT INTO guest_folio_lines (
			organization_id, folio_id, trip_guest_id, catalog_item_id, line_type, item_name, quantity,
			unit_price_usd_cents, line_total_usd_cents, stock_mode, sort_order, created_by_user_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING `+guestFolioLineColumns,
		f.OrganizationID, f.ID, f.TripGuestID, itemID, lineType, name, qty, unitCents, unitCents*int64(qty), stockMode, sortOrder, actorID), l)
	return l, err
}

func insertFolioLineTx(ctx context.Context, tx pgx.Tx, f *GuestFolio, actorID *uuid.UUID, itemID *uuid.UUID, lineType, name string, qty int, unitCents int64, stockMode, clientRequestID string) (*GuestFolioLine, error) {
	var sortOrder int
	_ = tx.QueryRow(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM guest_folio_lines WHERE folio_id = $1`, f.ID).Scan(&sortOrder)
	var reqID *string
	if clientRequestID != "" {
		reqID = &clientRequestID
	}
	l := &GuestFolioLine{}
	err := scanGuestFolioLine(tx.QueryRow(ctx, `
		INSERT INTO guest_folio_lines (
			organization_id, folio_id, trip_guest_id, catalog_item_id, line_type, item_name, quantity,
			unit_price_usd_cents, line_total_usd_cents, stock_mode, sort_order, created_by_user_id, client_request_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (trip_guest_id, client_request_id) WHERE client_request_id IS NOT NULL DO NOTHING
		RETURNING `+guestFolioLineColumns,
		f.OrganizationID, f.ID, f.TripGuestID, itemID, lineType, name, qty, unitCents, unitCents*int64(qty), stockMode, sortOrder, actorID, reqID), l)
	if isNoRows(err) {
		return nil, errDuplicateFolioLineRequest
	}
	return l, err
}

func (p *Pool) upsertTipLine(ctx context.Context, f *GuestFolio, actorID *uuid.UUID, amountCents int64) (*GuestFolioLine, error) {
	var sortOrder int
	_ = p.QueryRow(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM guest_folio_lines WHERE folio_id = $1`, f.ID).Scan(&sortOrder)
	l := &GuestFolioLine{}
	err := scanGuestFolioLine(p.QueryRow(ctx, `
		INSERT INTO guest_folio_lines (
			organization_id, folio_id, trip_guest_id, line_type, item_name, quantity,
			unit_price_usd_cents, line_total_usd_cents, stock_mode, sort_order, created_by_user_id
		)
		VALUES ($1,$2,$3,'crew_tip','Crew tip',1,$4,$4,'none',$5,$6)
		ON CONFLICT (folio_id) WHERE line_type = 'crew_tip' AND voided_at IS NULL DO UPDATE SET
			unit_price_usd_cents = EXCLUDED.unit_price_usd_cents,
			line_total_usd_cents = EXCLUDED.line_total_usd_cents,
			created_by_user_id = EXCLUDED.created_by_user_id,
			updated_at = now()
		RETURNING `+guestFolioLineColumns,
		f.OrganizationID, f.ID, f.TripGuestID, amountCents, sortOrder, actorID), l)
	return l, err
}

func upsertTipLineTx(ctx context.Context, tx pgx.Tx, f *GuestFolio, actorID *uuid.UUID, amountCents int64) (*GuestFolioLine, error) {
	var sortOrder int
	_ = tx.QueryRow(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM guest_folio_lines WHERE folio_id = $1 AND voided_at IS NULL`, f.ID).Scan(&sortOrder)
	l := &GuestFolioLine{}
	err := scanGuestFolioLine(tx.QueryRow(ctx, `
		INSERT INTO guest_folio_lines (
			organization_id, folio_id, trip_guest_id, line_type, item_name, quantity,
			unit_price_usd_cents, line_total_usd_cents, stock_mode, sort_order, created_by_user_id
		)
		VALUES ($1,$2,$3,'crew_tip','Crew tip',1,$4,$4,'none',$5,$6)
		ON CONFLICT (folio_id) WHERE line_type = 'crew_tip' AND voided_at IS NULL DO UPDATE SET
			unit_price_usd_cents = EXCLUDED.unit_price_usd_cents,
			line_total_usd_cents = EXCLUDED.line_total_usd_cents,
			created_by_user_id = EXCLUDED.created_by_user_id,
			updated_at = now()
		RETURNING `+guestFolioLineColumns,
		f.OrganizationID, f.ID, f.TripGuestID, amountCents, sortOrder, actorID), l)
	return l, err
}

func (p *Pool) guestFolioView(ctx context.Context, f *GuestFolio) (*GuestFolioView, error) {
	if f.Lines == nil {
		lines, err := p.GuestFolioLines(ctx, f.OrganizationID, f.ID)
		if err != nil {
			return nil, err
		}
		f.Lines = lines
	}
	v := &GuestFolioView{Folio: f}
	err := p.QueryRow(ctx, `
		SELECT o.name, b.display_name, t.itinerary, t.start_date, t.end_date, g.full_name, g.email
		FROM guest_folios f
		JOIN organizations o ON o.id = f.organization_id
		JOIN trips t ON t.id = f.trip_id AND t.organization_id = f.organization_id
		JOIN boats b ON b.id = t.boat_id AND b.organization_id = f.organization_id
		JOIN trip_guests g ON g.id = f.trip_guest_id AND g.organization_id = f.organization_id AND g.trip_id = f.trip_id
		WHERE f.id = $1
	`, f.ID).Scan(&v.OrganizationName, &v.BoatName, &v.TripItinerary, &v.TripStartDate, &v.TripEndDate, &v.GuestFullName, &v.GuestEmail)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func folioLinesTx(ctx context.Context, tx pgx.Tx, orgID, folioID uuid.UUID) ([]*GuestFolioLine, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+guestFolioLineColumns+`
		FROM guest_folio_lines
		WHERE organization_id = $1 AND folio_id = $2 AND voided_at IS NULL
		ORDER BY sort_order, created_at
	`, orgID, folioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*GuestFolioLine
	for rows.Next() {
		l := &GuestFolioLine{}
		if err := scanGuestFolioLine(rows, l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func paymentSettingsTx(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) (*PaymentSettings, error) {
	s := &PaymentSettings{}
	err := scanPaymentSettings(tx.QueryRow(ctx, `
		SELECT organization_id, default_currency, supported_currencies, enabled_payment_methods,
		       card_fee_basis_points, folio_email_footer, created_at, updated_at
		FROM organization_payment_settings
		WHERE organization_id = $1
	`, orgID), s)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return s, err
}

func latestExchangeRateTx(ctx context.Context, tx pgx.Tx, baseCurrency, quoteCurrency string, now time.Time) (*ExchangeRate, error) {
	base, err := NormalizeCurrency(baseCurrency)
	if err != nil {
		return nil, err
	}
	quote, err := NormalizeCurrency(quoteCurrency)
	if err != nil {
		return nil, err
	}
	if base == quote {
		return &ExchangeRate{
			ID:              uuid.Nil,
			Provider:        "identity",
			BaseCurrency:    base,
			QuoteCurrency:   quote,
			RateNumerator:   1,
			RateDenominator: 1,
			AsOf:            now,
			FetchedAt:       now,
			ExpiresAt:       now.Add(time.Hour),
		}, nil
	}
	r := &ExchangeRate{}
	err = scanExchangeRate(tx.QueryRow(ctx, `
		SELECT id, provider, base_currency, quote_currency, rate_numerator, rate_denominator, as_of, fetched_at, expires_at
		FROM exchange_rates
		WHERE base_currency = $1 AND quote_currency = $2 AND expires_at > $3
		ORDER BY as_of DESC, fetched_at DESC
		LIMIT 1
	`, base, quote, now), r)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return r, err
}

func tripGuestForFolioTx(ctx context.Context, tx pgx.Tx, orgID, tripID, tripGuestID uuid.UUID) (*TripGuest, error) {
	g := &TripGuest{}
	err := scanTripGuest(tx.QueryRow(ctx, `
		SELECT `+tripGuestColumns+`
		FROM trip_guests
		WHERE organization_id = $1 AND trip_id = $2 AND id = $3 AND revoked_at IS NULL
	`, orgID, tripID, tripGuestID), g)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return g, err
}

func catalogItemByIDTx(ctx context.Context, tx pgx.Tx, orgID, itemID uuid.UUID) (*CatalogItem, error) {
	i := &CatalogItem{}
	err := scanCatalogItem(tx.QueryRow(ctx, `
		SELECT `+catalogItemSelect+`
		FROM catalog_items i
		JOIN catalog_categories c ON c.id = i.category_id
		WHERE i.organization_id = $1 AND i.id = $2
	`, orgID, itemID), i)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return i, err
}

func folioLineByClientRequestTx(ctx context.Context, tx pgx.Tx, orgID, tripGuestID uuid.UUID, clientRequestID string) (*GuestFolioLine, error) {
	l := &GuestFolioLine{}
	err := scanGuestFolioLine(tx.QueryRow(ctx, `
		SELECT `+guestFolioLineColumns+`
		FROM guest_folio_lines
		WHERE organization_id = $1 AND trip_guest_id = $2 AND client_request_id = $3
	`, orgID, tripGuestID, clientRequestID), l)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return l, err
}

func (p *Pool) OpenGuestFoliosForTripTx(ctx context.Context, tx pgx.Tx, orgID, tripID, actorID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO guest_folios (organization_id, trip_id, trip_guest_id, guest_user_id, status, opened_by_user_id)
		SELECT g.organization_id, g.trip_id, g.id, g.guest_user_id, 'open', $3
		FROM trip_guests g
		WHERE g.organization_id = $1 AND g.trip_id = $2 AND g.revoked_at IS NULL
		ON CONFLICT (trip_guest_id) DO NOTHING
	`, orgID, tripID, actorID)
	return err
}

func postFolioStockMovementTx(ctx context.Context, tx pgx.Tx, orgID, boatID, itemID, lineID, actorID uuid.UUID, movementType string, delta int) (int, error) {
	if delta == 0 {
		var current int
		err := tx.QueryRow(ctx, `
			SELECT quantity_on_hand
			FROM boat_inventory_items
			WHERE organization_id = $1 AND boat_id = $2 AND catalog_item_id = $3
		`, orgID, boatID, itemID).Scan(&current)
		if isNoRows(err) {
			return 0, nil
		}
		return current, err
	}
	var before int
	err := tx.QueryRow(ctx, `
		SELECT quantity_on_hand
		FROM boat_inventory_items
		WHERE organization_id = $1 AND boat_id = $2 AND catalog_item_id = $3
		FOR UPDATE
	`, orgID, boatID, itemID).Scan(&before)
	if isNoRows(err) {
		before = 0
		if _, err := tx.Exec(ctx, `
			INSERT INTO boat_inventory_items (organization_id, boat_id, catalog_item_id, quantity_on_hand)
			VALUES ($1,$2,$3,0)
		`, orgID, boatID, itemID); err != nil {
			return 0, err
		}
	} else if err != nil {
		return 0, err
	}
	after := before + delta
	if _, err := tx.Exec(ctx, `
		UPDATE boat_inventory_items
		SET quantity_on_hand = $4, updated_at = now()
		WHERE organization_id = $1 AND boat_id = $2 AND catalog_item_id = $3
	`, orgID, boatID, itemID, after); err != nil {
		return 0, err
	}
	sourceType := "guest_folio_line"
	_, err = tx.Exec(ctx, `
		INSERT INTO stock_movements (
			organization_id, boat_id, catalog_item_id, actor_user_id, movement_type,
			delta_quantity, quantity_before, quantity_after, source_type, source_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, orgID, boatID, itemID, actorID, movementType, delta, before, after, sourceType, lineID)
	return after, err
}

func negativeStockWarning(itemID uuid.UUID, itemName string, quantity int) FolioWarning {
	return FolioWarning{
		Code:           "negative_stock",
		Message:        itemName + " stock is below zero.",
		CatalogItemID:  &itemID,
		QuantityOnHand: &quantity,
	}
}

func hasString(values []string, needle string) bool {
	for _, v := range values {
		if v == needle {
			return true
		}
	}
	return false
}
