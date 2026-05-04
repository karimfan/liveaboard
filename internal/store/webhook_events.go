package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrWebhookEventReplayed is returned by RecordWebhookEvent when the
// given event id has already been processed. Callers should treat this
// as a successful no-op (HTTP 200 with empty body).
var ErrWebhookEventReplayed = errors.New("store: webhook event already processed")

// RecordWebhookEvent inserts an idempotency record for the given event
// id. If the id is already present, returns ErrWebhookEventReplayed.
//
// Callers should call this BEFORE applying any side effects, so a
// replayed delivery short-circuits before re-processing.
func (p *Pool) RecordWebhookEvent(ctx context.Context, eventID string) error {
	_, err := p.Exec(ctx, `INSERT INTO webhook_events (id) VALUES ($1)`, eventID)
	if err != nil {
		var pgErr interface{ SQLState() string }
		if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
			return ErrWebhookEventReplayed
		}
		// Fallback string check for older pgx error wrappings.
		if !errors.Is(err, pgx.ErrNoRows) && contains(err.Error(), "duplicate key") {
			return ErrWebhookEventReplayed
		}
		return err
	}
	return nil
}
