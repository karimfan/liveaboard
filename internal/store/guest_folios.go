package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrFolioExists = errors.New("store: folio already exists")
	ErrFolioClosed = errors.New("store: folio is closed")
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
	CatalogItemID     *uuid.UUID
	LineType          string
	ItemName          string
	Quantity          int
	UnitPriceUSDCents int64
	LineTotalUSDCents int64
	StockMode         string
	SortOrder         int
	CreatedByUserID   *uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
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
}

type AddFolioLineInput struct {
	ActorUserID   uuid.UUID
	LineType      string
	CatalogItemID uuid.UUID
	Quantity      int
	TipUSDCents   int64
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
	sort_order, created_by_user_id, created_at, updated_at`

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
		&l.SortOrder, &l.CreatedByUserID, &l.CreatedAt, &l.UpdatedAt)
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
		WHERE organization_id = $1 AND folio_id = $2
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

func (p *Pool) AddGuestFolioLine(ctx context.Context, orgID, tripID, tripGuestID uuid.UUID, in AddFolioLineInput) (*GuestFolioView, error) {
	f, err := p.folioForMutation(ctx, orgID, tripID, tripGuestID)
	if err != nil {
		return nil, err
	}
	lineType := in.LineType
	if lineType == "" {
		lineType = FolioLineCatalogItem
	}
	switch lineType {
	case FolioLineCatalogItem:
		if in.Quantity <= 0 {
			return nil, errors.New("quantity must be positive")
		}
		item, err := p.CatalogItemByID(ctx, orgID, in.CatalogItemID)
		if err != nil {
			return nil, err
		}
		if !item.IsActive || item.ArchivedAt != nil {
			return nil, ErrNotFound
		}
		if _, err := p.insertFolioLine(ctx, f, &in.ActorUserID, &in.CatalogItemID, FolioLineCatalogItem, item.Name, in.Quantity, item.PriceUSDCents, item.StockMode); err != nil {
			return nil, err
		}
	case FolioLineCrewTip:
		if in.TipUSDCents < 0 {
			return nil, errors.New("tip_usd_cents must be non-negative")
		}
		if _, err := p.upsertTipLine(ctx, f, &in.ActorUserID, in.TipUSDCents); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported line_type")
	}
	return p.GuestFolioByTripGuest(ctx, orgID, tripID, tripGuestID)
}

func (p *Pool) UpdateGuestFolioLine(ctx context.Context, orgID, tripID, tripGuestID, lineID uuid.UUID, in UpdateFolioLineInput) (*GuestFolioView, error) {
	f, err := p.folioForMutation(ctx, orgID, tripID, tripGuestID)
	if err != nil {
		return nil, err
	}
	line := &GuestFolioLine{}
	err = scanGuestFolioLine(p.QueryRow(ctx, `
		SELECT `+guestFolioLineColumns+`
		FROM guest_folio_lines
		WHERE organization_id = $1 AND folio_id = $2 AND id = $3
	`, orgID, f.ID, lineID), line)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	switch line.LineType {
	case FolioLineCatalogItem:
		if in.Quantity == nil || *in.Quantity <= 0 {
			return nil, errors.New("quantity must be positive")
		}
		_, err = p.Exec(ctx, `
			UPDATE guest_folio_lines
			SET quantity = $4, line_total_usd_cents = unit_price_usd_cents * $4, updated_at = now()
			WHERE organization_id = $1 AND folio_id = $2 AND id = $3
		`, orgID, f.ID, lineID, *in.Quantity)
	case FolioLineCrewTip:
		if in.TipUSDCents == nil || *in.TipUSDCents < 0 {
			return nil, errors.New("tip_usd_cents must be non-negative")
		}
		_, err = p.Exec(ctx, `
			UPDATE guest_folio_lines
			SET unit_price_usd_cents = $4, line_total_usd_cents = $4, updated_at = now()
			WHERE organization_id = $1 AND folio_id = $2 AND id = $3
		`, orgID, f.ID, lineID, *in.TipUSDCents)
	}
	if err != nil {
		return nil, err
	}
	return p.GuestFolioByTripGuest(ctx, orgID, tripID, tripGuestID)
}

func (p *Pool) DeleteGuestFolioLine(ctx context.Context, orgID, tripID, tripGuestID, lineID uuid.UUID) (*GuestFolioView, error) {
	f, err := p.folioForMutation(ctx, orgID, tripID, tripGuestID)
	if err != nil {
		return nil, err
	}
	tag, err := p.Exec(ctx, `
		DELETE FROM guest_folio_lines
		WHERE organization_id = $1 AND folio_id = $2 AND id = $3
	`, orgID, f.ID, lineID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
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
		if line.StockMode != StockModeCounted {
			continue
		}
		if line.CatalogItemID == nil {
			return nil, errors.New("counted folio line missing catalog item")
		}
		if err := decrementStockForFolioLine(ctx, tx, orgID, boatID, *line.CatalogItemID, line.ID, in.ActorUserID, line.Quantity); err != nil {
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

func (p *Pool) insertFolioLine(ctx context.Context, f *GuestFolio, actorID *uuid.UUID, itemID *uuid.UUID, lineType, name string, qty int, unitCents int64, stockMode string) (*GuestFolioLine, error) {
	var sortOrder int
	_ = p.QueryRow(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM guest_folio_lines WHERE folio_id = $1`, f.ID).Scan(&sortOrder)
	l := &GuestFolioLine{}
	err := scanGuestFolioLine(p.QueryRow(ctx, `
		INSERT INTO guest_folio_lines (
			organization_id, folio_id, catalog_item_id, line_type, item_name, quantity,
			unit_price_usd_cents, line_total_usd_cents, stock_mode, sort_order, created_by_user_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING `+guestFolioLineColumns,
		f.OrganizationID, f.ID, itemID, lineType, name, qty, unitCents, unitCents*int64(qty), stockMode, sortOrder, actorID), l)
	return l, err
}

