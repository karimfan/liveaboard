package store

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	PaymentMethodCard  = "card"
	PaymentMethodCash  = "cash"
	PaymentMethodOther = "other"
)

var allowedPaymentMethods = []string{PaymentMethodCard, PaymentMethodCash, PaymentMethodOther}

type PaymentSettings struct {
	OrganizationID        uuid.UUID
	DefaultCurrency       string
	SupportedCurrencies   []string
	EnabledPaymentMethods []string
	CardFeeBasisPoints    int
	FolioEmailFooter      *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	RateReadiness         []PaymentCurrencyRateStatus
}

// PaymentCurrencyRateStatus is the per-currency rate snapshot
// surfaced on the Payments page. Sprint 024 adds Status and
// FetchedAt computed from the row's fetched_at (not as_of, because
// Frankfurter publishes daily with weekend/holiday gaps — the
// operator-visible freshness is "is the automation still alive"
// rather than "how old is the underlying market datum"). Ready
// remains in place for back-compat: any non-expired rate row
// (Frankfurter, manual fallback, anything) still gates checkout.
type PaymentCurrencyRateStatus struct {
	Currency  string
	Ready     bool
	Status    string // "fresh" | "stale" | "missing"
	Rate      *ExchangeRate
	FetchedAt *time.Time
}

type PaymentSettingsInput struct {
	DefaultCurrency       string
	SupportedCurrencies   []string
	EnabledPaymentMethods []string
	CardFeeBasisPoints    int
	FolioEmailFooter      *string
}

func scanPaymentSettings(row interface{ Scan(dest ...any) error }, s *PaymentSettings) error {
	return row.Scan(&s.OrganizationID, &s.DefaultCurrency, &s.SupportedCurrencies, &s.EnabledPaymentMethods, &s.CardFeeBasisPoints, &s.FolioEmailFooter, &s.CreatedAt, &s.UpdatedAt)
}

func (p *Pool) PaymentSettings(ctx context.Context, orgID uuid.UUID, now time.Time) (*PaymentSettings, error) {
	if err := p.EnsurePaymentSettings(ctx, orgID); err != nil {
		return nil, err
	}
	s := &PaymentSettings{}
	err := scanPaymentSettings(p.QueryRow(ctx, `
		SELECT organization_id, default_currency, supported_currencies, enabled_payment_methods,
		       card_fee_basis_points, folio_email_footer, created_at, updated_at
		FROM organization_payment_settings
		WHERE organization_id = $1
	`, orgID), s)
	if err != nil {
		return nil, err
	}
	s.RateReadiness = p.paymentRateReadiness(ctx, s.SupportedCurrencies, now)
	return s, nil
}

func (p *Pool) EnsurePaymentSettings(ctx context.Context, orgID uuid.UUID) error {
	var currency *string
	err := p.QueryRow(ctx, `SELECT currency FROM organizations WHERE id = $1`, orgID).Scan(&currency)
	if isNoRows(err) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	defaultCurrency := "USD"
	if currency != nil && strings.TrimSpace(*currency) != "" {
		if c, err := NormalizeCurrency(*currency); err == nil {
			defaultCurrency = c
		}
	}
	_, err = p.Exec(ctx, `
		INSERT INTO organization_payment_settings (organization_id, default_currency, supported_currencies)
		VALUES ($1, $2, ARRAY['USD','EUR']::text[])
		ON CONFLICT (organization_id) DO NOTHING
	`, orgID, defaultCurrency)
	return err
}

func (p *Pool) UpdatePaymentSettings(ctx context.Context, orgID uuid.UUID, in PaymentSettingsInput, now time.Time) (*PaymentSettings, error) {
	defaultCurrency, supported, methods, err := normalizePaymentSettings(in)
	if err != nil {
		return nil, err
	}
	// Sprint 023: country currency ∈ accepted. If the org has a
	// country currency set, the new supported list must still contain
	// it so a conversion target is always available.
	org, err := p.OrganizationByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if org.Currency != nil && *org.Currency != "" {
		stillThere := false
		for _, c := range supported {
			if c == *org.Currency {
				stillThere = true
				break
			}
		}
		if !stillThere {
			return nil, errors.New("country currency must remain in supported_currencies (change it on the Organization page first)")
		}
	}
	s := &PaymentSettings{}
	err = scanPaymentSettings(p.QueryRow(ctx, `
		INSERT INTO organization_payment_settings (
			organization_id, default_currency, supported_currencies, enabled_payment_methods,
			card_fee_basis_points, folio_email_footer
		)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (organization_id) DO UPDATE SET
			default_currency = EXCLUDED.default_currency,
			supported_currencies = EXCLUDED.supported_currencies,
			enabled_payment_methods = EXCLUDED.enabled_payment_methods,
			card_fee_basis_points = EXCLUDED.card_fee_basis_points,
			folio_email_footer = EXCLUDED.folio_email_footer,
			updated_at = now()
		RETURNING organization_id, default_currency, supported_currencies, enabled_payment_methods,
			card_fee_basis_points, folio_email_footer, created_at, updated_at
	`, orgID, defaultCurrency, supported, methods, in.CardFeeBasisPoints, cleanNullable(in.FolioEmailFooter)), s)
	if err != nil {
		return nil, err
	}
	s.RateReadiness = p.paymentRateReadiness(ctx, s.SupportedCurrencies, now)
	return s, nil
}

