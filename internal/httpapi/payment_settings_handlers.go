package httpapi

import (
	"net/http"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
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
	writeJSON(w, http.StatusOK, paymentSettingsView(settings))
}

func (s *Server) handleUpdatePaymentSettings(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req paymentSettingsReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
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
	writeJSON(w, http.StatusOK, paymentSettingsView(settings))
}

func paymentSettingsView(s *store.PaymentSettings) map[string]any {
	rates := make([]map[string]any, 0, len(s.RateReadiness))
	for _, r := range s.RateReadiness {
		v := map[string]any{
			"currency": r.Currency,
			"ready":    r.Ready,
		}
		if r.Rate != nil {
			v["rate"] = fxRateView(r.Rate)
		}
		rates = append(rates, v)
	}
	return map[string]any{
		"organization_id":         s.OrganizationID,
		"default_currency":        s.DefaultCurrency,
		"supported_currencies":    s.SupportedCurrencies,
		"enabled_payment_methods": s.EnabledPaymentMethods,
		"card_fee_basis_points":   s.CardFeeBasisPoints,
		"folio_email_footer":      s.FolioEmailFooter,
		"rate_readiness":          rates,
		"created_at":              s.CreatedAt,
		"updated_at":              s.UpdatedAt,
	}
}
