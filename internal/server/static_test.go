package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"oreilly-cache/internal/server"
)

// buildStaticDir creates a minimal fake frontend dist directory in tmp.
func buildStaticDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile := func(rel, body string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	writeFile("index.html", "<html>SPA</html>")
	writeFile("assets/app.abc123.js", "console.log('hello')")
	return dir
}

func TestSPAServesRoot(t *testing.T) {
	dir := buildStaticDir(t)
	h := server.NewHandler(newStore(t), newMockCovers(), discardLog, os.DirFS(dir))

	w := do(t, h, "GET", "/", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if body := w.Body.String(); body != "<html>SPA</html>" {
		t.Errorf("body: %q", body)
	}
}

func TestSPAAssetLongCache(t *testing.T) {
	dir := buildStaticDir(t)
	h := server.NewHandler(newStore(t), newMockCovers(), discardLog, os.DirFS(dir))

	w := do(t, h, "GET", "/assets/app.abc123.js", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	cc := w.Header().Get("Cache-Control")
	if cc != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control: %q", cc)
	}
}

func TestSPAFallbackNoCacheHeader(t *testing.T) {
	dir := buildStaticDir(t)
	h := server.NewHandler(newStore(t), newMockCovers(), discardLog, os.DirFS(dir))

	// /browse doesn't exist as a file — SPA fallback must serve index.html.
	w := do(t, h, "GET", "/browse?type=book", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if body := w.Body.String(); body != "<html>SPA</html>" {
		t.Errorf("body: %q", body)
	}
	cc := w.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control: %q, want no-cache", cc)
	}
}

func TestSPAAPIRouteTakesPrecedence(t *testing.T) {
	dir := buildStaticDir(t)
	st := newStore(t)
	// Seed the publisher index so /publishers returns 200 with JSON.
	seedJSON(t, st, "publishers/index.json", map[string]any{"publishers": []any{}})

	h := server.NewHandler(st, newMockCovers(), discardLog, os.DirFS(dir))

	w := do(t, h, "GET", "/api/publishers", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: %q, want application/json", ct)
	}
}

func TestSPAPathTraversalBlocked(t *testing.T) {
	dir := buildStaticDir(t)
	h := server.NewHandler(newStore(t), newMockCovers(), discardLog, os.DirFS(dir))

	w := do(t, h, "GET", "/../../../etc/passwd", nil)
	// Must not escape staticDir — expect 403 or 404, never 200.
	if w.Code == http.StatusOK {
		t.Errorf("path traversal succeeded, got 200")
	}
}

func TestSPADisabledWhenFlagEmpty(t *testing.T) {
	// With no staticDir, / should return 404 (no route registered).
	h := server.NewHandler(newStore(t), newMockCovers(), discardLog, nil)

	w := do(t, h, "GET", "/", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 with no static dir, got %d", w.Code)
	}
}

func TestSPAMissingIndexHTML(t *testing.T) {
	// Static dir exists but no index.html — fallback should 404, not panic.
	dir := t.TempDir()
	h := server.NewHandler(newStore(t), newMockCovers(), discardLog, os.DirFS(dir))

	w := do(t, h, "GET", "/browse", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when index.html missing, got %d", w.Code)
	}
}