func (p *Pool) upsertTipLine(ctx context.Context, f *GuestFolio, actorID *uuid.UUID, amountCents int64) (*GuestFolioLine, error) {
	var sortOrder int
	_ = p.QueryRow(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM guest_folio_lines WHERE folio_id = $1`, f.ID).Scan(&sortOrder)
	l := &GuestFolioLine{}
	err := scanGuestFolioLine(p.QueryRow(ctx, `
		INSERT INTO guest_folio_lines (
			organization_id, folio_id, line_type, item_name, quantity,
			unit_price_usd_cents, line_total_usd_cents, stock_mode, sort_order, created_by_user_id
		)
		VALUES ($1,$2,'crew_tip','Crew tip',1,$3,$3,'none',$4,$5)
		ON CONFLICT (folio_id) WHERE line_type = 'crew_tip' DO UPDATE SET
			unit_price_usd_cents = EXCLUDED.unit_price_usd_cents,
			line_total_usd_cents = EXCLUDED.line_total_usd_cents,
			created_by_user_id = EXCLUDED.created_by_user_id,
			updated_at = now()
		RETURNING `+guestFolioLineColumns,
		f.OrganizationID, f.ID, amountCents, sortOrder, actorID), l)
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
		WHERE organization_id = $1 AND folio_id = $2
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

func decrementStockForFolioLine(ctx context.Context, tx pgx.Tx, orgID, boatID, itemID, lineID, actorID uuid.UUID, qty int) error {
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
			return err
		}
	} else if err != nil {
		return err
	}
	after := before - qty
	if after < 0 {
		return errors.New("stock adjustment would make quantity negative")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE boat_inventory_items
		SET quantity_on_hand = $4, updated_at = now()
		WHERE organization_id = $1 AND boat_id = $2 AND catalog_item_id = $3
	`, orgID, boatID, itemID, after); err != nil {
		return err
	}
	sourceType := "guest_folio_line"
	_, err = tx.Exec(ctx, `
		INSERT INTO stock_movements (
			organization_id, boat_id, catalog_item_id, actor_user_id, movement_type,
			delta_quantity, quantity_before, quantity_after, source_type, source_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, orgID, boatID, itemID, actorID, MovementFolioCharge, -qty, before, after, sourceType, lineID)
	return err
}

func hasString(values []string, needle string) bool {
	for _, v := range values {
		if v == needle {
			return true
		}
	}
	return false
}
