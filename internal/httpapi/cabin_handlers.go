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

type cabinAssignReq struct {
	BerthID string  `json:"berth_id"`
	Notes   *string `json:"notes"`
}

func (s *Server) handleGetBoatCabins(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	boatID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	if !s.authorizeBoatLayoutAccess(w, r, u, boatID) {
		return
	}
	layout, err := s.Auth.Store.BoatCabinLayout(r.Context(), u.OrganizationID, boatID)
	if err != nil {
		writeCabinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cabinLayoutView(layout))
}

func (s *Server) handlePreviewBoatCabins(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	boatID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	if !s.authorizeBoatLayoutAccess(w, r, u, boatID) {
		return
	}
	var in store.CabinLayoutInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	preview, err := s.Auth.Store.PreviewCabinLayout(r.Context(), u.OrganizationID, boatID, in)
	if err != nil {
		writeCabinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) handleReplaceBoatCabins(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	boatID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	if !s.authorizeBoatLayoutAccess(w, r, u, boatID) {
		return
	}
	var in store.CabinLayoutInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	layout, err := s.Auth.Store.ReplaceBoatCabinLayout(r.Context(), u.OrganizationID, boatID, u.ID, in)
	if err != nil {
		writeCabinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cabinLayoutView(layout))
}

func (s *Server) handlePatchBoatCabin(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	boatID, cabinID, ok := boatAndCabinParams(w, r)
	if !ok {
		return
	}
	if !s.authorizeBoatLayoutAccess(w, r, u, boatID) {
		return
	}
	var in store.CabinInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	cabin, err := s.Auth.Store.UpdateBoatCabin(r.Context(), u.OrganizationID, boatID, cabinID, u.ID, in)
	if err != nil {
		writeCabinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cabinView(cabin))
}

func (s *Server) handleDeleteBoatCabin(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	boatID, cabinID, ok := boatAndCabinParams(w, r)
	if !ok {
		return
	}
	if !s.authorizeBoatLayoutAccess(w, r, u, boatID) {
		return
	}
	if err := s.Auth.Store.DeactivateBoatCabin(r.Context(), u.OrganizationID, boatID, cabinID, u.ID); err != nil {
		writeCabinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handlePatchBoatBerth(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	boatID, _, berthID, ok := boatCabinBerthParams(w, r)
	if !ok {
		return
	}
	if !s.authorizeBoatLayoutAccess(w, r, u, boatID) {
		return
	}
	var in store.BerthInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	berth, err := s.Auth.Store.UpdateBoatBerth(r.Context(), u.OrganizationID, boatID, berthID, u.ID, in)
	if err != nil {
		writeCabinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, berthView(berth))
}

func (s *Server) handleDeleteBoatBerth(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	boatID, _, berthID, ok := boatCabinBerthParams(w, r)
	if !ok {
		return
	}
	if !s.authorizeBoatLayoutAccess(w, r, u, boatID) {
		return
	}
	if err := s.Auth.Store.DeactivateBoatBerth(r.Context(), u.OrganizationID, boatID, berthID, u.ID); err != nil {
		writeCabinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleTripCabinBoard(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	board, err := s.Auth.Store.TripCabinBoard(r.Context(), u.OrganizationID, tripID, time.Now().UTC())
	if err != nil {
		writeCabinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, board)
}

func (s *Server) handleAssignGuestCabin(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, guestID, ok := tripAndGuestParams(w, r)
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	var req cabinAssignReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	berthID, err := uuid.Parse(req.BerthID)
	if err != nil || berthID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "berth_id must be a uuid")
		return
	}
	assignment, err := s.Auth.Store.AssignTripGuestBerth(r.Context(), u.OrganizationID, tripID, guestID, berthID, u.ID, req.Notes)
	if err != nil {
		writeCabinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cabinAssignmentView(assignment))
}

func (s *Server) handleUnassignGuestCabin(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, guestID, ok := tripAndGuestParams(w, r)
	if !ok {
		return
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return
	}
	if err := s.Auth.Store.UnassignTripGuestBerth(r.Context(), u.OrganizationID, tripID, guestID, u.ID); err != nil {
		writeCabinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) authorizeBoatLayoutAccess(w http.ResponseWriter, r *http.Request, u *store.User, boatID uuid.UUID) bool {
	if _, err := s.Auth.Store.BoatByID(r.Context(), u.OrganizationID, boatID); err != nil {
		writeCabinError(w, err)
		return false
	}
	if u.Role == store.RoleOrgAdmin {
		return true
	}
	if u.Role == store.RoleCruiseDirector {
		ok, err := s.Auth.Store.UserAssignedToBoat(r.Context(), u.OrganizationID, boatID, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return false
		}
		if ok {
			return true
		}
	}
	writeError(w, http.StatusForbidden, "forbidden", "boat is not assigned to you")
	return false
}

func boatAndCabinParams(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	boatID, ok := uuidParam(w, r, "id")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	cabinID, err := uuid.Parse(chi.URLParam(r, "cabin_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "cabin_id must be a uuid")
		return uuid.Nil, uuid.Nil, false
	}
	return boatID, cabinID, true
}

func boatCabinBerthParams(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	boatID, cabinID, ok := boatAndCabinParams(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	berthID, err := uuid.Parse(chi.URLParam(r, "berth_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "berth_id must be a uuid")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return boatID, cabinID, berthID, true
}

func writeCabinError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", trimSentinel(err.Error()))
	case errors.Is(err, store.ErrCabinAssignmentConflict):
		writeError(w, http.StatusConflict, "berth_unavailable", "That berth is already assigned on this trip.")
	case errors.Is(err, store.ErrCabinLayoutInUse):
		writeError(w, http.StatusConflict, "layout_in_use", "Cabin layout is in use by active assignments.")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "not found")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}

func cabinLayoutView(l *store.CabinLayout) map[string]any {
	cabins := make([]map[string]any, 0, len(l.Cabins))
	for _, c := range l.Cabins {
		cabins = append(cabins, cabinView(c))
	}
	return map[string]any{
		"boat_id":            l.BoatID,
		"active_cabin_count": l.ActiveCabinCount,
		"active_berth_count": l.ActiveBerthCount,
		"cabins":             cabins,
	}
}

func cabinView(c *store.BoatCabin) map[string]any {
	berths := make([]map[string]any, 0, len(c.Berths))
	for _, b := range c.Berths {
		berths = append(berths, berthView(b))
	}
	return map[string]any{
		"id":         c.ID,
		"boat_id":    c.BoatID,
		"label":      c.Label,
		"deck":       c.Deck,
		"sort_order": c.SortOrder,
		"notes":      c.Notes,
		"is_active":  c.IsActive,
		"berths":     berths,
	}
}

func berthView(b *store.BoatCabinBerth) map[string]any {
	return map[string]any{
		"id":            b.ID,
		"boat_id":       b.BoatID,
		"cabin_id":      b.CabinID,
		"berth_label":   b.BerthLabel,
		"display_label": b.DisplayLabel,
		"sort_order":    b.SortOrder,
		"notes":         b.Notes,
		"is_active":     b.IsActive,
	}
}
