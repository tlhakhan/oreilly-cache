package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"oreilly-cache/internal/server"
	"oreilly-cache/internal/store"
	"oreilly-cache/internal/transform"
	"oreilly-cache/internal/upstream"
)

// ---- helpers ----

var discardLog = slog.New(slog.NewTextHandler(io.Discard, nil))

func newStore(t *testing.T) *store.Store {
	t.Helper()
	return store.New(filepath.Join(t.TempDir(), "cache"))
}

// seedJSON writes JSON-marshaled v to relPath in st.
func seedJSON(t *testing.T, st *store.Store, relPath string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %s: %v", relPath, err)
	}
	if err := st.WriteAtomic(relPath, b); err != nil {
		t.Fatalf("seed %s: %v", relPath, err)
	}
}

func do(t *testing.T, h http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// ---- mock cover client ----

type mockCovers struct {
	mu    sync.Mutex
	calls atomic.Int64
	resp  map[string]coverResp
}

type coverResp struct {
	data []byte
	ct   string
	err  error
}

func newMockCovers() *mockCovers {
	return &mockCovers{resp: make(map[string]coverResp)}
}

func (m *mockCovers) set(identifier, size string, data []byte, ct string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resp[identifier+"/"+size] = coverResp{data: data, ct: ct, err: err}
}

func (m *mockCovers) GetCover(_ context.Context, identifier, size string) ([]byte, string, error) {
	m.calls.Add(1)
	m.mu.Lock()
	r, ok := m.resp[identifier+"/"+size]
	m.mu.Unlock()
	if !ok {
		return nil, "", fmt.Errorf("mockCovers: no response for %s/%s", identifier, size)
	}
	return r.data, r.ct, r.err
}

// ---- JSON endpoint tests ----

func TestPublisherIndex(t *testing.T) {
	st := newStore(t)
	index := transform.PublisherIndex{Publishers: []transform.Publisher{
		{UUID: "u1", Name: "Pub One"},
		{UUID: "u2", Name: "Pub Two"},
	}}
	seedJSON(t, st, store.PublisherIndexPath(), index)

	h := server.NewHandler(st, newMockCovers(), discardLog, "")
	w := do(t, h, "GET", "/api/publishers", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type: %q", ct)
	}
	var got transform.PublisherIndex
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Publishers) != 2 {
		t.Errorf("publisher count: got %d, want 2", len(got.Publishers))
	}
}

