package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

// invitation_handlers.go: Sprint 010 admin-only invitation surface.
// Sprint 009 created invitations from {email, role} only. Sprint 010
// captures full_name (required) and phone (optional) at invite time so
// the admin's mental model is "I'm adding a Cruise Director" — not
// "I'm slinging a token at a stranger."

// --- admin (org_admin only) ---

type inviteReq struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
}

func (s *Server) handleCreateInvitation(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req inviteReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	if req.Role == "" {
		req.Role = store.RoleCruiseDirector
	}
	inv, err := s.Auth.Invite(r.Context(), u.OrganizationID, u.ID, auth.InviteInput{
		Email:    req.Email,
		FullName: req.FullName,
		Phone:    req.Phone,
		Role:     req.Role,
	})
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
		"full_name":         view.FullName,
		"role":              view.Role,
		"organization_name": view.OrganizationName,
		"expires_at":        view.ExpiresAt,
	})
}

type acceptInvitationReq struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// handleAcceptInvitation consumes a token and creates the user from
// the invitation row's metadata (no name input from the invitee — the
// admin captured it at invite time).
func (s *Server) handleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var req acceptInvitationReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := s.Auth.AcceptInvitation(r.Context(), req.Token, req.Password)
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
		"full_name":  inv.FullName,
		"phone":      inv.Phone,
		"role":       inv.Role,
		"expires_at": inv.ExpiresAt,
	}
}

func parseUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, name))
}
