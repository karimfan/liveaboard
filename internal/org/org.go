// Package org provides read-only views of an Organization for the
// authenticated Org Admin. Mutations live in their own packages.
package org

import (
	"context"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/store"
)

type Service struct {
	Store *store.Pool
}

func New(s *store.Pool) *Service { return &Service{Store: s} }

// Dashboard is the response shape for GET /api/organization (US-2.1).
// Boats / active_trips / total_guests are zero until later sprints add
// the corresponding tables; the field shape is the final shape.
type Dashboard struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Currency  *string   `json:"currency"`
	CreatedAt string    `json:"created_at"`
	Stats     Stats     `json:"stats"`
}

type Stats struct {
	Boats       int `json:"boats"`
	ActiveTrips int `json:"active_trips"`
	TotalGuests int `json:"total_guests"`
}

func (s *Service) Dashboard(ctx context.Context, orgID uuid.UUID) (*Dashboard, error) {
	o, err := s.Store.OrganizationByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return &Dashboard{
		ID:        o.ID,
		Name:      o.Name,
		Currency:  o.Currency,
		CreatedAt: o.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Stats:     Stats{Boats: 0, ActiveTrips: 0, TotalGuests: 0},
	}, nil
}
