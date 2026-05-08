package httpapi

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/store"
)

func (s *Server) recordStaffAudit(ctx context.Context, orgID, actorID uuid.UUID, action, entityType string, entityID, tripID, tripGuestID *uuid.UUID, meta map[string]any) {
	_, _ = s.Auth.Store.RecordAuditEvent(ctx, store.AuditEventInput{
		OrganizationID: orgID,
		Actor:          store.StaffAuditActor(actorID),
		Action:         action,
		EntityType:     entityType,
		EntityID:       entityID,
		TripID:         tripID,
		TripGuestID:    tripGuestID,
		Metadata:       meta,
	})
}

func (s *Server) recordGuestAudit(ctx context.Context, orgID, actorID uuid.UUID, action, entityType string, entityID, tripID, tripGuestID *uuid.UUID, meta map[string]any) {
	_, _ = s.Auth.Store.RecordAuditEvent(ctx, store.AuditEventInput{
		OrganizationID: orgID,
		Actor:          store.GuestAuditActor(actorID),
		Action:         action,
		EntityType:     entityType,
		EntityID:       entityID,
		TripID:         tripID,
		TripGuestID:    tripGuestID,
		Metadata:       meta,
	})
}

func emailDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parts[1]))
}
