package httpapi

import (
	"net/http"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
)

func (s *Server) handleCockpit(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	cockpit, err := s.Auth.Store.Cockpit(r.Context(), u.OrganizationID, u.ID, u.Role, time.Now().UTC())
	if err != nil {
		s.Log.Error("cockpit failed", "err", err, "org_id", u.OrganizationID, "user_id", u.ID)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, cockpit)
}
