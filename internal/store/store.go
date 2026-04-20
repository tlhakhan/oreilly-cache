// Package store owns all on-disk cache I/O for oreilly-cache.
// It is the only package allowed to write to the cache directory.
// All other packages read via Read/Exists or receive an *os.File from Open.
package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// Store is the single owner of the on-disk cache directory.
// All writes go through WriteAtomic; concurrent HTTP readers may
// call Read or Open at any time without coordination.
type Store struct {
	Root string
}

func New(root string) *Store {
	return &Store{Root: root}
}

// WriteAtomic writes data to relPath under Root atomically:
// tmp (same dir as target) → fsync → rename.
// A failed or interrupted write never corrupts an existing file at relPath.
func (s *Store) WriteAtomic(relPath string, data []byte) error {
	abs := filepath.Join(s.Root, filepath.FromSlash(relPath))
	dir := filepath.Dir(abs)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("store: mkdir %s: %w", dir, err)
	}

	// tmp is created in the same directory as the target so that
	// os.Rename stays within one filesystem and is therefore atomic on POSIX.
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("store: create tmp: %w", err)
	}
	tmpName := tmp.Name()

	committed := false
	defer func() {
		if !committed {
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("store: write: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("store: fsync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("store: close: %w", err)
	}
	if err := os.Rename(tmpName, abs); err != nil {
		return fmt.Errorf("store: rename: %w", err)
	}

	committed = true
	return nil
}

// Read returns the contents of relPath under Root.
func (s *Store) Read(relPath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(relPath)))
}

// Open returns an *os.File for relPath, suitable for http.ServeContent.
// Caller is responsible for closing it.
func (s *Store) Open(relPath string) (*os.File, error) {
	return os.Open(filepath.Join(s.Root, filepath.FromSlash(relPath)))
}

// Exists reports whether relPath exists under Root.
func (s *Store) Exists(relPath string) bool {
	_, err := os.Stat(filepath.Join(s.Root, filepath.FromSlash(relPath)))
	return err == nil
}

// AbsPath returns the absolute filesystem path for relPath under Root.
func (s *Store) AbsPath(relPath string) string {
	return filepath.Join(s.Root, filepath.FromSlash(relPath))
}

// --- path helpers ---------------------------------------------------------
// These are pure functions so they can be used in both store and handler
// code without importing the full Store.

func PublisherIndexPath() string {
	return "publishers/index.json"
}

func PublisherPath(uuid string) string {
	return "publishers/by-uuid/" + uuid + ".json"
}

func PublisherRawPath(uuid string) string {
	return "publishers/by-uuid/" + uuid + ".raw.json"
}

func PublisherItemsPath(uuid string) string {
	return "publishers/by-uuid/" + uuid + "-items.json"
}

func PublisherItemsRawPath(uuid string) string {
	return "publishers/by-uuid/" + uuid + "-items.raw.json"
}

func PublisherItemsSkipPath(uuid string) string {
	return "publishers/by-uuid/" + uuid + "-items.skip"
}

func ItemPath(ourn string) string {
	return "items/by-ourn/" + ourn + ".json"
}

func ItemRawPath(ourn string) string {
	return "items/by-ourn/" + ourn + ".raw.json"
}

func CoverPath(identifier, size string) string {
	return "covers/" + identifier + "/" + size + ".jpg"
}

func CoverNotFoundPath(identifier, size string) string {
	return "covers/" + identifier + "/" + size + ".404"
}

func ItemTypeIndexPath(typeName string) string {
	return "items/by-type/" + typeName + ".json"
}

func LastScrapePath() string {
	return "meta/last-scrape.json"
}
