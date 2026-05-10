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

type PaymentCurrencyRateStatus struct {
	Currency string
	Ready    bool
	Rate     *ExchangeRate
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
		} else if rate, err := p.LatestExchangeRate(ctx, "USD", c, now); err == nil {
			status.Ready = true
			status.Rate = rate
		}
		out = append(out, status)
	}
	return out
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
