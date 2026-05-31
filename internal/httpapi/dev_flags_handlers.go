package httpapi

import "net/http"

// handleDevFlags is a public endpoint that surfaces a small set of
// dev-mode signals to the SPA so it can render dev-only affordances
// (test data fill, fake-guest generators, the inbox viewer link).
// Today it reports a single flag: filesystem_email, which mirrors
// the existence of the dev inbox viewer mount.
//
// The endpoint is public on purpose — the signal itself isn't
// sensitive (knowing the transport doesn't grant any new
// capabilities), and gating it on auth would force every public
// page (login, signup, guest registration) to authenticate to
// learn whether it can offer test-data buttons.
func (s *Server) handleDevFlags(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"filesystem_email":     s.DevInboxDir != "",
		"ui_redesign_switcher": s.IsDev,
	})
}
