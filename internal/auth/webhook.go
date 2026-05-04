package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	svix "github.com/svix/svix-webhooks/go"

	"github.com/karimfan/liveaboard/internal/store"
)

// WebhookReceiver verifies and dispatches Clerk webhook events.
// Verification uses svix (Clerk's underlying webhook delivery service)
// against a per-endpoint signing secret. Idempotency is enforced via
// the webhook_events table — a replayed delivery short-circuits with
// HTTP 200.
type WebhookReceiver struct {
	Provider Provider
	Store    *store.Pool
	Log      *slog.Logger
	Secret   string // CLERK_WEBHOOK_SECRET
	Now      func() time.Time

	wh *svix.Webhook
}

// NewWebhookReceiver constructs a receiver. Returns an error if Secret is
// empty so production wiring fails fast.
func NewWebhookReceiver(provider Provider, pool *store.Pool, log *slog.Logger, secret string) (*WebhookReceiver, error) {
	if secret == "" {
		return nil, errors.New("auth: webhook secret is empty")
	}
	wh, err := svix.NewWebhook(secret)
	if err != nil {
		return nil, err
	}
	return &WebhookReceiver{
		Provider: provider,
		Store:    pool,
		Log:      log,
		Secret:   secret,
		Now:      time.Now,
		wh:       wh,
	}, nil
}

// clerkWebhookPayload is the envelope of every Clerk webhook event.
type clerkWebhookPayload struct {
	Type      string          `json:"type"`
	Object    string          `json:"object"`
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"`
}

// Handle is the http.HandlerFunc for POST /api/webhooks/clerk.
//
//   - Verifies the svix signature using the configured secret.
//   - Reads the svix-id header for idempotency.
//   - Records the event in webhook_events; replays return 200 immediately.
//   - Dispatches to the per-event handler. Handler errors are logged but
//     return 200 so Clerk does not retry indefinitely; structural errors
//     return 500 so Clerk retries with backoff.
func (r *WebhookReceiver) Handle(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, req.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	if err := r.wh.Verify(body, req.Header); err != nil {
		// Do not log the body — it is unverified.
		r.Log.Warn("clerk webhook: signature verification failed", "err", err)
		writeError(w, http.StatusUnauthorized, "invalid_signature", "signature verification failed")
		return
	}

	eventID := req.Header.Get("svix-id")
	if eventID == "" {
		writeError(w, http.StatusBadRequest, "missing_event_id", "svix-id header required")
		return
	}

	if err := r.Store.RecordWebhookEvent(req.Context(), eventID); err != nil {
		if errors.Is(err, store.ErrWebhookEventReplayed) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "replayed": true})
			return
		}
		r.Log.Error("clerk webhook: record event", "err", err, "event_id", eventID)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	var payload clerkWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		r.Log.Error("clerk webhook: parse envelope", "err", err, "event_id", eventID)
		writeError(w, http.StatusBadRequest, "invalid_payload", "could not parse webhook envelope")
		return
	}

	if err := r.dispatch(req.Context(), payload); err != nil {
		// The event has been recorded as processed; we don't unwind that.
		// Logging the failure surfaces it for ops; Clerk's at-least-once
		// retry would only re-trigger the replay path.
		r.Log.Error("clerk webhook: dispatch", "err", err, "event_id", eventID, "type", payload.Type)
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (r *WebhookReceiver) dispatch(ctx context.Context, payload clerkWebhookPayload) error {
	switch payload.Type {
	case "user.created":
		return r.handleUserCreated(ctx, payload.Data)
	case "user.updated":
		return r.handleUserUpdated(ctx, payload.Data)
	case "user.deleted":
		return r.handleUserDeleted(ctx, payload.Data)

	case "organization.created":
		return r.handleOrganizationCreated(ctx, payload.Data)
	case "organization.updated":
		return r.handleOrganizationUpdated(ctx, payload.Data)
	case "organization.deleted":
		return r.handleOrganizationDeleted(ctx, payload.Data)

	case "organizationMembership.created":
		return r.handleMembershipCreated(ctx, payload.Data)
	case "organizationMembership.updated":
		return r.handleMembershipUpdated(ctx, payload.Data)
	case "organizationMembership.deleted":
		return r.handleMembershipDeleted(ctx, payload.Data)

	default:
		// Unknown / uninteresting event types are not an error; Clerk will
		// emit many event types we don't currently subscribe to or care
		// about. Log at info so we can see them in dev.
		r.Log.Info("clerk webhook: unhandled event type", "type", payload.Type)
		return nil
	}
}
