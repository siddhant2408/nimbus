package server

import (
	"io/fs"
	"net/http"
	"strings"
)

var frontendFS fs.FS

// SetFrontendFS sets the filesystem used to serve frontend assets.
// Call this with the embedded dist/ FS before starting the server.
func SetFrontendFS(f fs.FS) {
	frontendFS = f
}

// frontendHandler serves the embedded SPA assets with index.html fallback.
// If no embedded FS is set, it serves the fallback HTML.
func frontendHandler() http.Handler {
	if frontendFS == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fallbackHTML))
		})
	}

	fileServer := http.FileServer(http.FS(frontendFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the exact file first.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		if _, err := fs.Stat(frontendFS, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for client-side routes.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
