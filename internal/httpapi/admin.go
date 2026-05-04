package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

// AdminHandlers serves /api/admin/* for the Sprint 008 admin chrome.
// Most routes are mounted behind RequireOrgAdmin; the trips list is
// mounted behind RequireSession because Site Directors hit the same
// URL but receive a server-scoped subset.
type AdminHandlers struct {
	Store *store.Pool
}

// HandleOverview returns the aggregate counts the Overview screen
// needs for setup-completeness + a list of trips needing attention.
func (a *AdminHandlers) HandleOverview(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	ctx := r.Context()
	now := time.Now().UTC()

	boatCount, err := a.Store.BoatCountForOrg(ctx, u.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	tripCount, err := a.Store.TripCountForOrg(ctx, u.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	directorCount, err := a.Store.CountActiveUsersByRole(ctx, u.OrganizationID, store.RoleSiteDirector)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	org, err := a.Store.OrganizationByID(ctx, u.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	attention, err := a.Store.TripsNeedingAttention(ctx, u.OrganizationID, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	boatNames, err := a.boatNamesForOrg(ctx, u.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	currencySet := org.Currency != nil && *org.Currency != ""
	steps := []map[string]any{
		{"key": "currency", "label": "Set organization currency", "done": currencySet, "hint": derefOrEmpty(org.Currency), "href": "/admin/organization"},
		{"key": "boats", "label": "Add or import a boat", "done": boatCount > 0, "hint": pluralize(boatCount, "boat", "boats"), "href": "/admin/fleet"},
		{"key": "directors", "label": "Invite a Site Director", "done": directorCount > 0, "hint": pluralize(directorCount, "active", "active"), "href": "/admin/users"},
		{"key": "trips", "label": "Create your first trip", "done": tripCount > 0, "hint": pluralize(tripCount, "trip", "trips"), "href": "/admin/trips"},
	}
	doneCount := 0
	for _, s := range steps {
		if s["done"].(bool) {
			doneCount++
		}
	}
	pct := 0
	if len(steps) > 0 {
		pct = int(float64(doneCount) / float64(len(steps)) * 100)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"setup": map[string]any{
			"pct":   pct,
			"steps": steps,
		},
		"counts": map[string]any{
			"boats":          boatCount,
			"trips":          tripCount,
			"site_directors": directorCount,
		},
		"trips_needing_attention": tripsToView(attention, boatNames),
	})
}

// HandleListBoats returns every boat in the org.
func (a *AdminHandlers) HandleListBoats(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	boats, err := a.Store.BoatsForOrg(r.Context(), u.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := make([]map[string]any, 0, len(boats))
	for _, b := range boats {
		out = append(out, boatView(b))
	}
	writeJSON(w, http.StatusOK, map[string]any{"boats": out})
}

// HandleGetBoat returns a single boat by id.
func (a *AdminHandlers) HandleGetBoat(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	boatID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	boat, err := a.Store.BoatByID(r.Context(), u.OrganizationID, boatID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "boat not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, boatView(boat))
}

// HandleListBoatTrips returns trips for a single boat.
func (a *AdminHandlers) HandleListBoatTrips(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	boatID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	boat, err := a.Store.BoatByID(r.Context(), u.OrganizationID, boatID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "boat not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	trips, err := a.Store.TripsForBoat(r.Context(), boat.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	directorNames, err := a.directorNamesForOrg(r.Context(), u.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := make([]map[string]any, 0, len(trips))
	for _, t := range trips {
		out = append(out, tripView(t, boat.DisplayName, directorNames))
	}
	writeJSON(w, http.StatusOK, map[string]any{"trips": out})
}

// HandleListTrips returns trips for the org. Site Directors get their
// own assigned trips only; admins get all org trips.
func (a *AdminHandlers) HandleListTrips(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	ctx := r.Context()

	var trips []*store.Trip
	var err error
	switch u.Role {
	case store.RoleOrgAdmin:
		trips, err = a.Store.TripsByOrgInRange(ctx, u.OrganizationID,
			time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC))
	case store.RoleSiteDirector:
		trips, err = a.Store.TripsForUser(ctx, u.OrganizationID, u.ID)
	default:
		writeError(w, http.StatusForbidden, "forbidden", "role cannot list trips")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	boatNames, err := a.boatNamesForOrg(ctx, u.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	directorNames, err := a.directorNamesForOrg(ctx, u.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := make([]map[string]any, 0, len(trips))
	for _, t := range trips {
		out = append(out, tripView(t, boatNames[t.BoatID], directorNames))
	}
	writeJSON(w, http.StatusOK, map[string]any{"trips": out, "scope": tripScopeFor(u.Role)})
}

// HandleAssignDirector PATCHes a trip's site_director_user_id. Pass
// {"site_director_user_id": null} to clear, or a uuid string to assign.
func (a *AdminHandlers) HandleAssignDirector(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	tripID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		SiteDirectorUserID *string `json:"site_director_user_id"`
	}
	if !readJSONInto(w, r, &in) {
		return
	}
	var directorPtr *uuid.UUID
	if in.SiteDirectorUserID != nil && *in.SiteDirectorUserID != "" {
		id, err := uuid.Parse(*in.SiteDirectorUserID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "site_director_user_id must be a uuid")
			return
		}
		// Validate the candidate director belongs to this org.
		director, err := a.Store.UserByID(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) || (err == nil && director.OrganizationID != u.OrganizationID) {
			writeError(w, http.StatusBadRequest, "invalid_input", "user not in this organization")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		directorPtr = &id
	}
	if err := a.Store.AssignSiteDirector(r.Context(), u.OrganizationID, tripID, directorPtr); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "trip not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleListUsers returns every user in the org.
func (a *AdminHandlers) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	users, err := a.Store.UsersForOrg(r.Context(), u.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := make([]map[string]any, 0, len(users))
	for _, x := range users {
		out = append(out, userView(x))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

// HandlePatchOrganization updates the org's name and currency.
func (a *AdminHandlers) HandlePatchOrganization(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var in struct {
		Name     string  `json:"name"`
		Currency *string `json:"currency"`
	}
	if !readJSONInto(w, r, &in) {
		return
	}
	if in.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "name is required")
		return
	}
	org, err := a.Store.UpdateOrganizationProfile(r.Context(), u.OrganizationID, in.Name, in.Currency)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, organizationView(org))
}

// --- helpers ---

func (a *AdminHandlers) boatNamesForOrg(ctx context.Context, orgID uuid.UUID) (map[uuid.UUID]string, error) {
	boats, err := a.Store.BoatsForOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]string, len(boats))
	for _, b := range boats {
		out[b.ID] = b.DisplayName
	}
	return out, nil
}

func (a *AdminHandlers) directorNamesForOrg(ctx context.Context, orgID uuid.UUID) (map[uuid.UUID]string, error) {
	users, err := a.Store.UsersForOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]string, len(users))
	for _, u := range users {
		out[u.ID] = u.FullName
	}
	return out, nil
}

func uuidParam(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	raw := chi.URLParam(r, name)
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "id must be a uuid")
		return uuid.Nil, false
	}
	return id, true
}

func readJSONInto(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return false
	}
	return true
}

func boatView(b *store.Boat) map[string]any {
	return map[string]any{
		"id":          b.ID,
		"slug":        b.SourceSlug,
		"name":        b.DisplayName,
		"source_name": b.SourceName,
		"image_url":   b.SourceImageURL,
		"source_url":  b.SourceURL,
		"last_synced": b.SourceLastSyncedAt,
	}
}

func tripView(t *store.Trip, boatName string, directorNames map[uuid.UUID]string) map[string]any {
	var directorName *string
	if t.SiteDirectorUserID != nil {
		if name, ok := directorNames[*t.SiteDirectorUserID]; ok {
			directorName = &name
		}
	}
	return map[string]any{
		"id":                    t.ID,
		"boat_id":               t.BoatID,
		"boat_name":             boatName,
		"start_date":            t.StartDate.Format("2006-01-02"),
		"end_date":              t.EndDate.Format("2006-01-02"),
		"itinerary":             t.Itinerary,
		"departure_port":        t.DeparturePort,
		"return_port":           t.ReturnPort,
		"price_text":            t.PriceText,
		"availability_text":     t.AvailabilityText,
		"site_director_user_id": t.SiteDirectorUserID,
		"site_director_name":    directorName,
	}
}

func userView(u *store.User) map[string]any {
	return map[string]any{
		"id":        u.ID,
		"email":     u.Email,
		"full_name": u.FullName,
		"role":      u.Role,
		"is_active": u.IsActive,
	}
}

func organizationView(o *store.Organization) map[string]any {
	return map[string]any{
		"id":         o.ID,
		"name":       o.Name,
		"currency":   o.Currency,
		"created_at": o.CreatedAt,
		"updated_at": o.UpdatedAt,
	}
}

func tripsToView(ts []*store.Trip, boatNames map[uuid.UUID]string) []map[string]any {
	out := make([]map[string]any, 0, len(ts))
	for _, t := range ts {
		out = append(out, map[string]any{
			"id":         t.ID,
			"boat_name":  boatNames[t.BoatID],
			"itinerary":  t.Itinerary,
			"start_date": t.StartDate.Format("2006-01-02"),
			"end_date":   t.EndDate.Format("2006-01-02"),
			"reason":     "no director assigned",
		})
	}
	return out
}

func tripScopeFor(role string) string {
	if role == store.RoleSiteDirector {
		return "assigned_to_me"
	}
	return "all"
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func pluralize(n int, sing, plur string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, sing)
	}
	return fmt.Sprintf("%d %s", n, plur)
}
