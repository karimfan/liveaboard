package httpapi

import (
	"net/http"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

// HandleCruiseDirectorOverview is the single-payload endpoint that
// powers the Cruise Director landing page. It returns:
//
//   - the caller's contact card (profile),
//   - upcoming/active/past trip counts (stats),
//   - the caller's assigned trips (trips), sorted by start_date asc.
//
// The endpoint is cruise-director-only. Org Admins receive 403; they
// have their own /api/admin/overview.
func (s *Server) handleCruiseDirectorOverview(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u.Role != store.RoleCruiseDirector {
		writeError(w, http.StatusForbidden, "forbidden", "cruise director role required")
		return
	}

	ctx := r.Context()
	org, err := s.Auth.Store.OrganizationByID(ctx, u.OrganizationID)
	if err != nil {
		s.Log.Error("cruise director overview: org lookup", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	trips, err := s.Auth.Store.TripsForUser(ctx, u.OrganizationID, u.ID)
	if err != nil {
		s.Log.Error("cruise director overview: trips lookup", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	boats, err := s.Auth.Store.BoatsForOrg(ctx, u.OrganizationID)
	if err != nil {
		s.Log.Error("cruise director overview: boats lookup", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	boatNames := make(map[string]string, len(boats))
	for _, b := range boats {
		boatNames[b.ID.String()] = b.DisplayName
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	upcoming, active, past := 0, 0, 0
	tripViews := make([]map[string]any, 0, len(trips))
	for _, t := range trips {
		status := classifyTripStatus(t, today)
		switch status {
		case "upcoming":
			upcoming++
		case "active":
			active++
		case "past":
			past++
		}
		tripViews = append(tripViews, map[string]any{
			"id":         t.ID,
			"boat_id":    t.BoatID,
			"boat_name":  boatNames[t.BoatID.String()],
			"itinerary":  t.Itinerary,
			"start_date": t.StartDate.Format("2006-01-02"),
			"end_date":   t.EndDate.Format("2006-01-02"),
			"status":     status,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"profile": map[string]any{
			"id":                u.ID,
			"full_name":         u.FullName,
			"email":             u.Email,
			"phone":             u.Phone,
			"role":              u.Role,
			"organization_name": org.Name,
		},
		"stats": map[string]any{
			"upcoming": upcoming,
			"active":   active,
			"past":     past,
		},
		"trips": tripViews,
	})
}

// classifyTripStatus buckets a trip into upcoming / active / past
// relative to `today` (UTC, midnight-aligned). Trip dates are stored
// as DATE in Postgres; the comparison is purely calendar-based.
//
//   - active : start_date <= today <= end_date
//   - past   : end_date < today
//   - upcoming: start_date > today
func classifyTripStatus(t *store.Trip, today time.Time) string {
	if t.EndDate.Before(today) {
		return "past"
	}
	if t.StartDate.After(today) {
		return "upcoming"
	}
	return "active"
}
