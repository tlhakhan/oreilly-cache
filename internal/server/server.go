// Package server implements the HTTP API for oreilly-cache.
// URL shape and disk layout are intentionally decoupled: handlers bridge them.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"golang.org/x/sync/singleflight"

	"oreilly-cache/internal/store"
	"oreilly-cache/internal/upstream"
)

// storeReader is the subset of store.Store the server needs.
type storeReader interface {
	Open(relPath string) (*os.File, error)
	Exists(relPath string) bool
	Read(relPath string) ([]byte, error)
	WriteAtomic(relPath string, data []byte) error
}

// coverClient is the subset of upstream.Client used for lazy cover fetching.
type coverClient interface {
	GetCover(ctx context.Context, identifier, size string) ([]byte, string, error)
}

type coverResult struct {
	data []byte
	ct   string
}

type server struct {
	store    storeReader
	covers   coverClient
	log      *slog.Logger
	inflight singleflight.Group
	start    time.Time
}

// NewHandler wires routes and returns an http.Handler ready to serve.
func NewHandler(st storeReader, covers coverClient, log *slog.Logger) http.Handler {
	s := &server{store: st, covers: covers, log: log, start: time.Now()}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /publishers", s.handlePublisherIndex)
	mux.HandleFunc("GET /publishers/{uuid}", s.handlePublisher)
	mux.HandleFunc("GET /publishers/{uuid}/items", s.handlePublisherItems)
	mux.HandleFunc("GET /items/{id}", s.handleItem)
	mux.HandleFunc("GET /covers/{identifier}/{size}", s.handleCover)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	return mux
}

func (s *server) handlePublisherIndex(w http.ResponseWriter, r *http.Request) {
	s.serveJSONFile(w, r, store.PublisherIndexPath())
}

func (s *server) handlePublisher(w http.ResponseWriter, r *http.Request) {
	s.serveJSONFile(w, r, store.PublisherPath(r.PathValue("uuid")))
}

func (s *server) handlePublisherItems(w http.ResponseWriter, r *http.Request) {
	s.serveJSONFile(w, r, store.PublisherItemsPath(r.PathValue("uuid")))
}

func (s *server) handleItem(w http.ResponseWriter, r *http.Request) {
	s.serveJSONFile(w, r, store.ItemPath(r.PathValue("id")))
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	type response struct {
		Status  string          `json:"status"`
		Uptime  string          `json:"uptime"`
		Scrape  json.RawMessage `json:"scrape"`
	}
	resp := response{
		Status: "ok",
		Uptime: time.Since(s.start).Truncate(time.Second).String(),
	}
	if b, err := s.store.Read(store.LastScrapePath()); err == nil {
		resp.Scrape = json.RawMessage(b)
	}
	b, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(b) //nolint:errcheck
}

// handleCover serves cover images with lazy upstream fetching.
// On a cache miss it deduplicates concurrent requests via singleflight,
// fetches from upstream, writes to disk, then serves.
// 404s are negative-cached with a .404 sentinel file.
func (s *server) handleCover(w http.ResponseWriter, r *http.Request) {
	identifier := r.PathValue("identifier")
	size := r.PathValue("size")

	coverPath := store.CoverPath(identifier, size)
	notFoundPath := store.CoverNotFoundPath(identifier, size)

	// Fast path: serve from disk cache.
	if f, err := s.store.Open(coverPath); err == nil {
		defer f.Close()
		fi, _ := f.Stat()
		http.ServeContent(w, r, size+".jpg", fi.ModTime(), f)
		return
	}

	// Negative-cache check: a previous request confirmed upstream has no cover.
	if s.store.Exists(notFoundPath) {
		http.NotFound(w, r)
		return
	}

	// Cache miss: dedup concurrent fetches for the same cover.
	key := identifier + "/" + size
	val, err, _ := s.inflight.Do(key, func() (any, error) {
		data, ct, err := s.covers.GetCover(r.Context(), identifier, size)
		return coverResult{data: data, ct: ct}, err
	})
	result, _ := val.(coverResult)

	if errors.Is(err, upstream.ErrNotFound) {
		// Write a zero-byte sentinel so subsequent requests skip upstream.
		if werr := s.store.WriteAtomic(notFoundPath, []byte{}); werr != nil {
			s.log.Error("write cover sentinel", "path", notFoundPath, "err", werr)
		}
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("fetch cover", "identifier", identifier, "size", size, "err", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	// Write to disk so subsequent requests hit the fast path.
	if werr := s.store.WriteAtomic(coverPath, result.data); werr != nil {
		s.log.Error("cache cover", "path", coverPath, "err", werr)
	}

	// Serve from disk so http.ServeContent can set proper cache headers.
	if f, err := s.store.Open(coverPath); err == nil {
		defer f.Close()
		fi, _ := f.Stat()
		http.ServeContent(w, r, size+".jpg", fi.ModTime(), f)
		return
	}

	// Fallback: write failed or file vanished; serve in-memory bytes directly.
	if result.ct != "" {
		w.Header().Set("Content-Type", result.ct)
	}
	w.Write(result.data) //nolint:errcheck
}

// serveJSONFile opens relPath from the store and serves it via http.ServeContent,
// which handles If-Modified-Since / ETag conditional GETs automatically.
func (s *server) serveJSONFile(w http.ResponseWriter, r *http.Request, relPath string) {
	f, err := s.store.Open(relPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		s.log.Error("open cache file", "path", relPath, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		s.log.Error("stat cache file", "path", relPath, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	http.ServeContent(w, r, relPath, fi.ModTime(), f)
}
