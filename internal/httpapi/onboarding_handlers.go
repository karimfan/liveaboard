package httpapi

import (
	"net/http"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

// handleGetOnboarding returns the unified onboarding payload for the
// current org. Includes the dismissed timestamp, the four wizard
// steps with done flags, the full SetupCompleteness percent (for
// Overview's banner), and the boats-without-layouts list so the
// wizard's layouts step can render without a second round-trip.
func (s *Server) handleGetOnboarding(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	state, err := s.Auth.Store.OnboardingState(r.Context(), u.OrganizationID)
	if err != nil {
		s.Log.Error("onboarding state", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, onboardingStateView(state))
}

// handleDismissOnboarding marks the org as having dismissed the
// onboarding wizard. Idempotent — the store-level COALESCE preserves
// the original timestamp on repeat calls. The audit event records
// only the first dismissal (subsequent calls return the prior
// timestamp without writing the event again).
func (s *Server) handleDismissOnboarding(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	previous, err := s.Auth.Store.OrganizationByID(r.Context(), u.OrganizationID)
	if err != nil {
		s.Log.Error("onboarding dismiss: lookup", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	now := time.Now().UTC()
	if _, err := s.Auth.Store.DismissOrganizationOnboarding(r.Context(), u.OrganizationID, now); err != nil {
		s.Log.Error("onboarding dismiss", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if previous.OnboardingDismissedAt == nil {
		s.recordStaffAudit(r.Context(), u.OrganizationID, u.ID,
			"organization.onboarding_dismissed", "organization",
			&u.OrganizationID, nil, nil, nil)
	}
	state, err := s.Auth.Store.OnboardingState(r.Context(), u.OrganizationID)
	if err != nil {
		s.Log.Error("onboarding state post-dismiss", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, onboardingStateView(state))
}

func onboardingStateView(s *store.OnboardingState) map[string]any {
	steps := make([]map[string]any, 0, len(s.Steps))
	for _, step := range s.Steps {
		steps = append(steps, map[string]any{
			"key":   step.Key,
			"label": step.Label,
			"done":  step.Done,
			"hint":  step.Hint,
		})
	}
	boats := make([]map[string]any, 0, len(s.BoatsWithoutLayouts))
	for _, b := range s.BoatsWithoutLayouts {
		boats = append(boats, map[string]any{
			"boat_id":   b.BoatID,
			"boat_name": b.BoatName,
		})
	}
	return map[string]any{
		"dismissed_at":          s.DismissedAt,
		"onboarding_complete":   s.OnboardingComplete,
		"setup_pct":             s.SetupPercent,
		"steps":                 steps,
		"boats_without_layouts": boats,
	}
}