func TestPublisherIndexNotFound(t *testing.T) {
	h := server.NewHandler(newStore(t), newMockCovers(), discardLog, "")
	w := do(t, h, "GET", "/api/publishers", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestPublisher(t *testing.T) {
	st := newStore(t)
	seedJSON(t, st, store.PublisherPath("abc-123"), transform.Publisher{UUID: "abc-123", Name: "Test Pub"})

	h := server.NewHandler(st, newMockCovers(), discardLog, "")
	w := do(t, h, "GET", "/api/publishers/abc-123", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var got transform.Publisher
	json.Unmarshal(w.Body.Bytes(), &got) //nolint:errcheck
	if got.UUID != "abc-123" {
		t.Errorf("uuid: %q", got.UUID)
	}
}

func TestPublisherNotFound(t *testing.T) {
	h := server.NewHandler(newStore(t), newMockCovers(), discardLog, "")
	w := do(t, h, "GET", "/api/publishers/no-such-uuid", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestPublisherItems(t *testing.T) {
	st := newStore(t)
	list := transform.ItemList{Items: []transform.Item{
		{OURN: "urn:a", Name: "Book A"},
	}}
	seedJSON(t, st, store.PublisherItemsPath("pub-1"), list)

	h := server.NewHandler(st, newMockCovers(), discardLog, "")
	w := do(t, h, "GET", "/api/publishers/pub-1/items", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var got transform.ItemList
	json.Unmarshal(w.Body.Bytes(), &got) //nolint:errcheck
	if len(got.Items) != 1 {
		t.Errorf("item count: got %d, want 1", len(got.Items))
	}
}

func TestItem(t *testing.T) {
	st := newStore(t)
	ourn := "urn:orm:book:9781098128944"
	seedJSON(t, st, store.ItemPath(ourn), transform.Item{
		OURN: ourn,
		Name: "Learning Go",
	})

	h := server.NewHandler(st, newMockCovers(), discardLog, "")
	w := do(t, h, "GET", "/api/items/"+ourn, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var got transform.Item
	json.Unmarshal(w.Body.Bytes(), &got) //nolint:errcheck
	if got.Name != "Learning Go" {
		t.Errorf("name: %q", got.Name)
	}
}

func TestItemNotFound(t *testing.T) {
	h := server.NewHandler(newStore(t), newMockCovers(), discardLog, "")
	w := do(t, h, "GET", "/api/items/no-such-id", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestConditionalGet verifies that If-Modified-Since triggers a 304.
func TestConditionalGet(t *testing.T) {
	st := newStore(t)
	seedJSON(t, st, store.PublisherIndexPath(), transform.PublisherIndex{})

	h := server.NewHandler(st, newMockCovers(), discardLog, "")

	// First request to capture Last-Modified.
	w1 := do(t, h, "GET", "/api/publishers", nil)
	lastMod := w1.Header().Get("Last-Modified")
	if lastMod == "" {
		t.Fatal("no Last-Modified header on first response")
	}

	// Second request with If-Modified-Since set to a future time → 304.
	future := time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)
	w2 := do(t, h, "GET", "/api/publishers", map[string]string{"If-Modified-Since": future})
	if w2.Code != http.StatusNotModified {
		t.Errorf("expected 304, got %d", w2.Code)
	}
}

// ---- /healthz ----

func TestHealthzNoScrape(t *testing.T) {
	h := server.NewHandler(newStore(t), newMockCovers(), discardLog, "")
	w := do(t, h, "GET", "/api/healthz", nil)

	if w.Code != http.StatusOK {
		t.Errorf("status: %d", w.Code)
	}
	var m map[string]any
	json.Unmarshal(w.Body.Bytes(), &m) //nolint:errcheck
	if m["status"] != "ok" {
		t.Errorf("status field: %v", m["status"])
	}
}

func TestHealthzWithScrape(t *testing.T) {
	st := newStore(t)
	scrapeData := map[string]any{
		"publisher_count": 5,
		"item_count":      42,
	}
	seedJSON(t, st, store.LastScrapePath(), scrapeData)

	h := server.NewHandler(st, newMockCovers(), discardLog, "")
	w := do(t, h, "GET", "/api/healthz", nil)

	if w.Code != http.StatusOK {
		t.Errorf("status: %d", w.Code)
	}
	var m map[string]any
	json.Unmarshal(w.Body.Bytes(), &m) //nolint:errcheck
	if m["scrape"] == nil {
		t.Error("expected scrape field to be populated")
	}
}

// ---- cover handler ----

func TestCoverServedFromDiskCache(t *testing.T) {
	st := newStore(t)
	imgBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0} // JPEG magic
	st.WriteAtomic(store.CoverPath("isbn:123", "large"), imgBytes) //nolint:errcheck

	covers := newMockCovers()
	h := server.NewHandler(st, covers, discardLog, "")
	w := do(t, h, "GET", "/api/covers/isbn:123/large", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if covers.calls.Load() != 0 {
		t.Error("upstream should not be called when cover is cached")
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "image/") {
		t.Errorf("Content-Type: %q", w.Header().Get("Content-Type"))
	}
}

func TestCoverNegativeCache(t *testing.T) {
	st := newStore(t)
	// Write the .404 sentinel.
	st.WriteAtomic(store.CoverNotFoundPath("isbn:missing", "large"), []byte{}) //nolint:errcheck

	covers := newMockCovers()
	h := server.NewHandler(st, covers, discardLog, "")
	w := do(t, h, "GET", "/api/covers/isbn:missing/large", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	if covers.calls.Load() != 0 {
		t.Error("upstream should not be called for negatively-cached cover")
	}
}

func TestCoverFetchAndCache(t *testing.T) {
	st := newStore(t)
	imgBytes := []byte{0xFF, 0xD8, 0xFF} // minimal JPEG

	covers := newMockCovers()
	covers.set("isbn:new", "large", imgBytes, "image/jpeg", nil)

	h := server.NewHandler(st, covers, discardLog, "")
	w := do(t, h, "GET", "/api/covers/isbn:new/large", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	// Cover should now be on disk.
	if !st.Exists(store.CoverPath("isbn:new", "large")) {
		t.Error("cover not written to disk cache after fetch")
	}
	// Second request should be served from disk without calling upstream.
	w2 := do(t, h, "GET", "/api/covers/isbn:new/large", nil)
	if w2.Code != http.StatusOK {
		t.Fatalf("second request status: %d", w2.Code)
	}
	if covers.calls.Load() != 1 {
		t.Errorf("upstream called %d times, want 1", covers.calls.Load())
	}
}

func TestCoverUpstreamNotFound(t *testing.T) {
	st := newStore(t)
	covers := newMockCovers()
	covers.set("isbn:gone", "large", nil, "", upstream.ErrNotFound)

	h := server.NewHandler(st, covers, discardLog, "")
	w := do(t, h, "GET", "/api/covers/isbn:gone/large", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	// Sentinel file should be written.
	if !st.Exists(store.CoverNotFoundPath("isbn:gone", "large")) {
		t.Error("negative-cache sentinel not written after upstream 404")
	}
}

func TestCoverUpstreamError(t *testing.T) {
	covers := newMockCovers()
	covers.set("isbn:err", "large", nil, "", errors.New("network error"))

	h := server.NewHandler(newStore(t), covers, discardLog, "")
	w := do(t, h, "GET", "/api/covers/isbn:err/large", nil)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

// TestCoverDedup verifies that concurrent requests for the same uncached cover
// result in at most a handful of upstream calls (not one per request).
func TestCoverDedup(t *testing.T) {
	imgBytes := make([]byte, 1024)
	covers := newMockCovers()
	covers.set("isbn:dedup", "large", imgBytes, "image/jpeg", nil)

	h := server.NewHandler(newStore(t), covers, discardLog, "")

	const goroutines = 40
	var wg sync.WaitGroup
	codes := make([]int, goroutines)
	start := make(chan struct{})

	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			w := do(t, h, "GET", "/api/covers/isbn:dedup/large", nil)
			codes[i] = w.Code
		}(i)
	}
	close(start)
	wg.Wait()

	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("[%d] status: %d", i, code)
		}
	}
	// Upstream calls should be far fewer than goroutine count.
	if n := covers.calls.Load(); n >= int64(goroutines) {
		t.Errorf("upstream called %d times for %d concurrent requests — dedup not working", n, goroutines)
	}
}

// TestMethodNotAllowed verifies non-GET requests are rejected.
func TestMethodNotAllowed(t *testing.T) {
	st := newStore(t)
	seedJSON(t, st, store.PublisherIndexPath(), transform.PublisherIndex{})

	h := server.NewHandler(st, newMockCovers(), discardLog, "")
	w := do(t, h, "POST", "/api/publishers", nil)

	// Go 1.22 mux returns 405 for wrong method on registered route.
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestUnknownRoute verifies unregistered paths return 404.
func TestUnknownRoute(t *testing.T) {
	h := server.NewHandler(newStore(t), newMockCovers(), discardLog, "")
	w := do(t, h, "GET", "/api/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// Ensure *store.Store satisfies the server's internal storeReader interface
// (compile-time check via blank assignment; the interface is unexported so we
// verify indirectly by passing a real store to NewHandler).
func TestStoreImplementsInterface(t *testing.T) {
	// If this compiles, the interface is satisfied.
	st := store.New(t.TempDir())
	_ = server.NewHandler(st, newMockCovers(), discardLog, "")
	if st == nil {
		t.Fatal("store should not be nil")
	}
}

// Make sure os.File field is readable by checking a file we wrote.
func TestFileReadableAfterSeed(t *testing.T) {
	st := newStore(t)
	want := []byte(`{"publishers":[]}`)
	st.WriteAtomic(store.PublisherIndexPath(), want) //nolint:errcheck

	f, err := st.Open(store.PublisherIndexPath())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, _ := io.ReadAll(f)
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Suppress unused import warnings by referencing os in test.
var _ = os.Stderr
