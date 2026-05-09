package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/email"
	"github.com/karimfan/liveaboard/internal/store"
)

// cruise_director_assign.go: Sprint 013 — assign / unassign Cruise
// Directors on a trip and notify the affected user by email.
//
// Replaces the Sprint 008 PATCH-with-single-uuid endpoint. Each trip
// can now carry multiple directors via the trip_cruise_directors
// join table. Add and remove are separate routes so the UI can
// dispatch one network call per chip change.

type assignCruiseDirectorReq struct {
	UserID string `json:"user_id"`
}

// handleAssignCruiseDirector: POST /api/admin/trips/{id}/cruise-directors
// Body: {user_id}. Idempotent — re-assigning a user already on the
// trip is a no-op (no duplicate email). Validates that the trip and
// the candidate director both belong to the calling admin's org.
// On success, sends a `trip_assigned` email and returns the trip's
// updated director list.
func (s *Server) handleAssignCruiseDirector(w http.ResponseWriter, r *http.Request) {
	admin := auth.UserFromContext(r.Context())
	tripID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}

	var req assignCruiseDirectorReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	directorID, err := uuid.Parse(strings.TrimSpace(req.UserID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "user_id must be a uuid")
		return
	}

	trip, err := s.tripForOrg(r.Context(), admin.OrganizationID, tripID)
	if err != nil {
		writeTripLookupError(w, err)
		return
	}
	if isTripOperationallyClosed(trip) {
		writeError(w, http.StatusConflict, "trip_closed", "trip is completed or cancelled")
		return
	}
	director, err := s.cruiseDirectorForOrg(r.Context(), admin.OrganizationID, directorID)
	if err != nil {
		writeDirectorLookupError(w, err)
		return
	}

	added, err := s.Auth.Store.AssignCruiseDirector(r.Context(), trip.ID, director.ID, &admin.ID)
	if err != nil {
		s.Log.Error("assign cruise director", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if added {
		// Best-effort email; don't fail the request if mail send
		// errors. Log and move on so the operator's UI reflects
		// the assignment.
		if err := s.sendTripAssignmentEmail(r.Context(), email.KindTripAssigned, director, admin, trip); err != nil {
			s.Log.Error("trip assigned email", "err", err, "user_id", director.ID, "trip_id", trip.ID)
		}
	}
	s.writeTripDirectorsView(w, r.Context(), trip)
}

// handleUnassignCruiseDirector: DELETE /api/admin/trips/{id}/cruise-directors/{user_id}
// Idempotent — removing a user not currently on the trip is a no-op.
// Sends a `trip_unassigned` email only when a row was actually
// deleted.
func (s *Server) handleUnassignCruiseDirector(w http.ResponseWriter, r *http.Request) {
	admin := auth.UserFromContext(r.Context())
	tripID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	userIDStr := chi.URLParam(r, "user_id")
	directorID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "user_id must be a uuid")
		return
	}

	trip, err := s.tripForOrg(r.Context(), admin.OrganizationID, tripID)
	if err != nil {
		writeTripLookupError(w, err)
		return
	}
	if isTripOperationallyClosed(trip) {
		writeError(w, http.StatusConflict, "trip_closed", "trip is completed or cancelled")
		return
	}
	director, err := s.cruiseDirectorForOrg(r.Context(), admin.OrganizationID, directorID)
	if err != nil {
		writeDirectorLookupError(w, err)
		return
	}

	removed, err := s.Auth.Store.UnassignCruiseDirector(r.Context(), trip.ID, director.ID)
	if err != nil {
		s.Log.Error("unassign cruise director", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if removed {
		if err := s.sendTripAssignmentEmail(r.Context(), email.KindTripUnassigned, director, admin, trip); err != nil {
			s.Log.Error("trip unassigned email", "err", err, "user_id", director.ID, "trip_id", trip.ID)
		}
	}
	s.writeTripDirectorsView(w, r.Context(), trip)
}

// --- helpers ---

// tripForOrg loads a trip and verifies it belongs to orgID. Returns
// store.ErrNotFound if the trip is missing or in another org.
func (s *Server) tripForOrg(ctx context.Context, orgID, tripID uuid.UUID) (*store.Trip, error) {
	t, err := s.Auth.Store.TripByID(ctx, orgID, tripID)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// cruiseDirectorForOrg loads a user and verifies they belong to the
// org and have the cruise_director role + are active. Returns
// store.ErrNotFound when any check fails so the handler can map all
// rejection cases to the same 400 (no enumeration).
func (s *Server) cruiseDirectorForOrg(ctx context.Context, orgID, userID uuid.UUID) (*store.User, error) {
	u, err := s.Auth.Store.UserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.OrganizationID != orgID || u.Role != store.RoleCruiseDirector || !u.IsActive {
		return nil, store.ErrNotFound
	}
	return u, nil
}

func writeTripLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "trip not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal", "internal error")
}

func writeDirectorLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "invalid_input", "user is not an active cruise director in this organization")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal", "internal error")
}

// writeTripDirectorsView writes the trip's full director list as the
// response, so the SPA can patch the row in place without an extra
// fetch. Mirrors the shape inside `tripView`.
func (s *Server) writeTripDirectorsView(w http.ResponseWriter, ctx context.Context, trip *store.Trip) {
	ids, err := s.Auth.Store.CruiseDirectorIDsForTrip(ctx, trip.ID)
	if err != nil {
		s.Log.Error("load trip directors", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	idStrs := make([]string, 0, len(ids))
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		idStrs = append(idStrs, id.String())
		// One UserByID per director is acceptable here — trips
		// rarely have more than 2-3 directors and this endpoint
		// fires only on assignment changes, not on every render.
		if u, err := s.Auth.Store.UserByID(ctx, id); err == nil {
			names = append(names, u.FullName)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"trip_id":                  trip.ID,
		"cruise_director_user_ids": idStrs,
		"cruise_director_names":    names,
	})
}

// sendTripAssignmentEmail renders + sends one of the two trip-related
// emails. The recipient is the affected director; the inviter is the
// admin who changed the assignment.
func (s *Server) sendTripAssignmentEmail(
	ctx context.Context,
	kind email.Kind,
	director *store.User,
	admin *store.User,
	trip *store.Trip,
) error {
	boat, err := s.Auth.Store.BoatByID(ctx, director.OrganizationID, trip.BoatID)
	if err != nil {
		return fmt.Errorf("load boat: %w", err)
	}
	link := fmt.Sprintf("%s/admin/trips", strings.TrimRight(s.Auth.AppBaseURL, "/"))
	msg, err := email.Render(kind, email.Vars{
		AppName:        "Liveaboard",
		RecipientEmail: director.Email,
		RecipientName:  director.FullName,
		InviterName:    admin.FullName,
		ActionURL:      link,
		TripBoatName:   boat.DisplayName,
		TripItinerary:  trip.Itinerary,
		TripStartDate:  trip.StartDate,
		TripEndDate:    trip.EndDate,
	})
	if err != nil {
		return err
	}
	msg.From = s.Auth.SenderFrom
	msg.To = director.Email
	return s.Auth.Email.Send(ctx, msg)
}
