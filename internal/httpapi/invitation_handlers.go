package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

// invitation_handlers.go: Sprint 009 admin-only invitation surface.
// All routes are mounted behind RequireOrgAdmin; the accept/lookup
// endpoints are public (token-bearing).

// --- admin (org_admin only) ---

type inviteReq struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (s *Server) handleCreateInvitation(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req inviteReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	if req.Role == "" {
		req.Role = store.RoleSiteDirector
	}
	inv, err := s.Auth.Invite(r.Context(), u.OrganizationID, u.ID, req.Email, req.Role)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, invitationView(inv))
}

func (s *Server) handleResendInvitation(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "id must be a uuid")
		return
	}
	inv, err := s.Auth.ResendInvitation(r.Context(), u.OrganizationID, id, u.ID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, invitationView(inv))
}

func (s *Server) handleRevokeInvitation(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "id must be a uuid")
		return
	}
	if err := s.Auth.RevokeInvitation(r.Context(), u.OrganizationID, id); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListInvitations(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	invs, err := s.Auth.PendingInvitations(r.Context(), u.OrganizationID)
	if err != nil {
		s.Log.Error("list invitations", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := make([]map[string]any, 0, len(invs))
	for _, inv := range invs {
		out = append(out, invitationView(inv))
	}
	writeJSON(w, http.StatusOK, map[string]any{"invitations": out})
}

// --- public (token-bearing) ---

func (s *Server) handleLookupInvitation(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	view, err := s.Auth.LookupInvitation(r.Context(), token)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"email":             view.Email,
		"role":              view.Role,
		"organization_name": view.OrganizationName,
		"expires_at":        view.ExpiresAt,
	})
}

type acceptInvitationReq struct {
	Token    string `json:"token"`
	FullName string `json:"full_name"`
	Password string `json:"password"`
}

func (s *Server) handleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var req acceptInvitationReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := s.Auth.AcceptInvitation(r.Context(), req.Token, req.FullName, req.Password)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	auth.SetSessionCookie(w, res.Token, res.ExpiresAt, s.CookieSecure)
	writeJSON(w, http.StatusOK, authUserView(res.User))
}

// --- helpers ---

func invitationView(inv *store.Invitation) map[string]any {
	return map[string]any{
		"id":         inv.ID,
		"email":      inv.Email,
		"role":       inv.Role,
		"expires_at": inv.ExpiresAt,
	}
}

func parseUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, name))
}
