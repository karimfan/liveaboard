package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

// handleAdminReports serves GET /api/admin/reports.
//
// Query params:
//   - from, to: ISO YYYY-MM-DD bounds for the trip status / revenue
//     rollups. Both optional; missing values fall back to the default
//     window (-30d / +180d). Windows wider than store.MaxReportWindow
//     are rejected with 400.
func (s *Server) handleAdminReports(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "session required")
		return
	}
	window, ok := parseReportWindow(w, r)
	if !ok {
		return
	}
	out, err := s.Auth.Store.AdminReports(r.Context(), u.OrganizationID, window)
	if err != nil {
		s.Log.Error("admin reports", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, adminReportsView(out))
}

// handleTripDashboard serves GET /api/admin/trips/{id}/dashboard.
// Org Admins can read any org trip; Cruise Directors only assigned
// trips. Other roles get 403.
func (s *Server) handleTripDashboard(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "session required")
		return
	}
	tripID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}

	switch u.Role {
	case store.RoleOrgAdmin:
		// any org trip
	case store.RoleCruiseDirector:
		assigned, err := s.Auth.Store.UserAssignedToTrip(r.Context(), tripID, u.ID)
		if err != nil {
			s.Log.Error("trip dashboard: assignment lookup", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		if !assigned {
			writeError(w, http.StatusForbidden, "forbidden", "not assigned to this trip")
			return
		}
	default:
		writeError(w, http.StatusForbidden, "forbidden", "role cannot view trip dashboard")
		return
	}

	dash, err := s.Auth.Store.TripDashboard(r.Context(), u.OrganizationID, tripID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "trip not found")
		return
	}
	if err != nil {
		s.Log.Error("trip dashboard", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, tripDashboardView(dash))
}

// handleGuestTab serves GET /api/guest/trip-registrations/{trip_guest_id}/tab.
// The guest session middleware has already established a *store.GuestUser
// on the context; the store query joins trip_guests on guest_user_id so
// a tampered URL cannot reveal another guest's data.
func (s *Server) handleGuestTab(w http.ResponseWriter, r *http.Request) {
	g := auth.GuestFromContext(r.Context())
	if g == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "guest session required")
		return
	}
	tripGuestID, ok := uuidParam(w, r, "trip_guest_id")
	if !ok {
		return
	}
	tab, err := s.Auth.Store.GuestTab(r.Context(), g.ID, tripGuestID)
	if errors.Is(err, store.ErrNotFound) {
		// Opaque 404, matching the existing guest routes' treatment of
		// foreign trip_guest_ids.
		writeError(w, http.StatusNotFound, "not_found", "trip not found")
		return
	}
	if err != nil {
		s.Log.Error("guest tab", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, guestTabView(tab))
}

// --- helpers ---

// parseReportWindow reads from / to query params and validates them.
// Returns (window, true) on success; on failure, writes a 400 error
// to w and returns (_, false).
func parseReportWindow(w http.ResponseWriter, r *http.Request) (store.ReportWindow, bool) {
	now := time.Now().UTC()
	def := store.DefaultReportWindow(now)
	q := r.URL.Query()
	parse := func(raw string) (time.Time, error) {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	win := def
	if s := q.Get("from"); s != "" {
		t, err := parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "from must be YYYY-MM-DD")
			return store.ReportWindow{}, false
		}
		win.From = t
	}
	if s := q.Get("to"); s != "" {
		t, err := parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "to must be YYYY-MM-DD")
			return store.ReportWindow{}, false
		}
		win.To = t
	}
	if err := win.Validate(); err != nil {
		if errors.Is(err, store.ErrReportWindowTooWide) {
			writeError(w, http.StatusBadRequest, "window_too_wide", "report window exceeds the 1-year maximum")
			return store.ReportWindow{}, false
		}
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return store.ReportWindow{}, false
	}
	return win, true
}

// --- view shapers ---

func adminReportsView(r *store.AdminReports) map[string]any {
	return map[string]any{
		"window": map[string]any{
			"from": r.Window.From.Format("2006-01-02"),
			"to":   r.Window.To.Format("2006-01-02"),
		},
		"setup": map[string]any{
			"pct":   r.Setup.Percent,
			"steps": setupStepsView(r.Setup.Steps),
		},
		"trip_status_counts": map[string]any{
			"planned":   r.TripStatusCounts.Planned,
			"active":    r.TripStatusCounts.Active,
			"completed": r.TripStatusCounts.Completed,
			"cancelled": r.TripStatusCounts.Cancelled,
		},
		"trip_operational": tripOperationalRowsView(r.TripOperational),
		"trip_revenue":     tripRevenueRowsView(r.TripRevenue),
	}
}

func tripOperationalRowsView(rows []store.TripOperationalRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"trip_id":           r.TripID,
			"boat_id":           r.BoatID,
			"boat_name":         r.BoatName,
			"start_date":        r.StartDate.Format("2006-01-02"),
			"end_date":          r.EndDate.Format("2006-01-02"),
			"status":            r.Status,
			"num_guests":        r.NumGuests,
			"guest_count":       r.GuestCount,
			"submitted_count":   r.SubmittedCount,
			"document_count":    r.DocumentCount,
			"cabin_assignments": r.CabinAssignments,
			"director_count":    r.DirectorCount,
		})
	}
	return out
}

