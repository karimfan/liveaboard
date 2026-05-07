package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

func (s *Server) handleListFXRates(w http.ResponseWriter, r *http.Request) {
	rates, err := s.Auth.Store.LatestExchangeRates(r.Context(), time.Now().UTC())
	if err != nil {
		s.Log.Error("list fx rates", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := make([]map[string]any, 0, len(rates))
	for _, r := range rates {
		out = append(out, fxRateView(r))
	}
	writeJSON(w, http.StatusOK, map[string]any{"rates": out})
}

type upsertFXRateReq struct {
	Provider        string `json:"provider"`
	BaseCurrency    string `json:"base_currency"`
	QuoteCurrency   string `json:"quote_currency"`
	RateNumerator   int64  `json:"rate_numerator"`
	RateDenominator int64  `json:"rate_denominator"`
	AsOf            string `json:"as_of"`
	ExpiresAt       string `json:"expires_at"`
}

func (s *Server) handleCreateFXRate(w http.ResponseWriter, r *http.Request) {
	var req upsertFXRateReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	asOf, err := time.Parse(time.RFC3339, req.AsOf)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "as_of must be RFC3339")
		return
	}
	expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "expires_at must be RFC3339")
		return
	}
	rate, err := s.Auth.Store.UpsertExchangeRate(r.Context(), req.Provider, req.BaseCurrency, req.QuoteCurrency, req.RateNumerator, req.RateDenominator, asOf, expiresAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, fxRateView(rate))
}

type checkoutQuoteReq struct {
	TargetCurrency    string         `json:"target_currency"`
	SourceAmountCents *int64         `json:"source_amount_cents,omitempty"`
	Lines             []quoteLineReq `json:"lines,omitempty"`
}

type quoteLineReq struct {
	CatalogItemID string `json:"catalog_item_id"`
	Quantity      int    `json:"quantity"`
}

func (s *Server) handleCheckoutQuote(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req checkoutQuoteReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	lines := make([]store.QuoteLineInput, 0, len(req.Lines))
	for _, line := range req.Lines {
		id, err := uuid.Parse(line.CatalogItemID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "catalog_item_id must be a uuid")
			return
		}
		lines = append(lines, store.QuoteLineInput{CatalogItemID: id, Quantity: line.Quantity})
	}
	q, err := s.Auth.Store.CreateCheckoutQuote(r.Context(), store.CreateCheckoutQuoteInput{
		OrganizationID:    u.OrganizationID,
		RequestedBy:       &u.ID,
		TargetCurrency:    req.TargetCurrency,
		SourceAmountCents: req.SourceAmountCents,
		Lines:             lines,
		Now:               time.Now().UTC(),
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "missing_rate_or_item", "exchange rate or catalog item not found")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, checkoutQuoteView(q))
}

func fxRateView(r *store.ExchangeRate) map[string]any {
	return map[string]any{
		"id":               r.ID,
		"provider":         r.Provider,
		"base_currency":    r.BaseCurrency,
		"quote_currency":   r.QuoteCurrency,
		"rate_numerator":   r.RateNumerator,
		"rate_denominator": r.RateDenominator,
		"as_of":            r.AsOf,
		"fetched_at":       r.FetchedAt,
		"expires_at":       r.ExpiresAt,
	}
}

func checkoutQuoteView(q *store.CheckoutQuote) map[string]any {
	lines := make([]map[string]any, 0, len(q.Lines))
	for _, l := range q.Lines {
		lines = append(lines, map[string]any{
			"id":                   l.ID,
			"catalog_item_id":      l.CatalogItemID,
			"item_name":            l.ItemName,
			"quantity":             l.Quantity,
			"unit_price_usd_cents": l.UnitPriceUSDCents,
			"line_total_usd_cents": l.LineTotalUSDCents,
			"sort_order":           l.SortOrder,
		})
	}
	return map[string]any{
		"id":                  q.ID,
		"source_currency":     q.SourceCurrency,
		"target_currency":     q.TargetCurrency,
		"source_amount_cents": q.SourceAmountCents,
		"target_amount_minor": q.TargetAmountMinor,
		"currency_exponent":   q.CurrencyExponent,
		"rate_provider":       q.RateProvider,
		"rate_numerator":      q.RateNumerator,
		"rate_denominator":    q.RateDenominator,
		"rate_as_of":          q.RateAsOf,
		"expires_at":          q.ExpiresAt,
		"created_at":          q.CreatedAt,
		"lines":               lines,
	}
}
