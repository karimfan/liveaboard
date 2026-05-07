package httpapi

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

type categoryReq struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Archived  *bool  `json:"archived,omitempty"`
}

func (s *Server) handleListCatalogCategories(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	cats, err := s.Auth.Store.CatalogCategories(r.Context(), u.OrganizationID)
	if err != nil {
		s.Log.Error("catalog categories", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := make([]map[string]any, 0, len(cats))
	for _, c := range cats {
		out = append(out, catalogCategoryView(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": out})
}

func (s *Server) handleCreateCatalogCategory(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req categoryReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	c, err := s.Auth.Store.CreateCatalogCategory(r.Context(), u.OrganizationID, req.Name, req.SortOrder)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, catalogCategoryView(c))
}

func (s *Server) handleUpdateCatalogCategory(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	id, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	var req categoryReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	c, err := s.Auth.Store.UpdateCatalogCategory(r.Context(), u.OrganizationID, id, req.Name, req.SortOrder, req.Archived)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, catalogCategoryView(c))
}

type itemReq struct {
	CategoryID    string  `json:"category_id"`
	Name          string  `json:"name"`
	Description   *string `json:"description"`
	Unit          string  `json:"unit"`
	ChargeType    string  `json:"charge_type"`
	StockMode     string  `json:"stock_mode"`
	PriceUSDCents int64   `json:"price_usd_cents"`
	IsTaxable     bool    `json:"is_taxable"`
	IsRequiredFee bool    `json:"is_required_fee"`
	IsActive      *bool   `json:"is_active,omitempty"`
	Archived      *bool   `json:"archived,omitempty"`
}

func (s *Server) handleListCatalogItems(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	items, err := s.Auth.Store.CatalogItems(r.Context(), u.OrganizationID)
	if err != nil {
		s.Log.Error("catalog items", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, i := range items {
		out = append(out, catalogItemView(i))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleCreateCatalogItem(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req itemReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	in, ok := catalogItemInputFromReq(w, req)
	if !ok {
		return
	}
	item, err := s.Auth.Store.CreateCatalogItem(r.Context(), u.OrganizationID, in)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, catalogItemView(item))
}

func (s *Server) handleUpdateCatalogItem(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	id, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	var req itemReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	in, ok := catalogItemInputFromReq(w, req)
	if !ok {
		return
	}
	item, err := s.Auth.Store.UpdateCatalogItem(r.Context(), u.OrganizationID, id, in, req.Archived)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, catalogItemView(item))
}

func (s *Server) handleApplyCatalogDefaults(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if err := s.Auth.Store.SeedDefaultCatalog(r.Context(), u.OrganizationID); err != nil {
		s.Log.Error("apply catalog defaults", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func catalogItemInputFromReq(w http.ResponseWriter, req itemReq) (store.CatalogItemInput, bool) {
	catID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "category_id must be a uuid")
		return store.CatalogItemInput{}, false
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	return store.CatalogItemInput{
		CategoryID:    catID,
		Name:          req.Name,
		Description:   req.Description,
		Unit:          req.Unit,
		ChargeType:    req.ChargeType,
		StockMode:     req.StockMode,
		PriceUSDCents: req.PriceUSDCents,
		IsTaxable:     req.IsTaxable,
		IsRequiredFee: req.IsRequiredFee,
		IsActive:      active,
	}, true
}

func writeCatalogError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "record not found")
	case isUniqueErr(err):
		writeError(w, http.StatusConflict, "conflict", "name or template already exists")
	default:
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	}
}

func isUniqueErr(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}

func catalogCategoryView(c *store.CatalogCategory) map[string]any {
	return map[string]any{
		"id":              c.ID,
		"template_key":    c.TemplateKey,
		"name":            c.Name,
		"sort_order":      c.SortOrder,
		"is_default_seed": c.IsDefaultSeed,
		"archived_at":     c.ArchivedAt,
		"item_count":      c.ItemCount,
	}
}

func catalogItemView(i *store.CatalogItem) map[string]any {
	return map[string]any{
		"id":              i.ID,
		"category_id":     i.CategoryID,
		"category_name":   i.CategoryName,
		"template_key":    i.TemplateKey,
		"name":            i.Name,
		"description":     i.Description,
		"unit":            i.Unit,
		"charge_type":     i.ChargeType,
		"stock_mode":      i.StockMode,
		"price_usd_cents": i.PriceUSDCents,
		"is_taxable":      i.IsTaxable,
		"is_required_fee": i.IsRequiredFee,
		"is_active":       i.IsActive,
		"archived_at":     i.ArchivedAt,
	}
}
