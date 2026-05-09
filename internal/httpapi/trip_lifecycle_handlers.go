package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

type lifecycleTransitionReq struct {
	AcknowledgedWarnings []string `json:"acknowledged_warnings"`
	Reason               string   `json:"reason"`
}

type cancelTripReq struct {
	Reason string `json:"reason"`
}

func (s *Server) handleTripLifecycle(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	trip, ok := s.authorizeManifestAccess(w, r, u, tripID)
	if !ok {
		return
	}
	readiness, err := s.Auth.Store.TripReadiness(r.Context(), u.OrganizationID, tripID, time.Now().UTC())
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"trip":      tripLifecycleTripView(trip),
		"readiness": readiness,
	})
}

func (s *Server) handleStartTrip(w http.ResponseWriter, r *http.Request) {
	s.handleLifecycleTransition(w, r, "start")
}

func (s *Server) handleCompleteTrip(w http.ResponseWriter, r *http.Request) {
	s.handleLifecycleTransition(w, r, "complete")
}

func (s *Server) handleLifecycleTransition(w http.ResponseWriter, r *http.Request, transition string) {
	u := auth.UserFromContext(r.Context())
	tripID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	trip, err := s.Auth.Store.TripByID(r.Context(), u.OrganizationID, tripID)
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	var req lifecycleTransitionReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	override, ok := s.authorizeLifecycleTransition(w, r, u, trip)
	if !ok {
		return
	}
	overrideRole := ""
	if override {
		overrideRole = string(u.Role)
	}
	in := store.TripLifecycleTransitionInput{
		ActorUserID:          u.ID,
		OverrideUsed:         override,
		OverrideRole:         overrideRole,
		Reason:               req.Reason,
		AcknowledgedWarnings: req.AcknowledgedWarnings,
		Now:                  time.Now().UTC(),
	}
	var updated *store.Trip
	if transition == "start" {
		updated, err = s.Auth.Store.StartTrip(r.Context(), u.OrganizationID, tripID, in)
	} else {
		updated, err = s.Auth.Store.CompleteTrip(r.Context(), u.OrganizationID, tripID, in)
	}
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tripLifecycleTripView(updated))
}

func (s *Server) handleCancelTrip(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u.Role != store.RoleOrgAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "org admin role required")
		return
	}
	tripID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	var req cancelTripReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	trip, err := s.Auth.Store.CancelTrip(r.Context(), u.OrganizationID, tripID, u.ID, req.Reason, time.Now().UTC())
	if err != nil {
		writeLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tripLifecycleTripView(trip))
}

func (s *Server) authorizeLifecycleTransition(w http.ResponseWriter, r *http.Request, u *store.User, trip *store.Trip) (bool, bool) {
	if u.Role == store.RoleOrgAdmin {
		return true, true
	}
	if u.Role == store.RoleCruiseDirector {
		ok, err := s.Auth.Store.UserAssignedToTrip(r.Context(), trip.ID, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return false, false
		}
		if ok {
			return false, true
		}
	}
	writeError(w, http.StatusForbidden, "forbidden", "trip is not assigned to you")
	return false, false
}

func tripLifecycleTripView(t *store.Trip) map[string]any {
	return map[string]any{
		"id":                     t.ID,
		"status":                 t.Status,
		"started_at":             t.StartedAt,
		"started_by_user_id":     t.StartedByUserID,
		"completed_at":           t.CompletedAt,
		"completed_by_user_id":   t.CompletedByUserID,
		"cancelled_at":           t.CancelledAt,
		"cancelled_by_user_id":   t.CancelledByUserID,
		"cancellation_reason":    t.CancellationReason,
		"removed_from_source_at": t.RemovedFromSourceAt,
	}
}

func writeLifecycleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, store.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", "reason is required and must be 500 characters or fewer")
	case errors.Is(err, store.ErrTripTransition):
		writeError(w, http.StatusConflict, "invalid_transition", "trip cannot transition in its current state")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}

func isTripOperationallyClosed(t *store.Trip) bool {
	return t.Status == store.TripStatusCompleted || t.Status == store.TripStatusCancelled
}
