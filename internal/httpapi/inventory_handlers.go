package httpapi

import (
	"errors"
	"net/http"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

func (s *Server) handleInventoryBoatSummary(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	rows, err := s.Auth.Store.InventoryFleetSummary(r.Context(), u.OrganizationID)
	if err != nil {
		s.Log.Error("inventory boat summary", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"boats": rows})
}

func (s *Server) handleListBoatInventory(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	boatID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	rows, err := s.Auth.Store.BoatInventory(r.Context(), u.OrganizationID, boatID)
	if err != nil {
		writeInventoryError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, inventoryItemView(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

type setInventoryReq struct {
	QuantityOnHand int     `json:"quantity_on_hand"`
	ReorderLevel   *int    `json:"reorder_level"`
	ParLevel       *int    `json:"par_level"`
	Notes          *string `json:"notes"`
}

func (s *Server) handleSetBoatInventory(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	boatID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	itemID, err := parseUUIDParam(r, "item_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "item_id must be a uuid")
		return
	}
	var req setInventoryReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	row, err := s.Auth.Store.SetBoatInventoryItem(r.Context(), u.OrganizationID, boatID, itemID, store.InventorySetInput{
		QuantityOnHand: req.QuantityOnHand,
		ReorderLevel:   req.ReorderLevel,
		ParLevel:       req.ParLevel,
		Notes:          req.Notes,
	})
	if err != nil {
		writeInventoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inventoryItemView(row))
}

type adjustStockReq struct {
	MovementType  string  `json:"movement_type"`
	DeltaQuantity int     `json:"delta_quantity"`
	Note          *string `json:"note"`
}

func (s *Server) handleAdjustBoatInventory(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	boatID, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	itemID, err := parseUUIDParam(r, "item_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "item_id must be a uuid")
		return
	}
	var req adjustStockReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	src := "manual_adjustment"
	mv, row, err := s.Auth.Store.AdjustStock(r.Context(), u.OrganizationID, boatID, itemID, store.StockAdjustmentInput{
		ActorUserID:   &u.ID,
		MovementType:  req.MovementType,
		DeltaQuantity: req.DeltaQuantity,
		SourceType:    &src,
		Note:          req.Note,
	})
	if err != nil {
		writeInventoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"movement": stockMovementView(mv),
		"item":     inventoryItemView(row),
	})
}

func writeInventoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "record not found")
	default:
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	}
}

func inventoryItemView(i *store.BoatInventoryItem) map[string]any {
	return map[string]any{
		"id":                i.ID,
		"boat_id":           i.BoatID,
		"catalog_item_id":   i.CatalogItemID,
		"item_name":         i.ItemName,
		"category_name":     i.CategoryName,
		"unit":              i.Unit,
		"stock_mode":        i.StockMode,
		"price_usd_cents":   i.PriceUSDCents,
		"quantity_on_hand":  i.QuantityOnHand,
		"quantity_reserved": i.QuantityReserved,
		"reorder_level":     i.ReorderLevel,
		"par_level":         i.ParLevel,
		"last_counted_at":   i.LastCountedAt,
		"notes":             i.Notes,
		"status":            i.Status,
	}
}

func stockMovementView(m *store.StockMovement) map[string]any {
	return map[string]any{
		"id":              m.ID,
		"boat_id":         m.BoatID,
		"catalog_item_id": m.CatalogItemID,
		"actor_user_id":   m.ActorUserID,
		"movement_type":   m.MovementType,
		"delta_quantity":  m.DeltaQuantity,
		"quantity_before": m.QuantityBefore,
		"quantity_after":  m.QuantityAfter,
		"source_type":     m.SourceType,
		"source_id":       m.SourceID,
		"note":            m.Note,
		"created_at":      m.CreatedAt,
	}
}
