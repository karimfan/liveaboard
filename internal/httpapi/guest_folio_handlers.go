package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/email"
	"github.com/karimfan/liveaboard/internal/store"
)

type addFolioLineReq struct {
	LineType        string `json:"line_type"`
	CatalogItemID   string `json:"catalog_item_id,omitempty"`
	Quantity        int    `json:"quantity,omitempty"`
	TipUSDCents     int64  `json:"tip_usd_cents,omitempty"`
	ClientRequestID string `json:"client_request_id,omitempty"`
}

type updateFolioLineReq struct {
	Quantity    *int   `json:"quantity,omitempty"`
	TipUSDCents *int64 `json:"tip_usd_cents,omitempty"`
}

type closeFolioReq struct {
	PaymentMethod      string `json:"payment_method"`
	SettlementCurrency string `json:"settlement_currency"`
}

type addLedgerLineReq struct {
	TripGuestID     string `json:"trip_guest_id"`
	CatalogItemID   string `json:"catalog_item_id"`
	Quantity        int    `json:"quantity"`
	ClientRequestID string `json:"client_request_id,omitempty"`
}

func (s *Server) handleTripLedger(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	ledger, err := s.Auth.Store.TripConsumptionLedger(r.Context(), u.OrganizationID, tripID)
	if err != nil {
		writeFolioError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tripLedgerView(ledger))
}

func (s *Server) handleAddTripLedgerLine(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	var req addLedgerLineReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	guestID, err := uuid.Parse(req.TripGuestID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "trip_guest_id must be a uuid")
		return
	}
	itemID, err := uuid.Parse(req.CatalogItemID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "catalog_item_id must be a uuid")
		return
	}
	view, err := s.Auth.Store.AddGuestFolioLine(r.Context(), u.OrganizationID, tripID, guestID, store.AddFolioLineInput{
		ActorUserID:     u.ID,
		LineType:        store.FolioLineCatalogItem,
		CatalogItemID:   itemID,
		Quantity:        req.Quantity,
		ClientRequestID: req.ClientRequestID,
	})
	if err != nil {
		writeFolioError(w, err)
		return
	}
	s.recordStaffAudit(r.Context(), u.OrganizationID, u.ID, "guest.folio_line_added", "guest_folio", &view.Folio.ID, &tripID, &guestID, map[string]any{"line_type": store.FolioLineCatalogItem, "quantity": req.Quantity, "catalog_item_id": itemID})
	writeJSON(w, http.StatusOK, guestFolioView(view))
}

func (s *Server) handleGetGuestFolio(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, guestID, ok := tripAndGuestParams(w, r)
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	view, err := s.Auth.Store.GuestFolioByTripGuest(r.Context(), u.OrganizationID, tripID, guestID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "folio not found")
		return
	}
	if err != nil {
		writeFolioError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, guestFolioView(view))
}

func (s *Server) handleOpenGuestFolio(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, guestID, ok := tripAndGuestParams(w, r)
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	if !s.ensureTripMutable(w, r, u.OrganizationID, tripID) {
		return
	}
	view, err := s.Auth.Store.OpenGuestFolio(r.Context(), u.OrganizationID, tripID, guestID, u.ID)
	if err != nil {
		writeFolioError(w, err)
		return
	}
	s.recordStaffAudit(r.Context(), u.OrganizationID, u.ID, "guest.folio_opened", "guest_folio", &view.Folio.ID, &tripID, &guestID, map[string]any{"status": view.Folio.Status})
	writeJSON(w, http.StatusCreated, guestFolioView(view))
}

func (s *Server) handleAddGuestFolioLine(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, guestID, ok := tripAndGuestParams(w, r)
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	if !s.ensureTripMutable(w, r, u.OrganizationID, tripID) {
		return
	}
	var req addFolioLineReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	var itemID uuid.UUID
	if req.LineType == "" || req.LineType == store.FolioLineCatalogItem {
		id, err := uuid.Parse(req.CatalogItemID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "catalog_item_id must be a uuid")
			return
		}
		itemID = id
	}
	view, err := s.Auth.Store.AddGuestFolioLine(r.Context(), u.OrganizationID, tripID, guestID, store.AddFolioLineInput{
		ActorUserID:     u.ID,
		LineType:        req.LineType,
		CatalogItemID:   itemID,
		Quantity:        req.Quantity,
		TipUSDCents:     req.TipUSDCents,
		ClientRequestID: req.ClientRequestID,
	})
	if err != nil {
		writeFolioError(w, err)
		return
	}
	s.recordStaffAudit(r.Context(), u.OrganizationID, u.ID, "guest.folio_line_added", "guest_folio", &view.Folio.ID, &tripID, &guestID, map[string]any{"line_type": req.LineType, "quantity": req.Quantity})
	writeJSON(w, http.StatusOK, guestFolioView(view))
}

