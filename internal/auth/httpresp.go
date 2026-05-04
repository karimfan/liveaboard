package auth

import (
	"encoding/json"
	"net/http"
)

// writeJSON serializes v as JSON with the given status. Used by handlers
// in this package (currently the session middleware, when it returns
// 401s of its own).
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON {"error": code, "message": msg} payload.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": code, "message": msg})
}
