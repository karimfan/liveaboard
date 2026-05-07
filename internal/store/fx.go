package store

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ExchangeRate struct {
	ID              uuid.UUID
	Provider        string
	BaseCurrency    string
	QuoteCurrency   string
	RateNumerator   int64
	RateDenominator int64
	AsOf            time.Time
	FetchedAt       time.Time
	ExpiresAt       time.Time
}

var currencyExponents = map[string]int{
	"USD": 2, "EUR": 2, "GBP": 2, "AUD": 2, "CAD": 2, "NZD": 2,
	"IDR": 0, "JPY": 0, "KRW": 0, "THB": 2, "PHP": 2, "SGD": 2,
	"MYR": 2, "MVR": 2, "BHD": 3, "KWD": 3, "OMR": 3,
}

func CurrencyExponent(code string) (int, bool) {
	exp, ok := currencyExponents[strings.ToUpper(strings.TrimSpace(code))]
	return exp, ok
}

func NormalizeCurrency(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 3 {
		return "", errors.New("currency must be a 3-letter ISO code")
	}
	if _, ok := CurrencyExponent(code); !ok {
		return "", errors.New("unsupported currency")
	}
	return code, nil
}

func scanExchangeRate(row interface{ Scan(dest ...any) error }, r *ExchangeRate) error {
	return row.Scan(&r.ID, &r.Provider, &r.BaseCurrency, &r.QuoteCurrency, &r.RateNumerator, &r.RateDenominator, &r.AsOf, &r.FetchedAt, &r.ExpiresAt)
}

func (p *Pool) UpsertExchangeRate(ctx context.Context, provider, baseCurrency, quoteCurrency string, numerator, denominator int64, asOf, expiresAt time.Time) (*ExchangeRate, error) {
	base, err := NormalizeCurrency(baseCurrency)
	if err != nil {
		return nil, err
	}
	quote, err := NormalizeCurrency(quoteCurrency)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(provider) == "" {
		return nil, errors.New("provider required")
	}
	if numerator <= 0 || denominator <= 0 {
		return nil, errors.New("rate numerator and denominator must be positive")
	}
	r := &ExchangeRate{}
	err = scanExchangeRate(p.QueryRow(ctx, `
		INSERT INTO exchange_rates (provider, base_currency, quote_currency, rate_numerator, rate_denominator, as_of, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, provider, base_currency, quote_currency, rate_numerator, rate_denominator, as_of, fetched_at, expires_at
	`, provider, base, quote, numerator, denominator, asOf, expiresAt), r)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (p *Pool) LatestExchangeRates(ctx context.Context, now time.Time) ([]*ExchangeRate, error) {
	rows, err := p.Query(ctx, `
		SELECT DISTINCT ON (base_currency, quote_currency)
			id, provider, base_currency, quote_currency, rate_numerator, rate_denominator, as_of, fetched_at, expires_at
		FROM exchange_rates
		WHERE expires_at > $1
		ORDER BY base_currency, quote_currency, as_of DESC, fetched_at DESC
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ExchangeRate
	for rows.Next() {
		r := &ExchangeRate{}
		if err := scanExchangeRate(rows, r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *Pool) LatestExchangeRate(ctx context.Context, baseCurrency, quoteCurrency string, now time.Time) (*ExchangeRate, error) {
	base, err := NormalizeCurrency(baseCurrency)
	if err != nil {
		return nil, err
	}
	quote, err := NormalizeCurrency(quoteCurrency)
	if err != nil {
		return nil, err
	}
	if base == quote {
		return &ExchangeRate{
			ID:              uuid.Nil,
			Provider:        "identity",
			BaseCurrency:    base,
			QuoteCurrency:   quote,
			RateNumerator:   1,
			RateDenominator: 1,
			AsOf:            now,
			FetchedAt:       now,
			ExpiresAt:       now.Add(time.Hour),
		}, nil
	}
	r := &ExchangeRate{}
	err = scanExchangeRate(p.QueryRow(ctx, `
		SELECT id, provider, base_currency, quote_currency, rate_numerator, rate_denominator, as_of, fetched_at, expires_at
		FROM exchange_rates
		WHERE base_currency = $1 AND quote_currency = $2 AND expires_at > $3
		ORDER BY as_of DESC, fetched_at DESC
		LIMIT 1
	`, base, quote, now), r)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func ConvertUSDCentsToMinor(sourceCents int64, targetCurrency string, rate *ExchangeRate) (int64, int, error) {
	if sourceCents < 0 {
		return 0, 0, errors.New("source amount must be non-negative")
	}
	targetCurrency, err := NormalizeCurrency(targetCurrency)
	if err != nil {
		return 0, 0, err
	}
	exp, _ := CurrencyExponent(targetCurrency)
	scale := int64(1)
	for i := 0; i < exp; i++ {
		scale *= 10
	}
	num := big.NewInt(sourceCents)
	num.Mul(num, big.NewInt(rate.RateNumerator))
	num.Mul(num, big.NewInt(scale))
	den := big.NewInt(rate.RateDenominator * 100)
	quo, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	// Round half up in target minor units.
	rem.Mul(rem, big.NewInt(2))
	if rem.Cmp(den) >= 0 {
		quo.Add(quo, big.NewInt(1))
	}
	if !quo.IsInt64() {
		return 0, 0, errors.New("converted amount overflows int64")
	}
	return quo.Int64(), exp, nil
}
