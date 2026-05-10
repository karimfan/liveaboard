package httpapi

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

type priceOverrideReq struct {
	CatalogItemID string `json:"catalog_item_id"`
	BoatID        string `json:"boat_id,omitempty"`
	TripID        string `json:"trip_id,omitempty"`
	PriceUSDCents int64  `json:"price_usd_cents"`
	Notes         string `json:"notes"`
}

func (s *Server) handleListPriceOverrides(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	overrides, err := s.Auth.Store.ListCatalogPriceOverrides(r.Context(), u.OrganizationID, r.URL.Query().Get("include_archived") == "true")
	if err != nil {
		s.Log.Error("list price overrides", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := make([]map[string]any, 0, len(overrides))
	for _, override := range overrides {
		out = append(out, priceOverrideView(override))
	}
	writeJSON(w, http.StatusOK, map[string]any{"overrides": out})
}

func (s *Server) handleUpsertBoatPriceOverride(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req priceOverrideReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	itemID, ok := parseUUIDField(w, req.CatalogItemID, "catalog_item_id")
	if !ok {
		return
	}
	boatID, ok := parseUUIDField(w, req.BoatID, "boat_id")
	if !ok {
		return
	}
	override, err := s.Auth.Store.UpsertBoatPriceOverride(r.Context(), u.OrganizationID, store.PriceOverrideInput{
		CatalogItemID: itemID,
		BoatID:        boatID,
		PriceUSDCents: req.PriceUSDCents,
		Notes:         req.Notes,
		ActorUserID:   u.ID,
	})
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	s.recordStaffAudit(r.Context(), u.OrganizationID, u.ID, "catalog.price_override_upserted", "catalog_price_override", &override.ID, nil, nil, map[string]any{
		"scope": "boat", "boat_id": boatID, "catalog_item_id": itemID, "price_usd_cents": override.PriceUSDCents,
	})
	writeJSON(w, http.StatusOK, priceOverrideView(override))
}

func (s *Server) handleUpsertTripPriceOverride(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req priceOverrideReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	itemID, ok := parseUUIDField(w, req.CatalogItemID, "catalog_item_id")
	if !ok {
		return
	}
	tripID, ok := parseUUIDField(w, req.TripID, "trip_id")
	if !ok {
		return
	}
	override, err := s.Auth.Store.UpsertTripPriceOverride(r.Context(), u.OrganizationID, store.PriceOverrideInput{
		CatalogItemID: itemID,
		TripID:        tripID,
		PriceUSDCents: req.PriceUSDCents,
		Notes:         req.Notes,
		ActorUserID:   u.ID,
	})
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	s.recordStaffAudit(r.Context(), u.OrganizationID, u.ID, "catalog.price_override_upserted", "catalog_price_override", &override.ID, &tripID, nil, map[string]any{
		"scope": "trip", "catalog_item_id": itemID, "price_usd_cents": override.PriceUSDCents,
	})
	writeJSON(w, http.StatusOK, priceOverrideView(override))
}

func (s *Server) handleArchivePriceOverride(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	id, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	override, err := s.Auth.Store.ArchiveCatalogPriceOverride(r.Context(), u.OrganizationID, id, u.ID)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	s.recordStaffAudit(r.Context(), u.OrganizationID, u.ID, "catalog.price_override_archived", "catalog_price_override", &override.ID, override.TripID, nil, map[string]any{
		"boat_id": override.BoatID, "catalog_item_id": override.CatalogItemID,
	})
	writeJSON(w, http.StatusOK, priceOverrideView(override))
}

func priceOverrideView(o *store.CatalogPriceOverride) map[string]any {
	scope := "boat"
	if o.TripID != nil {
		scope = "trip"
	}
	return map[string]any{
		"id":              o.ID,
		"catalog_item_id": o.CatalogItemID,
		"item_name":       o.ItemName,
		"scope":           scope,
		"boat_id":         o.BoatID,
		"boat_name":       o.BoatName,
		"trip_id":         o.TripID,
		"trip_label":      o.TripLabel,
		"price_usd_cents": o.PriceUSDCents,
		"notes":           o.Notes,
		"archived_at":     o.ArchivedAt,
		"created_at":      o.CreatedAt,
		"updated_at":      o.UpdatedAt,
	}
}

func parseUUIDField(w http.ResponseWriter, raw, field string) (uuid.UUID, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", field+" must be a uuid")
		return uuid.Nil, false
	}
	return id, true
}
