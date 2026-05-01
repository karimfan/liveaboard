package httpapi

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"

	webdist "github.com/karimfan/liveaboard/web"
)

// SPAHandler serves the Vite build output. Any GET that doesn't resolve
// to a file in web/dist falls back to index.html so client-side routes
// work.
//
// In dev mode the browser hits Vite directly on :5173, so this handler
// is effectively unused for /, /login, etc. — it only kicks in for the
// production binary.
func SPAHandler() http.Handler {
	dist, err := fs.Sub(webdist.Assets, "dist")
	if err != nil {
		// embed at compile time guarantees dist exists; this is unreachable.
		panic(err)
	}
	fileServer := http.FileServer(http.FS(dist))

	indexBytes, indexErr := fs.ReadFile(dist, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		clean := path.Clean("/" + r.URL.Path)
		if clean != "/" {
			candidate := strings.TrimPrefix(clean, "/")
			if f, err := dist.Open(candidate); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			} else if !errors.Is(err, fs.ErrNotExist) {
				http.Error(w, "asset error", http.StatusInternalServerError)
				return
			}
		}

		// SPA fallback: serve index.html if available, otherwise 404.
		if indexErr != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(indexBytes)
	})
}
