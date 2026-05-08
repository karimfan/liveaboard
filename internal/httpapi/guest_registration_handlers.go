package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

type acceptGuestInviteReq struct {
	Password string `json:"password"`
}

func (s *Server) handleLookupGuestInvitation(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	lookup, err := s.Auth.LookupGuestInvite(r.Context(), token)
	if err != nil {
		writeGuestServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, guestInviteLookupView(lookup))
}

func (s *Server) handleAcceptGuestInvitation(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	var req acceptGuestInviteReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := s.Auth.AcceptGuestInvite(r.Context(), token, req.Password)
	if err != nil {
		var lockout *auth.LockoutError
		if errors.As(err, &lockout) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":           "rate_limited",
				"message":         "Too many failed attempts.",
				"retry_after_sec": int(lockout.RetryAfter.Seconds()),
			})
			return
		}
		writeGuestServiceError(w, err)
		return
	}
	auth.SetGuestSessionCookie(w, res.Token, res.ExpiresAt, s.CookieSecure)
	writeJSON(w, http.StatusOK, map[string]any{
		"guest":         guestUserView(res.Guest),
		"trip_guest_id": res.TripGuest.ID,
	})
}

func (s *Server) handleGuestLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.GuestSessionCookieName); err == nil {
		_ = s.Auth.LogoutGuest(r.Context(), c.Value)
	}
	auth.ClearGuestSessionCookie(w, s.CookieSecure)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetGuestRegistration(w http.ResponseWriter, r *http.Request) {
	guest, tripGuest, ok := s.guestRegistrationAccess(w, r)
	if !ok {
		return
	}
	reg, err := s.Auth.Store.GuestRegistrationByTripGuest(r.Context(), tripGuest.ID)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{
			"trip_guest": tripGuestView(tripGuest, "account_created"),
			"guest":      guestUserView(guest),
			"status":     "draft",
			"payload":    map[string]any{},
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, registrationView(reg))
}

func (s *Server) handleSaveGuestRegistration(w http.ResponseWriter, r *http.Request) {
	guest, tripGuest, ok := s.guestRegistrationAccess(w, r)
	if !ok {
		return
	}
	// Once a guest has submitted, drafts are no longer the right gesture:
	// any further edit must re-validate via /submit so the persisted state
	// stays a fully-valid submission. Reject PATCH from this state.
	existing, err := s.Auth.Store.GuestRegistrationByTripGuest(r.Context(), tripGuest.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if existing != nil && existing.Status == "submitted" {
		writeError(w, http.StatusConflict, "already_submitted", "Registration is already submitted; use submit to save further edits.")
		return
	}
	payload, ok := readRegistrationPayload(w, r, false)
	if !ok {
		return
	}
	reg, err := s.Auth.Store.SaveGuestRegistration(r.Context(), tripGuest, guest.ID, payload, "draft", nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	s.recordGuestAudit(r.Context(), tripGuest.OrganizationID, guest.ID, "guest.registration_saved", "guest_registration", &reg.ID, &tripGuest.TripID, &tripGuest.ID, map[string]any{"status": reg.Status})
	writeJSON(w, http.StatusOK, registrationView(reg))
}

func (s *Server) handleSubmitGuestRegistration(w http.ResponseWriter, r *http.Request) {
	guest, tripGuest, ok := s.guestRegistrationAccess(w, r)
	if !ok {
		return
	}
	payload, ok := readRegistrationPayload(w, r, true)
	if !ok {
		return
	}
	// Re-submits keep the original submission timestamp. The store also
	// COALESCEs trip_guests.registration_submitted_at on the same key, but
	// passing the existing value here keeps the registration row honest.
	submittedAt := time.Now().UTC()
	if existing, err := s.Auth.Store.GuestRegistrationByTripGuest(r.Context(), tripGuest.ID); err == nil && existing.SubmittedAt != nil {
		submittedAt = *existing.SubmittedAt
	} else if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	reg, err := s.Auth.Store.SaveGuestRegistration(r.Context(), tripGuest, guest.ID, payload, "submitted", &submittedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	s.recordGuestAudit(r.Context(), tripGuest.OrganizationID, guest.ID, "guest.registration_submitted", "guest_registration", &reg.ID, &tripGuest.TripID, &tripGuest.ID, map[string]any{"status": reg.Status})
	writeJSON(w, http.StatusOK, registrationView(reg))
}

func (s *Server) guestRegistrationAccess(w http.ResponseWriter, r *http.Request) (*store.GuestUser, *store.TripGuest, bool) {
	guest := auth.GuestFromContext(r.Context())
	if guest == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "guest session required")
		return nil, nil, false
	}
	raw := chi.URLParam(r, "trip_guest_id")
	tripGuestID, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "trip_guest_id must be a uuid")
		return nil, nil, false
	}
	tripGuest, err := s.Auth.Store.AssertTripGuestAccess(r.Context(), guest.ID, tripGuestID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "registration not found")
		return nil, nil, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return nil, nil, false
	}
	return guest, tripGuest, true
}

func readRegistrationPayload(w http.ResponseWriter, r *http.Request, final bool) ([]byte, bool) {
	var payload map[string]any
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	if err := dec.Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return nil, false
	}
	if final {
		if err := validateRegistrationPayload(payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
			return nil, false
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "registration payload is invalid")
		return nil, false
	}
	return raw, true
}

func validateRegistrationPayload(p map[string]any) error {
	required := map[string][]string{
		"identity":          {"legal_name", "date_of_birth", "nationality"},
		"emergency_contact": {"name", "phone"},
		"dive_profile":      {"certification_agency", "certification_level"},
	}
	for section, fields := range required {
		obj, ok := p[section].(map[string]any)
		if !ok {
			return errors.New(section + " is required")
		}
		for _, field := range fields {
			if strings.TrimSpace(asString(obj[field])) == "" {
				return errors.New(section + "." + field + " is required")
			}
		}
	}
	if obj, ok := p["dive_profile"].(map[string]any); ok {
		if _, ok := obj["logged_dives"].(float64); !ok {
			return errors.New("dive_profile.logged_dives is required")
		}
	}
	if obj, ok := p["dietary"].(map[string]any); !ok || obj["no_dietary_or_allergy_notes"] != true && strings.TrimSpace(asString(obj["dietary_requirements"])) == "" && strings.TrimSpace(asString(obj["allergies"])) == "" {
		return errors.New("dietary acknowledgement or notes are required")
	}
	return nil
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func guestInviteLookupView(v *auth.GuestInviteLookup) map[string]any {
	return map[string]any{
		"trip_guest_id":     v.TripGuestID,
		"email":             v.Email,
		"full_name":         v.FullName,
		"organization_name": v.OrganizationName,
		"boat_name":         v.BoatName,
		"itinerary":         v.Itinerary,
		"start_date":        v.StartDate.Format("2006-01-02"),
		"end_date":          v.EndDate.Format("2006-01-02"),
		"expires_at":        v.ExpiresAt,
	}
}

func guestUserView(g *store.GuestUser) map[string]any {
	return map[string]any{
		"id":    g.ID,
		"email": g.Email,
	}
}

func registrationView(r *store.GuestTripRegistration) map[string]any {
	var payload any = map[string]any{}
	if len(r.Payload) > 0 {
		_ = json.Unmarshal(r.Payload, &payload)
	}
	return map[string]any{
		"id":            r.ID,
		"trip_guest_id": r.TripGuestID,
		"status":        r.Status,
		"payload":       payload,
		"submitted_at":  r.SubmittedAt,
		"created_at":    r.CreatedAt,
		"updated_at":    r.UpdatedAt,
	}
}