// EnsureCurrencyInSupported makes sure the given currency is in
// organization_payment_settings.supported_currencies for the org.
// Called from UpdateOrganizationProfile when the country currency is
// (re)set so the country currency ∈ accepted invariant holds without
// the operator having to touch the Payments page first.
func (p *Pool) EnsureCurrencyInSupported(ctx context.Context, orgID uuid.UUID, currency string) error {
	c, err := NormalizeCurrency(currency)
	if err != nil {
		return err
	}
	if err := p.EnsurePaymentSettings(ctx, orgID); err != nil {
		return err
	}
	_, err = p.Exec(ctx, `
		UPDATE organization_payment_settings
		SET supported_currencies = (
		    SELECT array_agg(DISTINCT v ORDER BY v)
		    FROM unnest(supported_currencies || ARRAY[$2]::text[]) AS v
		),
		    updated_at = now()
		WHERE organization_id = $1
	`, orgID, c)
	return err
}

func normalizePaymentSettings(in PaymentSettingsInput) (string, []string, []string, error) {
	if in.CardFeeBasisPoints < 0 || in.CardFeeBasisPoints > 2000 {
		return "", nil, nil, errors.New("card_fee_basis_points must be between 0 and 2000")
	}
	supportedSet := map[string]bool{"USD": true}
	for _, c := range in.SupportedCurrencies {
		n, err := NormalizeCurrency(c)
		if err != nil {
			return "", nil, nil, err
		}
		supportedSet[n] = true
	}
	supported := make([]string, 0, len(supportedSet))
	for c := range supportedSet {
		supported = append(supported, c)
	}
	slices.Sort(supported)
	defaultCurrency := strings.TrimSpace(in.DefaultCurrency)
	if defaultCurrency == "" {
		defaultCurrency = "USD"
	}
	var err error
	defaultCurrency, err = NormalizeCurrency(defaultCurrency)
	if err != nil {
		return "", nil, nil, err
	}
	if !supportedSet[defaultCurrency] {
		return "", nil, nil, errors.New("default_currency must be supported")
	}
	methodSet := map[string]bool{}
	for _, m := range in.EnabledPaymentMethods {
		m = strings.ToLower(strings.TrimSpace(m))
		if !slices.Contains(allowedPaymentMethods, m) {
			return "", nil, nil, errors.New("unsupported payment method")
		}
		methodSet[m] = true
	}
	if len(methodSet) == 0 {
		return "", nil, nil, errors.New("at least one payment method is required")
	}
	methods := make([]string, 0, len(methodSet))
	for m := range methodSet {
		methods = append(methods, m)
	}
	slices.Sort(methods)
	return defaultCurrency, supported, methods, nil
}

func validatePaymentMethod(method string) (string, error) {
	method = strings.ToLower(strings.TrimSpace(method))
	if !slices.Contains(allowedPaymentMethods, method) {
		return "", errors.New("unsupported payment method")
	}
	return method, nil
}

func (p *Pool) paymentRateReadiness(ctx context.Context, currencies []string, now time.Time) []PaymentCurrencyRateStatus {
	out := make([]PaymentCurrencyRateStatus, 0, len(currencies))
	for _, c := range currencies {
		status := PaymentCurrencyRateStatus{Currency: c}
		if c == "USD" {
			status.Ready = true
			status.Status = "fresh"
			out = append(out, status)
			continue
		}
		rate, err := p.LatestExchangeRate(ctx, "USD", c, now)
		if err != nil {
			// No non-expired rate exists for this quote.
			status.Status = "missing"
			out = append(out, status)
			continue
		}
		status.Ready = true
		status.Rate = rate
		fetched := rate.FetchedAt
		status.FetchedAt = &fetched
		// Freshness from fetched_at: 24h fresh, 48h stale, then
		// missing. (LatestExchangeRate already filters out
		// expires_at <= now, so reaching here means we have a
		// non-expired row; "missing" can still fire when the row
		// is older than 48h, which usually coincides with expiry.)
		age := now.Sub(fetched)
		switch {
		case age < 24*time.Hour:
			status.Status = "fresh"
		case age < 48*time.Hour:
			status.Status = "stale"
		default:
			status.Status = "missing"
		}
		out = append(out, status)
	}
	return out
}

// DistinctSupportedCurrencies returns the deduplicated union of
// every org's organization_payment_settings.supported_currencies.
// USD is filtered out by the caller (it never needs a rate). Used
// by fxauto.Refresher to know which currencies to fetch.
func (p *Pool) DistinctSupportedCurrencies(ctx context.Context) ([]string, error) {
	rows, err := p.Query(ctx, `
		SELECT DISTINCT unnest(supported_currencies)
		FROM organization_payment_settings
		WHERE array_length(supported_currencies, 1) > 0
		ORDER BY 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// LastFrankfurterRefreshForCurrencies returns the most recent
// fetched_at for any row with provider='frankfurter' whose
// quote_currency is in the given list. Returns nil when no such
// row exists. The payment-settings handler scopes this to the
// caller's supported_currencies so an org never sees an
// auto_refresh_at inflated by another org's accepted currencies.
func (p *Pool) LastFrankfurterRefreshForCurrencies(ctx context.Context, provider string, currencies []string) (*time.Time, error) {
	if len(currencies) == 0 {
		return nil, nil
	}
	var t *time.Time
	if err := p.QueryRow(ctx, `
		SELECT MAX(fetched_at)
		FROM exchange_rates
		WHERE provider = $1
		  AND quote_currency = ANY($2::text[])
	`, provider, currencies).Scan(&t); err != nil {
		return nil, err
	}
	return t, nil
}

func cleanNullable(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}
