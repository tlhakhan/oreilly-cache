package server

import (
	"net/http"
	"os"
	"path/filepath"
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

// spaHandler serves static files from staticDir and falls back to index.html
// for any path that doesn't match a real file, enabling client-side routing.
func spaHandler(staticDir string) http.Handler {
	indexPath := filepath.Join(staticDir, "index.html")
	fs := withCacheHeaders(http.FileServer(http.Dir(staticDir)))

	absStatic, _ := filepath.Abs(staticDir)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent path traversal: reject any path that escapes staticDir.
		cleaned := filepath.Clean(r.URL.Path)
		fullPath := filepath.Join(staticDir, cleaned)
		absFull, _ := filepath.Abs(fullPath)
		if !strings.HasPrefix(absFull+string(filepath.Separator), absStatic+string(filepath.Separator)) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		info, err := os.Stat(fullPath)
		if err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}

		// No real file — serve index.html so the SPA handles the route.
		if _, err := os.Stat(indexPath); err != nil {
			http.Error(w, "index.html not found; has the frontend been built?", http.StatusNotFound)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, indexPath)
	})
}