func (s *Server) handleUpdateGuestFolioLine(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, guestID, ok := tripAndGuestParams(w, r)
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	if !s.ensureTripMutable(w, r, u.OrganizationID, tripID) {
		return
	}
	lineID, err := uuid.Parse(chi.URLParam(r, "line_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "line_id must be a uuid")
		return
	}
	var req updateFolioLineReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	view, err := s.Auth.Store.UpdateGuestFolioLine(r.Context(), u.OrganizationID, tripID, guestID, lineID, store.UpdateFolioLineInput{
		ActorUserID: u.ID,
		Quantity:    req.Quantity,
		TipUSDCents: req.TipUSDCents,
	})
	if err != nil {
		writeFolioError(w, err)
		return
	}
	meta := map[string]any{"line_type": "unknown"}
	if req.Quantity != nil {
		meta["quantity"] = *req.Quantity
	}
	s.recordStaffAudit(r.Context(), u.OrganizationID, u.ID, "guest.folio_line_updated", "guest_folio_line", &lineID, &tripID, &guestID, meta)
	writeJSON(w, http.StatusOK, guestFolioView(view))
}

func (s *Server) handleDeleteGuestFolioLine(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, guestID, ok := tripAndGuestParams(w, r)
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	if !s.ensureTripMutable(w, r, u.OrganizationID, tripID) {
		return
	}
	lineID, err := uuid.Parse(chi.URLParam(r, "line_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "line_id must be a uuid")
		return
	}
	view, err := s.Auth.Store.DeleteGuestFolioLine(r.Context(), u.OrganizationID, tripID, guestID, lineID, u.ID)
	if err != nil {
		writeFolioError(w, err)
		return
	}
	s.recordStaffAudit(r.Context(), u.OrganizationID, u.ID, "guest.folio_line_deleted", "guest_folio_line", &lineID, &tripID, &guestID, map[string]any{"line_type": "unknown"})
	writeJSON(w, http.StatusOK, guestFolioView(view))
}

func (s *Server) handleCloseGuestFolio(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, guestID, ok := tripAndGuestParams(w, r)
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	var req closeFolioReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	view, err := s.Auth.Store.CloseGuestFolio(r.Context(), u.OrganizationID, tripID, guestID, store.CloseGuestFolioInput{
		ActorUserID:        u.ID,
		PaymentMethod:      req.PaymentMethod,
		SettlementCurrency: req.SettlementCurrency,
		Now:                time.Now().UTC(),
	})
	if err != nil {
		writeFolioError(w, err)
		return
	}
	if err := s.sendGuestFolioEmail(r, view); err != nil {
		_ = s.Auth.Store.MarkGuestFolioEmailFailed(r.Context(), view.Folio.ID, err.Error())
		s.Log.Error("guest folio email failed", "err", err, "folio_id", view.Folio.ID)
	} else {
		_ = s.Auth.Store.MarkGuestFolioEmailSent(r.Context(), view.Folio.ID, time.Now().UTC())
	}
	refreshed, err := s.Auth.Store.GuestFolioByTripGuest(r.Context(), u.OrganizationID, tripID, guestID)
	if err == nil {
		view = refreshed
	}
	meta := map[string]any{
		"payment_method":        req.PaymentMethod,
		"settlement_currency":   req.SettlementCurrency,
		"total_usd_cents":       view.Folio.TotalUSDCents,
		"card_fee_basis_points": view.Folio.CardFeeBasisPoints,
	}
	s.recordStaffAudit(r.Context(), u.OrganizationID, u.ID, "guest.folio_closed", "guest_folio", &view.Folio.ID, &tripID, &guestID, meta)
	writeJSON(w, http.StatusOK, guestFolioView(view))
}

func (s *Server) handleResendGuestFolioEmail(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, guestID, ok := tripAndGuestParams(w, r)
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	view, err := s.Auth.Store.GuestFolioByTripGuest(r.Context(), u.OrganizationID, tripID, guestID)
	if err != nil {
		writeFolioError(w, err)
		return
	}
	if view.Folio.Status != store.FolioStatusClosed {
		writeError(w, http.StatusBadRequest, "invalid_input", "folio is not closed")
		return
	}
	if err := s.sendGuestFolioEmail(r, view); err != nil {
		_ = s.Auth.Store.MarkGuestFolioEmailFailed(r.Context(), view.Folio.ID, err.Error())
		writeError(w, http.StatusBadGateway, "email_failed", "folio email could not be sent")
		return
	}
	_ = s.Auth.Store.MarkGuestFolioEmailSent(r.Context(), view.Folio.ID, time.Now().UTC())
	view, _ = s.Auth.Store.GuestFolioByTripGuest(r.Context(), u.OrganizationID, tripID, guestID)
	writeJSON(w, http.StatusOK, guestFolioView(view))
}

func (s *Server) sendGuestFolioEmail(r *http.Request, view *store.GuestFolioView) error {
	settings, err := s.Auth.Store.PaymentSettings(r.Context(), view.Folio.OrganizationID, time.Now().UTC())
	if err != nil {
		return err
	}
	lines := make([]email.FolioLineVar, 0, len(view.Folio.Lines))
	for _, line := range view.Folio.Lines {
		lines = append(lines, email.FolioLineVar{
			Name:      line.ItemName,
			Quantity:  line.Quantity,
			UnitPrice: formatUSD(line.UnitPriceUSDCents),
			Total:     formatUSD(line.LineTotalUSDCents),
		})
	}
	cardFee := ""
	if view.Folio.CardFeeUSDCents > 0 {
		cardFee = formatUSD(view.Folio.CardFeeUSDCents)
	}
	settlementTotal := ""
	if view.Folio.SettlementTotalMinor != nil && view.Folio.CurrencyExponent != nil {
		settlementTotal = formatMinor(*view.Folio.SettlementTotalMinor, *view.Folio.CurrencyExponent)
	}
	paymentMethod := ""
	if view.Folio.PaymentMethod != nil {
		paymentMethod = strings.ReplaceAll(*view.Folio.PaymentMethod, "_", " ")
	}
	currency := ""
	if view.Folio.SettlementCurrency != nil {
		currency = *view.Folio.SettlementCurrency
	}
	footer := ""
	if settings.FolioEmailFooter != nil {
		footer = *settings.FolioEmailFooter
	}
	msg, err := email.Render(email.KindGuestFolioClosed, email.Vars{
		AppName:                 "Liveaboard",
		OrganizationName:        view.OrganizationName,
		RecipientName:           view.GuestFullName,
		RecipientEmail:          view.GuestEmail,
		TripBoatName:            view.BoatName,
		TripItinerary:           view.TripItinerary,
		TripStartDate:           view.TripStartDate,
		TripEndDate:             view.TripEndDate,
		FolioLines:              lines,
		FolioSubtotalUSD:        formatUSD(view.Folio.SubtotalUSDCents),
		FolioCardFeeUSD:         cardFee,
		FolioTotalUSD:           formatUSD(view.Folio.TotalUSDCents),
		FolioSettlementTotal:    settlementTotal,
		FolioSettlementCurrency: currency,
		FolioPaymentMethod:      paymentMethod,
		FolioFooter:             footer,
	})
	if err != nil {
		return err
	}
	msg.From = s.Auth.SenderFrom
	msg.To = view.GuestEmail
	return s.Auth.Email.Send(r.Context(), msg)
}

func guestFolioView(v *store.GuestFolioView) map[string]any {
	f := v.Folio
	lines := make([]map[string]any, 0, len(f.Lines))
	for _, line := range f.Lines {
		lines = append(lines, map[string]any{
			"id":                   line.ID,
			"catalog_item_id":      line.CatalogItemID,
			"line_type":            line.LineType,
			"item_name":            line.ItemName,
			"quantity":             line.Quantity,
			"unit_price_usd_cents": line.UnitPriceUSDCents,
			"line_total_usd_cents": line.LineTotalUSDCents,
			"stock_mode":           line.StockMode,
			"sort_order":           line.SortOrder,
			"created_at":           line.CreatedAt,
			"updated_at":           line.UpdatedAt,
			"stock_posted_at":      line.StockPostedAt,
			"client_request_id":    line.ClientRequestID,
		})
	}
	warnings := make([]map[string]any, 0, len(v.Warnings))
	for _, warning := range v.Warnings {
		warnings = append(warnings, map[string]any{
			"code":             warning.Code,
			"message":          warning.Message,
			"catalog_item_id":  warning.CatalogItemID,
			"quantity_on_hand": warning.QuantityOnHand,
		})
	}
	return map[string]any{
		"id":                     f.ID,
		"organization_id":        f.OrganizationID,
		"trip_id":                f.TripID,
		"trip_guest_id":          f.TripGuestID,
		"status":                 f.Status,
		"closed_at":              f.ClosedAt,
		"subtotal_usd_cents":     f.SubtotalUSDCents,
		"card_fee_usd_cents":     f.CardFeeUSDCents,
		"total_usd_cents":        f.TotalUSDCents,
		"settlement_currency":    f.SettlementCurrency,
		"settlement_total_minor": f.SettlementTotalMinor,
		"currency_exponent":      f.CurrencyExponent,
		"rate_provider":          f.RateProvider,
		"rate_numerator":         f.RateNumerator,
		"rate_denominator":       f.RateDenominator,
		"rate_as_of":             f.RateAsOf,
		"payment_method":         f.PaymentMethod,
		"card_fee_basis_points":  f.CardFeeBasisPoints,
		"email_send_status":      f.EmailSendStatus,
		"email_last_sent_at":     f.EmailLastSentAt,
		"email_last_error":       f.EmailLastError,
		"lines":                  lines,
		"organization_name":      v.OrganizationName,
		"boat_name":              v.BoatName,
		"itinerary":              v.TripItinerary,
		"start_date":             v.TripStartDate,
		"end_date":               v.TripEndDate,
		"guest_full_name":        v.GuestFullName,
		"guest_email":            v.GuestEmail,
		"warnings":               warnings,
	}
}

func tripLedgerView(v *store.TripLedgerView) map[string]any {
	guests := make([]map[string]any, 0, len(v.Guests))
	for _, g := range v.Guests {
		guests = append(guests, map[string]any{
			"trip_guest_id":      g.TripGuestID,
			"full_name":          g.FullName,
			"email":              g.Email,
			"folio_id":           g.FolioID,
			"folio_status":       g.FolioStatus,
			"line_count":         g.LineCount,
			"subtotal_usd_cents": g.SubtotalUSDCents,
		})
	}
	catalog := make([]map[string]any, 0, len(v.Catalog))
	for _, item := range v.Catalog {
		if !item.IsActive || item.ArchivedAt != nil {
			continue
		}
		catalog = append(catalog, catalogItemView(item))
	}
	inventory := make([]map[string]any, 0, len(v.Inventory))
	for _, item := range v.Inventory {
		inventory = append(inventory, map[string]any{
			"catalog_item_id":  item.CatalogItemID,
			"quantity_on_hand": item.QuantityOnHand,
			"status":           item.Status,
		})
	}
	recent := make([]map[string]any, 0, len(v.Recent))
	for _, line := range v.Recent {
		recent = append(recent, map[string]any{
			"id":                   line.ID,
			"trip_guest_id":        line.TripGuestID,
			"guest_full_name":      line.GuestFullName,
			"item_name":            line.ItemName,
			"quantity":             line.Quantity,
			"line_total_usd_cents": line.LineTotalUSDCents,
			"stock_mode":           line.StockMode,
			"created_at":           line.CreatedAt,
		})
	}
	return map[string]any{
		"trip": map[string]any{
			"id":         v.Trip.ID,
			"boat_id":    v.Trip.BoatID,
			"status":     v.Trip.Status,
			"itinerary":  v.Trip.Itinerary,
			"start_date": v.Trip.StartDate,
			"end_date":   v.Trip.EndDate,
		},
		"guests":    guests,
		"catalog":   catalog,
		"inventory": inventory,
		"recent":    recent,
	}
}

func writeFolioError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, store.ErrFolioExists):
		writeError(w, http.StatusConflict, "folio_exists", "folio already exists")
	case errors.Is(err, store.ErrFolioClosed):
		writeError(w, http.StatusConflict, "folio_closed", "folio is closed")
	case errors.Is(err, store.ErrTripNotActive):
		writeError(w, http.StatusConflict, "trip_not_active", "trip is not active")
	case strings.Contains(err.Error(), "stock adjustment would make quantity negative"):
		writeError(w, http.StatusConflict, "insufficient_stock", "not enough stock to close this folio")
	default:
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	}
}

func formatUSD(cents int64) string {
	return "$" + formatMinor(cents, 2)
}

func formatMinor(amount int64, exp int) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	scale := int64(1)
	for i := 0; i < exp; i++ {
		scale *= 10
	}
	if exp == 0 {
		return fmt.Sprintf("%s%d", sign, amount)
	}
	whole := amount / scale
	frac := amount % scale
	return fmt.Sprintf("%s%d.%0*d", sign, whole, exp, frac)
}
