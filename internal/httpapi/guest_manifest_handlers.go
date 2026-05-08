package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

type addTripGuestReq struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	BerthID  string `json:"berth_id"`
}

func (s *Server) handleTripManifest(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	trip, ok := s.authorizeManifestAccess(w, r, u, tripID)
	if !ok {
		return
	}
	rows, err := s.Auth.Store.TripManifest(r.Context(), u.OrganizationID, tripID, time.Now().UTC())
	if err != nil {
		s.Log.Error("trip manifest", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	summaries, err := s.Auth.Store.TripManifestSummaries(r.Context(), u.OrganizationID, []uuid.UUID{tripID}, time.Now().UTC())
	if err != nil {
		s.Log.Error("trip manifest summary", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	boats, err := s.AdminAPI.boatNamesForOrg(r.Context(), u.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"trip":    tripView(trip, boats[trip.BoatID], nil, nil),
		"summary": manifestSummaryView(summaries[tripID]),
		"guests":  manifestRowsView(rows),
	})
}

func (s *Server) handleAddTripGuest(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	var req addTripGuestReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	berthID, err := uuid.Parse(req.BerthID)
	if err != nil || berthID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "berth_id must be a uuid")
		return
	}
	g, err := s.Auth.InviteTripGuest(r.Context(), u.OrganizationID, auth.InviteTripGuestInput{
		TripID:   tripID,
		ActorID:  u.ID,
		BerthID:  berthID,
		FullName: req.FullName,
		Email:    req.Email,
	})
	if err != nil {
		writeGuestServiceError(w, err)
		return
	}
	rows, _ := s.Auth.Store.TripManifest(r.Context(), u.OrganizationID, tripID, time.Now().UTC())
	for _, row := range rows {
		if row.Guest.ID == g.ID {
			writeJSON(w, http.StatusCreated, manifestRowView(row))
			return
		}
	}
	writeJSON(w, http.StatusCreated, tripGuestView(g, "invited"))
}

func (s *Server) handleResendTripGuestInvite(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, guestID, ok := tripAndGuestParams(w, r)
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	if _, err := s.Auth.ResendTripGuestInvite(r.Context(), u.OrganizationID, tripID, guestID); err != nil {
		writeGuestServiceError(w, err)
		return
	}
	rows, _ := s.Auth.Store.TripManifest(r.Context(), u.OrganizationID, tripID, time.Now().UTC())
	for _, row := range rows {
		if row.Guest.ID == guestID {
			writeJSON(w, http.StatusOK, manifestRowView(row))
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRevokeTripGuestInvite(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, guestID, ok := tripAndGuestParams(w, r)
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	if err := s.Auth.Store.RevokeTripGuestInvite(r.Context(), u.OrganizationID, tripID, guestID, time.Now().UTC()); err != nil {
		writeGuestServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStaffGuestRegistration(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, guestID, ok := tripAndGuestParams(w, r)
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	rows, err := s.Auth.Store.TripManifest(r.Context(), u.OrganizationID, tripID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	var manifestRow *store.TripGuestManifestRow
	for _, row := range rows {
		if row.Guest.ID == guestID {
			manifestRow = row
			break
		}
	}
	if manifestRow == nil {
		writeError(w, http.StatusNotFound, "not_found", "guest not found on this trip")
		return
	}

	reg, err := s.Auth.Store.GuestRegistrationByTripGuest(r.Context(), guestID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	body := map[string]any{
		"trip_guest":   manifestRowView(manifestRow),
		"registration": nil,
	}
	if reg != nil {
		body["registration"] = registrationView(reg)
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) authorizeManifestAccess(w http.ResponseWriter, r *http.Request, u *store.User, tripID uuid.UUID) (*store.Trip, bool) {
	trip, err := s.Auth.Store.TripByID(r.Context(), u.OrganizationID, tripID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "trip not found")
		return nil, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return nil, false
	}
	if u.Role == store.RoleOrgAdmin {
		return trip, true
	}
	if u.Role == store.RoleCruiseDirector {
		ok, err := s.Auth.Store.UserAssignedToTrip(r.Context(), tripID, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return nil, false
		}
		if ok {
			return trip, true
		}
	}
	writeError(w, http.StatusForbidden, "forbidden", "trip is not assigned to you")
	return nil, false
}

func tripAndGuestParams(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	tripID, ok := uuidParam(w, r, "id")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	raw := chi.URLParam(r, "guest_id")
	guestID, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "guest_id must be a uuid")
		return uuid.Nil, uuid.Nil, false
	}
	return tripID, guestID, true
}

func writeGuestServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", trimSentinel(err.Error()))
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials.")
	case errors.Is(err, auth.ErrTokenInvalid):
		writeError(w, http.StatusBadRequest, "token_invalid", "This link is invalid or has expired.")
	case errors.Is(err, store.ErrTripGuestExists):
		writeError(w, http.StatusConflict, "guest_exists", "A guest with this email is already on this trip.")
	case errors.Is(err, store.ErrCabinAssignmentConflict):
		writeError(w, http.StatusConflict, "berth_unavailable", "That berth is already assigned on this trip.")
	case errors.Is(err, store.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", trimSentinel(err.Error()))
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "not found")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}

func manifestRowsView(rows []*store.TripGuestManifestRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, manifestRowView(row))
	}
	return out
}

func manifestRowView(row *store.TripGuestManifestRow) map[string]any {
	v := tripGuestView(row.Guest, row.Status)
	v["invite_expires_at"] = row.InviteExpiresAt
	v["registration_status"] = row.RegistrationStatus
	v["registration_submitted_at"] = row.RegistrationSubmitted
	v["cabin_assignment"] = nil
	if row.CabinAssignment != nil {
		v["cabin_assignment"] = cabinAssignmentView(row.CabinAssignment)
	}
	return v
}

func cabinAssignmentView(a *store.TripCabinAssignment) map[string]any {
	if a == nil {
		return nil
	}
	return map[string]any{
		"id":            a.ID,
		"trip_id":       a.TripID,
		"trip_guest_id": a.TripGuestID,
		"boat_id":       a.BoatID,
		"berth_id":      a.BerthID,
		"cabin_label":   a.CabinLabelSnapshot,
		"berth_label":   a.BerthLabelSnapshot,
		"display_label": a.DisplayLabelSnapshot,
		"assigned_at":   a.AssignedAt,
		"notes":         a.Notes,
	}
}

func tripGuestView(g *store.TripGuest, status string) map[string]any {
	return map[string]any{
		"id":                  g.ID,
		"full_name":           g.FullName,
		"email":               g.Email,
		"status":              status,
		"invite_send_status":  g.InviteSendStatus,
		"invite_last_error":   g.InviteLastError,
		"invite_last_sent_at": g.InviteLastSentAt,
		"account_created_at":  g.AccountCreatedAt,
		"revoked_at":          g.RevokedAt,
	}
}

func manifestSummaryView(s store.TripManifestSummary) map[string]any {
	return map[string]any{
		"guest_count":     s.GuestCount,
		"submitted_count": s.SubmittedCount,
		"expected_count":  s.ExpectedCount,
		"has_warning":     s.HasWarning,
	}
}
