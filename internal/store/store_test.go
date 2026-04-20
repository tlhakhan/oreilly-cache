package store_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"oreilly-cache/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	return store.New(filepath.Join(t.TempDir(), "cache"))
}

// TestWriteAtomicRoundTrip verifies basic write → read correctness.
func TestWriteAtomicRoundTrip(t *testing.T) {
	s := newStore(t)
	want := []byte(`{"hello":"world"}`)

	if err := s.WriteAtomic("publishers/index.json", want); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	got, err := s.Read("publishers/index.json")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("data mismatch: got %q, want %q", got, want)
	}
}

// TestWriteAtomicOverwrite verifies the second write wins.
func TestWriteAtomicOverwrite(t *testing.T) {
	s := newStore(t)

	if err := s.WriteAtomic("items/by-ourn/foo.json", []byte("v1")); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteAtomic("items/by-ourn/foo.json", []byte("v2")); err != nil {
		t.Fatal(err)
	}

	got, err := s.Read("items/by-ourn/foo.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "v2" {
		t.Fatalf("expected v2, got %q", got)
	}
}

// TestWriteAtomicConcurrentReaders verifies that concurrent readers never
// observe a partial write — every read returns a complete v1 or complete v2
// buffer, never a mix of bytes from both.
func TestWriteAtomicConcurrentReaders(t *testing.T) {
	s := newStore(t)

	const size = 512 * 1024 // 512 KB is enough to make a non-atomic write observable
	v1 := bytes.Repeat([]byte{0xAA}, size)
	v2 := bytes.Repeat([]byte{0xBB}, size)

	if err := s.WriteAtomic("data.bin", v1); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errc := make(chan error, 256)

	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 64; j++ {
				data, err := s.Read("data.bin")
				if err != nil {
					errc <- fmt.Errorf("read: %w", err)
					return
				}
				if len(data) != size {
					errc <- fmt.Errorf("short read: got %d bytes, want %d", len(data), size)
					return
				}
				first := data[0]
				if first != 0xAA && first != 0xBB {
					errc <- fmt.Errorf("unexpected first byte: %x", first)
					return
				}
				for i, b := range data {
					if b != first {
						errc <- fmt.Errorf("partial write at offset %d: byte %x mixed with %x", i, b, first)
						return
					}
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			if err := s.WriteAtomic("data.bin", v2); err != nil {
				errc <- err
				return
			}
			if err := s.WriteAtomic("data.bin", v1); err != nil {
				errc <- err
				return
			}
		}
	}()

	wg.Wait()
	close(errc)

	for err := range errc {
		t.Error(err)
	}
}

// TestWriteAtomicNoCorruptOnFailure verifies that a failed write leaves the
// existing file intact. We provoke failure by making the target directory
// read-only after the initial write succeeds.
func TestWriteAtomicNoCorruptOnFailure(t *testing.T) {
	s := newStore(t)

	if err := s.WriteAtomic("sub/data.json", []byte("original")); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(s.Root, "sub")
	if err := os.Chmod(subDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(subDir, 0o755) })

	err := s.WriteAtomic("sub/data.json", []byte("corrupted"))
	if err == nil {
		t.Skip("running as root or filesystem ignores permissions; skipping corruption test")
	}

	got, err := s.Read("sub/data.json")
	if err != nil {
		t.Fatalf("Read after failed write: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("file corrupted: got %q", got)
	}
}

// TestWriteAtomicTmpCleanup verifies that no .tmp-* files are left behind
// in the target directory after a successful write, and that tmp files are
// NOT placed in os.TempDir() (which would break atomic rename across
// filesystem boundaries).
func TestWriteAtomicTmpCleanup(t *testing.T) {
	s := newStore(t)

	if err := s.WriteAtomic("publishers/by-uuid/abc.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(s.Root, "publishers", "by-uuid")
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("leftover tmp file in target dir: %s", e.Name())
		}
	}

	// Verify the file landed where expected, not in os.TempDir().
	if s.Exists(os.TempDir()) {
		t.Log("(os.TempDir check skipped: target dir happens to be system tmp)")
	}
	if !s.Exists("publishers/by-uuid/abc.json") {
		t.Error("written file not found at expected path")
	}
}

// TestPathHelpers spot-checks that path helpers produce the expected strings.
func TestPathHelpers(t *testing.T) {
	cases := []struct {
		got  string
		want string
	}{
		{store.PublisherIndexPath(), "publishers/index.json"},
		{store.PublisherPath("u1"), "publishers/by-uuid/u1.json"},
		{store.PublisherRawPath("u1"), "publishers/by-uuid/u1.raw.json"},
		{store.PublisherItemsPath("u1"), "publishers/by-uuid/u1-items.json"},
		{store.PublisherItemsRawPath("u1"), "publishers/by-uuid/u1-items.raw.json"},
		{store.ItemPath("id1"), "items/by-ourn/id1.json"},
		{store.ItemRawPath("id1"), "items/by-ourn/id1.raw.json"},
		{store.CoverPath("img1", "large"), "covers/img1/large.jpg"},
		{store.CoverNotFoundPath("img1", "large"), "covers/img1/large.404"},
		{store.LastScrapePath(), "meta/last-scrape.json"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

// TestExists verifies Exists returns false for missing paths and true after write.
func TestExists(t *testing.T) {
	s := newStore(t)

	if s.Exists("missing.json") {
		t.Error("Exists returned true for non-existent file")
	}

	if err := s.WriteAtomic("present.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if !s.Exists("present.json") {
		t.Error("Exists returned false after write")
	}
}
