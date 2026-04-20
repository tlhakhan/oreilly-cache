package server

import (
	"io/fs"
	"net/http"
	"strings"
)

// withCacheHeaders wraps a FileServer to set appropriate Cache-Control headers.
// Vite emits content-hashed filenames under /assets/, so those can be cached
// for a year. index.html and other top-level files must never be cached so
// users always get the latest entry-point with updated asset hashes.
func withCacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}

// spaHandler serves static files from fsys and falls back to index.html for
// any path that doesn't resolve to a real file, enabling client-side routing.
// Both embed.FS and os.DirFS are already sandboxed, so no explicit path-
// traversal check is needed here.
func spaHandler(fsys fs.FS) http.Handler {
	fileServer := withCacheHeaders(http.FileServerFS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "."
		}

		info, err := fs.Stat(fsys, path)
		if err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		// No real file — serve index.html so the SPA handles the route.
		if _, err := fs.Stat(fsys, "index.html"); err != nil {
			http.Error(w, "index.html not found; has the frontend been built?", http.StatusNotFound)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, fsys, "index.html")
	})
}
