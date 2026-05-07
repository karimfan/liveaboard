package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type QuoteLineInput struct {
	CatalogItemID uuid.UUID
	Quantity      int
}

type CheckoutQuoteLine struct {
	ID                uuid.UUID
	QuoteID           uuid.UUID
	CatalogItemID     *uuid.UUID
	ItemName          string
	Quantity          int
	UnitPriceUSDCents int64
	LineTotalUSDCents int64
	SortOrder         int
}

type CheckoutQuote struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	RequestedBy       *uuid.UUID
	SourceCurrency    string
	TargetCurrency    string
	SourceAmountCents int64
	TargetAmountMinor int64
	CurrencyExponent  int
	RateProvider      string
	RateNumerator     int64
	RateDenominator   int64
	RateAsOf          time.Time
	ExpiresAt         time.Time
	CreatedAt         time.Time
	Lines             []*CheckoutQuoteLine
}

type CreateCheckoutQuoteInput struct {
	OrganizationID    uuid.UUID
	RequestedBy       *uuid.UUID
	TargetCurrency    string
	SourceAmountCents *int64
	Lines             []QuoteLineInput
	Now               time.Time
}

func (p *Pool) CreateCheckoutQuote(ctx context.Context, in CreateCheckoutQuoteInput) (*CheckoutQuote, error) {
	if in.Now.IsZero() {
		in.Now = time.Now().UTC()
	}
	target, err := NormalizeCurrency(in.TargetCurrency)
	if err != nil {
		return nil, err
	}
	if in.SourceAmountCents == nil && len(in.Lines) == 0 {
		return nil, errors.New("source_amount_cents or lines required")
	}
	if in.SourceAmountCents != nil && len(in.Lines) > 0 {
		return nil, errors.New("provide source_amount_cents or lines, not both")
	}

	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var sourceCents int64
	lineSnapshots := []*CheckoutQuoteLine{}
	if in.SourceAmountCents != nil {
		if *in.SourceAmountCents < 0 {
			return nil, errors.New("source_amount_cents must be non-negative")
		}
		sourceCents = *in.SourceAmountCents
	} else {
		for idx, line := range in.Lines {
			if line.Quantity <= 0 {
				return nil, errors.New("line quantity must be positive")
			}
			item := &CatalogItem{}
			err := scanCatalogItem(tx.QueryRow(ctx, `
				SELECT `+catalogItemSelect+`
				FROM catalog_items i
				JOIN catalog_categories c ON c.id = i.category_id
				WHERE i.organization_id = $1 AND i.id = $2 AND i.archived_at IS NULL
			`, in.OrganizationID, line.CatalogItemID), item)
			if isNoRows(err) {
				return nil, ErrNotFound
			}
			if err != nil {
				return nil, err
			}
			lineTotal := item.PriceUSDCents * int64(line.Quantity)
			sourceCents += lineTotal
			id := line.CatalogItemID
			lineSnapshots = append(lineSnapshots, &CheckoutQuoteLine{
				CatalogItemID:     &id,
				ItemName:          item.Name,
				Quantity:          line.Quantity,
				UnitPriceUSDCents: item.PriceUSDCents,
				LineTotalUSDCents: lineTotal,
				SortOrder:         idx,
			})
		}
	}

	rate, err := p.LatestExchangeRate(ctx, "USD", target, in.Now)
	if err != nil {
		return nil, err
	}
	targetMinor, exp, err := ConvertUSDCentsToMinor(sourceCents, target, rate)
	if err != nil {
		return nil, err
	}

	q := &CheckoutQuote{}
	err = tx.QueryRow(ctx, `
		INSERT INTO checkout_quotes (
			organization_id, requested_by, source_currency, target_currency,
			source_amount_cents, target_amount_minor, currency_exponent,
			rate_provider, rate_numerator, rate_denominator, rate_as_of, expires_at
		)
		VALUES ($1,$2,'USD',$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, organization_id, requested_by, source_currency, target_currency,
			source_amount_cents, target_amount_minor, currency_exponent,
			rate_provider, rate_numerator, rate_denominator, rate_as_of, expires_at, created_at
	`, in.OrganizationID, in.RequestedBy, target, sourceCents, targetMinor, exp, rate.Provider, rate.RateNumerator, rate.RateDenominator, rate.AsOf, in.Now.Add(15*time.Minute)).
		Scan(&q.ID, &q.OrganizationID, &q.RequestedBy, &q.SourceCurrency, &q.TargetCurrency, &q.SourceAmountCents, &q.TargetAmountMinor, &q.CurrencyExponent, &q.RateProvider, &q.RateNumerator, &q.RateDenominator, &q.RateAsOf, &q.ExpiresAt, &q.CreatedAt)
	if err != nil {
		return nil, err
	}
	for _, line := range lineSnapshots {
		line.QuoteID = q.ID
		err := tx.QueryRow(ctx, `
			INSERT INTO checkout_quote_lines (
				quote_id, catalog_item_id, item_name, quantity,
				unit_price_usd_cents, line_total_usd_cents, sort_order
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			RETURNING id
		`, q.ID, line.CatalogItemID, line.ItemName, line.Quantity, line.UnitPriceUSDCents, line.LineTotalUSDCents, line.SortOrder).Scan(&line.ID)
		if err != nil {
			return nil, err
		}
		q.Lines = append(q.Lines, line)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return q, nil
}
