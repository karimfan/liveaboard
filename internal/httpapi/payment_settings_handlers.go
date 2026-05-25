package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/fxauto"
	"github.com/karimfan/liveaboard/internal/store"
)

type paymentSettingsReq struct {
	DefaultCurrency       string   `json:"default_currency"`
	SupportedCurrencies   []string `json:"supported_currencies"`
	EnabledPaymentMethods []string `json:"enabled_payment_methods"`
	CardFeeBasisPoints    int      `json:"card_fee_basis_points"`
	FolioEmailFooter      *string  `json:"folio_email_footer"`
}

func (s *Server) handleGetPaymentSettings(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	settings, err := s.Auth.Store.PaymentSettings(r.Context(), u.OrganizationID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, s.paymentSettingsView(r.Context(), settings))
}

func (s *Server) handleUpdatePaymentSettings(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req paymentSettingsReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	// Snapshot the org's accepted currencies *before* the update so
	// we can detect newly-added quotes and kick a targeted fetch.
	var prev map[string]struct{}
	if before, err := s.Auth.Store.PaymentSettings(r.Context(), u.OrganizationID, time.Now().UTC()); err == nil && before != nil {
		prev = make(map[string]struct{}, len(before.SupportedCurrencies))
		for _, c := range before.SupportedCurrencies {
			prev[c] = struct{}{}
		}
	}
	settings, err := s.Auth.Store.UpdatePaymentSettings(r.Context(), u.OrganizationID, store.PaymentSettingsInput{
		DefaultCurrency:       req.DefaultCurrency,
		SupportedCurrencies:   req.SupportedCurrencies,
		EnabledPaymentMethods: req.EnabledPaymentMethods,
		CardFeeBasisPoints:    req.CardFeeBasisPoints,
		FolioEmailFooter:      req.FolioEmailFooter,
	}, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	s.recordStaffAudit(r.Context(), u.OrganizationID, u.ID, "organization.payment_settings_updated", "organization_payment_settings", &u.OrganizationID, nil, nil, map[string]any{
		"default_currency":        settings.DefaultCurrency,
		"supported_currencies":    settings.SupportedCurrencies,
		"card_fee_basis_points":   settings.CardFeeBasisPoints,
		"enabled_payment_methods": settings.EnabledPaymentMethods,
	})
	s.maybeKickRefresh(r.Context(), prev, settings.SupportedCurrencies)
	writeJSON(w, http.StatusOK, s.paymentSettingsView(r.Context(), settings))
}

// maybeKickRefresh fires an on-demand fxauto.Refresher run for any
// currency the org just added. Runs in a detached goroutine so the
// PATCH response is never blocked on Frankfurter's latency. No-op
// when the refresher isn't wired (test mode) or nothing was added.
func (s *Server) maybeKickRefresh(_ context.Context, prev map[string]struct{}, after []string) {
	if s.FXRefresher == nil {
		return
	}
	added := make([]string, 0)
	for _, c := range after {
		if c == "USD" {
			continue
		}
		if _, had := prev[c]; !had {
			added = append(added, c)
		}
	}
	if len(added) == 0 {
		return
	}
	go func(only []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.FXRefresher.RefreshOnce(ctx, only)
	}(added)
}

func (s *Server) paymentSettingsView(ctx context.Context, ps *store.PaymentSettings) map[string]any {
	rates := make([]map[string]any, 0, len(ps.RateReadiness))
	for _, r := range ps.RateReadiness {
		v := map[string]any{
			"currency": r.Currency,
			"ready":    r.Ready,
			"status":   r.Status,
		}
		if r.FetchedAt != nil {
			v["fetched_at"] = r.FetchedAt
		}
		if r.Rate != nil {
			v["rate"] = fxRateView(r.Rate)
		}
		rates = append(rates, v)
	}
	out := map[string]any{
		"organization_id":         ps.OrganizationID,
		"default_currency":        ps.DefaultCurrency,
		"supported_currencies":    ps.SupportedCurrencies,
		"enabled_payment_methods": ps.EnabledPaymentMethods,
		"card_fee_basis_points":   ps.CardFeeBasisPoints,
		"folio_email_footer":      ps.FolioEmailFooter,
		"rate_readiness":          rates,
		"created_at":              ps.CreatedAt,
		"updated_at":              ps.UpdatedAt,
	}
	// Per-org auto_refresh_at: most recent Frankfurter fetch among
	// rows for this org's accepted quotes. Skipped on store error;
	// payments page degrades gracefully.
	scope := make([]string, 0, len(ps.SupportedCurrencies))
	for _, c := range ps.SupportedCurrencies {
		if c != "USD" {
			scope = append(scope, c)
		}
	}
	if len(scope) > 0 {
		if t, err := s.Auth.Store.LastFrankfurterRefreshForCurrencies(ctx, fxauto.Provider, scope); err == nil && t != nil {
			out["auto_refresh_at"] = *t
		}
	}
	return out
}