func tripRevenueRowsView(rows []store.TripRevenueRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		settle := make([]map[string]any, 0, len(r.SettlementByCurrency))
		for _, s := range r.SettlementByCurrency {
			settle = append(settle, map[string]any{
				"currency":    s.Currency,
				"total_minor": s.TotalMinor,
				"folio_count": s.FolioCount,
			})
		}
		out = append(out, map[string]any{
			"trip_id":                r.TripID,
			"boat_name":              r.BoatName,
			"start_date":             r.StartDate.Format("2006-01-02"),
			"end_date":               r.EndDate.Format("2006-01-02"),
			"status":                 r.Status,
			"open_folio_count":       r.OpenFolioCount,
			"closed_folio_count":     r.ClosedFolioCount,
			"charges_usd_cents":      r.ChargesUSDCents,
			"crew_tip_usd_cents":     r.CrewTipUSDCents,
			"card_fee_usd_cents":     r.CardFeeUSDCents,
			"settled_usd_cents":      r.SettledUSDCents,
			"outstanding_usd_cents":  r.OutstandingUSDCents,
			"voided_line_count":      r.VoidedLineCount,
			"voided_usd_cents":       r.VoidedUSDCents,
			"settlement_by_currency": settle,
		})
	}
	return out
}

func tripDashboardView(d *store.TripDashboard) map[string]any {
	return map[string]any{
		"trip":               tripHeaderView(d.Trip),
		"occupancy":          occupancyView(d.Occupancy),
		"registration_ready": registrationReadyView(d.RegistrationReady),
		"document_ready":     documentReadyView(d.DocumentReady),
		"folio_totals":       folioTotalsView(d.FolioTotals),
		"top_items":          topItemsView(d.TopItems),
		"low_stock":          lowStockView(d.LowStock),
	}
}

func tripHeaderView(h store.TripHeader) map[string]any {
	return map[string]any{
		"trip_id":        h.TripID,
		"boat_id":        h.BoatID,
		"boat_name":      h.BoatName,
		"start_date":     h.StartDate.Format("2006-01-02"),
		"end_date":       h.EndDate.Format("2006-01-02"),
		"itinerary":      h.Itinerary,
		"departure_port": h.DeparturePort,
		"return_port":    h.ReturnPort,
		"status":         h.Status,
		"started_at":     h.StartedAt,
		"completed_at":   h.CompletedAt,
		"cancelled_at":   h.CancelledAt,
		"director_count": h.DirectorCount,
	}
}

func occupancyView(o store.Occupancy) map[string]any {
	return map[string]any{
		"num_guests":        o.NumGuests,
		"guest_count":       o.GuestCount,
		"cabin_assignments": o.CabinAssignments,
		"berths_total":      o.BerthsTotal,
	}
}

func registrationReadyView(r store.RegistrationReadiness) map[string]any {
	return map[string]any{
		"submitted_count": r.SubmittedCount,
		"pending_count":   r.PendingCount,
		"guest_count":     r.GuestCount,
	}
}

func documentReadyView(d store.DocumentReadiness) map[string]any {
	return map[string]any{
		"uploaded_count":         d.UploadedCount,
		"guests_with_docs_count": d.GuestsWithDocsCount,
		"guest_count":            d.GuestCount,
	}
}

func folioTotalsView(f store.FolioTotals) map[string]any {
	return map[string]any{
		"open_count":            f.OpenCount,
		"closed_count":          f.ClosedCount,
		"charges_usd_cents":     f.ChargesUSDCents,
		"crew_tip_usd_cents":    f.CrewTipUSDCents,
		"card_fee_usd_cents":    f.CardFeeUSDCents,
		"settled_usd_cents":     f.SettledUSDCents,
		"outstanding_usd_cents": f.OutstandingUSDCents,
		"voided_line_count":     f.VoidedLineCount,
		"voided_usd_cents":      f.VoidedUSDCents,
	}
}

func topItemsView(rows []store.TripTopItemRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"catalog_item_id": r.CatalogItemID,
			"item_name":       r.ItemName,
			"quantity":        r.Quantity,
			"usd_cents":       r.USDCents,
		})
	}
	return out
}

func lowStockView(rows []store.LowStockRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"catalog_item_id":  r.CatalogItemID,
			"item_name":        r.ItemName,
			"category_name":    r.CategoryName,
			"quantity_on_hand": r.QuantityOnHand,
			"reorder_level":    r.ReorderLevel,
			"par_level":        r.ParLevel,
		})
	}
	return out
}

func guestTabView(t *store.GuestTab) map[string]any {
	lines := make([]map[string]any, 0, len(t.Lines))
	for _, l := range t.Lines {
		lines = append(lines, map[string]any{
			"id":                   l.ID,
			"line_type":            l.LineType,
			"item_name":            l.ItemName,
			"quantity":             l.Quantity,
			"unit_price_usd_cents": l.UnitPriceUSDCents,
			"line_total_usd_cents": l.LineTotalUSDCents,
			"created_at":           l.CreatedAt,
		})
	}
	out := map[string]any{
		"trip":               tripHeaderView(t.Trip),
		"has_folio":          t.HasFolio,
		"status":             t.Status,
		"lines":              lines,
		"subtotal_usd_cents": t.Subtotal,
		"card_fee_usd_cents": t.CardFee,
		"total_usd_cents":    t.TotalUSD,
	}
	if t.Settlement != nil {
		out["settlement"] = map[string]any{
			"currency":       t.Settlement.Currency,
			"total_minor":    t.Settlement.TotalMinor,
			"currency_exp":   t.Settlement.CurrencyExp,
			"payment_method": t.Settlement.PaymentMethod,
			"closed_at":      t.Settlement.ClosedAt,
		}
	}
	return out
}
