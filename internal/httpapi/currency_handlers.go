package httpapi

import (
	"net/http"

	"github.com/karimfan/liveaboard/internal/store"
)

// handleListCurrencies returns the ISO 4217 currency catalog the
// frontend pickers use. Mounted inside the authenticated route group
// so signed-in admins (and directors viewing the org settings hint)
// can use the picker without an unauthenticated catalog endpoint.
func (s *Server) handleListCurrencies(w http.ResponseWriter, _ *http.Request) {
	all := store.AllCurrencies()
	out := make([]map[string]any, 0, len(all))
	for _, c := range all {
		out = append(out, map[string]any{
			"code":     c.Code,
			"name":     c.Name,
			"exponent": c.Exponent,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"currencies": out})
}
