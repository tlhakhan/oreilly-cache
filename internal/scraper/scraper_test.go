package scraper_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"oreilly-cache/internal/scraper"
	"oreilly-cache/internal/store"
	"oreilly-cache/internal/transform"
	"oreilly-cache/internal/upstream"
)

// ---- mock store ----

type mockStore struct {
	mu      sync.Mutex
	written map[string][]byte
	exists  map[string]bool // pre-seeded existing paths
}

func newMockStore() *mockStore {
	return &mockStore{
		written: make(map[string][]byte),
		exists:  make(map[string]bool),
	}
}

func (m *mockStore) WriteAtomic(relPath string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	m.written[relPath] = cp
	return nil
}

func (m *mockStore) Exists(relPath string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.exists[relPath] {
		return true
	}
	_, ok := m.written[relPath]
	return ok
}

func (m *mockStore) Read(relPath string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b, ok := m.written[relPath]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("mockStore: not found: %s", relPath)
}

func (m *mockStore) get(relPath string) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.written[relPath]
}

// ---- mock client ----

// mockClient maps full URL strings to canned responses.
type mockClient struct {
	mu            sync.Mutex
	publisherResp map[string]publisherResp
	itemResp      map[string]itemResp
	base          string
}

type publisherResp struct {
	raw  []byte
	page *upstream.PublisherPage
	err  error
}

type itemResp struct {
	raw  []byte
	page *upstream.ItemPage
	err  error
}

func newMockClient() *mockClient {
	return &mockClient{
		publisherResp: make(map[string]publisherResp),
		itemResp:      make(map[string]itemResp),
		base:          "https://mock.test",
	}
}

func (c *mockClient) PublishersURL(limit, offset int) string {
	return fmt.Sprintf("%s/publishers?limit=%d&offset=%d", c.base, limit, offset)
}

func (c *mockClient) PublisherItemsURL(uuid string, limit, offset int) string {
	return fmt.Sprintf("%s/items?publisher_uuid=%s&limit=%d&offset=%d", c.base, uuid, limit, offset)
}

func (c *mockClient) FetchPublishers(ctx context.Context, url string) ([]byte, *upstream.PublisherPage, error) {
	c.mu.Lock()
	r, ok := c.publisherResp[url]
	c.mu.Unlock()
	if !ok {
		return nil, nil, fmt.Errorf("mockClient: no publisher response for %s", url)
	}
	return r.raw, r.page, r.err
}

func (c *mockClient) FetchPublisherItems(ctx context.Context, url string) ([]byte, *upstream.ItemPage, error) {
	c.mu.Lock()
	r, ok := c.itemResp[url]
	c.mu.Unlock()
	if !ok {
		return nil, nil, fmt.Errorf("mockClient: no item response for %s", url)
	}
	return r.raw, r.page, r.err
}

func (c *mockClient) setPublishers(url string, publishers []upstream.Publisher, next string) {
	page := &upstream.PublisherPage{Count: len(publishers), Next: next, Results: publishers}
	raw, _ := json.Marshal(map[string]any{
		"count":   len(publishers),
		"next":    next,
		"results": publishers,
	})
	c.publisherResp[url] = publisherResp{raw: raw, page: page}
}

func (c *mockClient) setItems(url string, items []upstream.Item, next string) {
	page := &upstream.ItemPage{Count: len(items), Next: next, Results: items}
	raw, _ := json.Marshal(map[string]any{
		"count":   len(items),
		"next":    next,
		"results": items,
	})
	c.itemResp[url] = itemResp{raw: raw, page: page}
}

// ---- helpers ----

var discardLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))

func newScraper(st *mockStore, cl *mockClient) *scraper.Scraper {
	return scraper.New(st, cl, discardLog, scraper.Config{Workers: 2, PageSize: 10})
}

func pub(uuid, name string) upstream.Publisher {
	return upstream.Publisher{UUID: uuid, Name: name, Slug: json.RawMessage(`"` + uuid + `"`), URL: "https://example.com/" + uuid + "/"}
}

func item(ourn, id, date string) upstream.Item {
	return upstream.Item{OURN: ourn, Name: "Title " + id, PublicationDate: date}
}

// ---- tests ----

func TestScrapePublishersWritesFiles(t *testing.T) {
	cl := newMockClient()
	st := newMockStore()

	cl.setPublishers(cl.PublishersURL(10, 0), []upstream.Publisher{
		pub("uuid-1", "Publisher One"),
		pub("uuid-2", "Publisher Two"),
	}, "")

	// Items: no items for either publisher.
	cl.setItems(cl.PublisherItemsURL("uuid-1", 10, 0), nil, "")
	cl.setItems(cl.PublisherItemsURL("uuid-2", 10, 0), nil, "")

	if err := newScraper(st, cl).Scrape(context.Background()); err != nil {
		t.Fatalf("Scrape: %v", err)
	}

	// Individual publisher files.
	for _, uuid := range []string{"uuid-1", "uuid-2"} {
		if st.get(store.PublisherPath(uuid)) == nil {
			t.Errorf("missing publisher file for %s", uuid)
		}
		if st.get(store.PublisherRawPath(uuid)) == nil {
			t.Errorf("missing publisher raw file for %s", uuid)
		}
	}

	// Publisher index written as commit point.
	indexBytes := st.get(store.PublisherIndexPath())
	if indexBytes == nil {
		t.Fatal("publisher index not written")
	}
	var index transform.PublisherIndex
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		t.Fatalf("parse index: %v", err)
	}
	if len(index.Publishers) != 2 {
		t.Errorf("index publisher count: got %d, want 2", len(index.Publishers))
	}

	// last-scrape.json written.
	if st.get(store.LastScrapePath()) == nil {
		t.Error("last-scrape.json not written")
	}
}

func TestScrapePublishersPagination(t *testing.T) {
	cl := newMockClient()
	st := newMockStore()

	page2URL := cl.base + "/publishers?limit=10&offset=10"
	cl.setPublishers(cl.PublishersURL(10, 0), []upstream.Publisher{pub("p1", "One")}, page2URL)
	cl.setPublishers(page2URL, []upstream.Publisher{pub("p2", "Two")}, "")
	cl.setItems(cl.PublisherItemsURL("p1", 10, 0), nil, "")
	cl.setItems(cl.PublisherItemsURL("p2", 10, 0), nil, "")

	if err := newScraper(st, cl).Scrape(context.Background()); err != nil {
		t.Fatal(err)
	}

	indexBytes := st.get(store.PublisherIndexPath())
	var index transform.PublisherIndex
	json.Unmarshal(indexBytes, &index) //nolint:errcheck
	if len(index.Publishers) != 2 {
		t.Errorf("index should have 2 publishers after pagination, got %d", len(index.Publishers))
	}
}

func TestScrapeItemsFullWrite(t *testing.T) {
	cl := newMockClient()
	st := newMockStore()

	cl.setPublishers(cl.PublishersURL(10, 0), []upstream.Publisher{pub("pub-1", "Pub")}, "")
	cl.setItems(cl.PublisherItemsURL("pub-1", 10, 0), []upstream.Item{
		item("urn:a", "id-a", "2024-01-01"),
		item("urn:b", "id-b", "2023-01-01"),
	}, "")

	if err := newScraper(st, cl).Scrape(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Individual item files (keyed by OURN).
	for _, ourn := range []string{"urn:a", "urn:b"} {
		if st.get(store.ItemPath(ourn)) == nil {
			t.Errorf("missing item file for %s", ourn)
		}
		if st.get(store.ItemRawPath(ourn)) == nil {
			t.Errorf("missing item raw file for %s", ourn)
		}
	}

	// Items list for publisher.
	listBytes := st.get(store.PublisherItemsPath("pub-1"))
	if listBytes == nil {
		t.Fatal("items list not written")
	}
	var list transform.ItemList
	json.Unmarshal(listBytes, &list) //nolint:errcheck
	if len(list.Items) != 2 {
		t.Errorf("items list count: got %d, want 2", len(list.Items))
	}
}

func TestScrapeItemsDeltaStopsAtKnown(t *testing.T) {
	cl := newMockClient()
	st := newMockStore()

	cl.setPublishers(cl.PublishersURL(10, 0), []upstream.Publisher{pub("pub-1", "Pub")}, "")
	// Three items; second page URL would error if called.
	cl.setItems(cl.PublisherItemsURL("pub-1", 10, 0), []upstream.Item{
		item("urn:new", "id-new", "2024-06-01"),
		item("urn:old", "id-old", "2023-01-01"), // already stored
	}, cl.base+"/items/page2-should-not-be-called")

	// Pre-seed the store: urn:old is already present.
	st.exists[store.ItemPath("urn:old")] = true

	if err := newScraper(st, cl).Scrape(context.Background()); err != nil {
		t.Fatal(err)
	}

	// New item written.
	if st.get(store.ItemPath("urn:new")) == nil {
		t.Error("new item should have been written")
	}
	// Known item NOT overwritten (only in exists, not in written).
	if st.get(store.ItemPath("urn:old")) != nil {
		t.Error("known item should not have been re-written")
	}
}

func TestScrapeItemsDeltaMergesWithExisting(t *testing.T) {
	cl := newMockClient()
	st := newMockStore()

	cl.setPublishers(cl.PublishersURL(10, 0), []upstream.Publisher{pub("pub-1", "Pub")}, "")
	cl.setItems(cl.PublisherItemsURL("pub-1", 10, 0), []upstream.Item{
		item("urn:new", "id-new", "2024-06-01"),
		item("urn:old", "id-old", "2023-01-01"), // triggers delta stop
	}, "")

	// Pre-seed existing items list and mark old item as present.
	oldList := transform.ItemList{Items: []transform.Item{
		{OURN: "urn:old", Name: "Old Book"},
	}}
	existingBytes, _ := json.Marshal(oldList)
	st.written[store.PublisherItemsPath("pub-1")] = existingBytes
	st.exists[store.ItemPath("urn:old")] = true

	if err := newScraper(st, cl).Scrape(context.Background()); err != nil {
		t.Fatal(err)
	}

	listBytes := st.get(store.PublisherItemsPath("pub-1"))
	var list transform.ItemList
	json.Unmarshal(listBytes, &list) //nolint:errcheck

	// Should have both new and old items.
	if len(list.Items) != 2 {
		t.Fatalf("merged list: got %d items, want 2", len(list.Items))
	}
	// New item is first (newest first).
	if list.Items[0].OURN != "urn:new" {
		t.Errorf("first item should be urn:new, got %s", list.Items[0].OURN)
	}
	if list.Items[1].OURN != "urn:old" {
		t.Errorf("second item should be urn:old, got %s", list.Items[1].OURN)
	}
}

func TestScrapeItemsMultiPageFull(t *testing.T) {
	cl := newMockClient()
	st := newMockStore()

	page2URL := cl.base + "/items/page2"
	cl.setPublishers(cl.PublishersURL(10, 0), []upstream.Publisher{pub("pub-1", "Pub")}, "")
	cl.setItems(cl.PublisherItemsURL("pub-1", 10, 0), []upstream.Item{
		item("urn:a", "id-a", "2024-01-01"),
	}, page2URL)
	cl.setItems(page2URL, []upstream.Item{
		item("urn:b", "id-b", "2023-01-01"),
	}, "")

	if err := newScraper(st, cl).Scrape(context.Background()); err != nil {
		t.Fatal(err)
	}

	listBytes := st.get(store.PublisherItemsPath("pub-1"))
	var list transform.ItemList
	json.Unmarshal(listBytes, &list) //nolint:errcheck
	if len(list.Items) != 2 {
		t.Errorf("full multi-page list: got %d items, want 2", len(list.Items))
	}
}

func TestScrapeResultWritten(t *testing.T) {
	cl := newMockClient()
	st := newMockStore()
	cl.setPublishers(cl.PublishersURL(10, 0), []upstream.Publisher{pub("p1", "P")}, "")
	cl.setItems(cl.PublisherItemsURL("p1", 10, 0), []upstream.Item{item("u", "id1", "2024-01-01")}, "")

	if err := newScraper(st, cl).Scrape(context.Background()); err != nil {
		t.Fatal(err)
	}

	b := st.get(store.LastScrapePath())
	if b == nil {
		t.Fatal("last-scrape.json not written")
	}
	var result scraper.ScrapeResult
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if result.PublisherCount != 1 {
		t.Errorf("publisher_count: got %d, want 1", result.PublisherCount)
	}
	if result.ItemCount != 1 {
		t.Errorf("item_count: got %d, want 1", result.ItemCount)
	}
	if result.StartedAt.IsZero() || result.FinishedAt.IsZero() {
		t.Error("timestamps should be set")
	}
}

func TestScrapeSkipsItemsForInactivePublisher(t *testing.T) {
	cl := newMockClient()
	st := newMockStore()

	inactive := false
	active := true

	cl.setPublishers(cl.PublishersURL(10, 0), []upstream.Publisher{
		{UUID: "active-pub", Name: "Active", IsActive: &active, URL: "https://example.com/active/"},
		{UUID: "inactive-pub", Name: "Inactive", IsActive: &inactive, URL: "https://example.com/inactive/"},
	}, "")
	cl.setItems(cl.PublisherItemsURL("active-pub", 10, 0), []upstream.Item{
		item("urn:x", "id-x", "2024-01-01"),
	}, "")
	// No item response registered for inactive-pub; if queried it would error.

	if err := newScraper(st, cl).Scrape(context.Background()); err != nil {
		t.Fatalf("Scrape: %v", err)
	}

	// Only the active publisher appears in the index.
	indexBytes := st.get(store.PublisherIndexPath())
	var index transform.PublisherIndex
	json.Unmarshal(indexBytes, &index) //nolint:errcheck
	if len(index.Publishers) != 1 || index.Publishers[0].UUID != "active-pub" {
		t.Errorf("index should contain only active publisher, got %+v", index.Publishers)
	}

	// No files written for the inactive publisher.
	if st.get(store.PublisherPath("inactive-pub")) != nil {
		t.Error("inactive publisher file should not be written")
	}
	if st.get(store.PublisherRawPath("inactive-pub")) != nil {
		t.Error("inactive publisher raw file should not be written")
	}

	// Items fetched for the active publisher.
	if st.get(store.ItemPath("urn:x")) == nil {
		t.Error("active publisher item not written")
	}
}

func TestScrapeSkipsNonWhitelistedPublisher(t *testing.T) {
	cl := newMockClient()
	st := newMockStore()

	f := false
	cl.setPublishers(cl.PublishersURL(10, 0), []upstream.Publisher{
		pub("allowed", "Allowed"),
		{UUID: "blocked", Name: "Blocked", URL: "https://example.com/blocked/", IsWhiteListed: &f},
	}, "")
	cl.setItems(cl.PublisherItemsURL("allowed", 10, 0), nil, "")

	if err := newScraper(st, cl).Scrape(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st.get(store.PublisherPath("blocked")) != nil {
		t.Error("non-whitelisted publisher should not be written")
	}
	if st.get(store.PublisherPath("allowed")) == nil {
		t.Error("whitelisted publisher should be written")
	}
}

func TestScrapeSkipsPublisherWithoutURL(t *testing.T) {
	cl := newMockClient()
	st := newMockStore()

	cl.setPublishers(cl.PublishersURL(10, 0), []upstream.Publisher{
		{UUID: "has-url", Name: "With URL", URL: "https://example.com/pub/"},
		{UUID: "no-url", Name: "No URL", URL: ""},
	}, "")
	cl.setItems(cl.PublisherItemsURL("has-url", 10, 0), nil, "")

	if err := newScraper(st, cl).Scrape(context.Background()); err != nil {
		t.Fatal(err)
	}

	if st.get(store.PublisherPath("no-url")) != nil {
		t.Error("publisher without url should not be written")
	}
	if st.get(store.PublisherPath("has-url")) == nil {
		t.Error("publisher with url should be written")
	}
	indexBytes := st.get(store.PublisherIndexPath())
	var index transform.PublisherIndex
	json.Unmarshal(indexBytes, &index) //nolint:errcheck
	if len(index.Publishers) != 1 || index.Publishers[0].UUID != "has-url" {
		t.Errorf("index should contain only publisher with url, got %+v", index.Publishers)
	}
}

func TestScrapePublisherWithMissingIsActiveFieldTreatedAsActive(t *testing.T) {
	cl := newMockClient()
	st := newMockStore()

	// IsActive is nil (field absent in upstream JSON).
	cl.setPublishers(cl.PublishersURL(10, 0), []upstream.Publisher{
		{UUID: "legacy-pub", Name: "Legacy", IsActive: nil, URL: "https://example.com/legacy/"},
	}, "")
	cl.setItems(cl.PublisherItemsURL("legacy-pub", 10, 0), []upstream.Item{
		item("urn:y", "id-y", "2023-01-01"),
	}, "")

	if err := newScraper(st, cl).Scrape(context.Background()); err != nil {
		t.Fatal(err)
	}

	if st.get(store.ItemPath("urn:y")) == nil {
		t.Error("publisher with absent is_active should be treated as active")
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	cl := newMockClient()
	st := newMockStore()
	cl.setPublishers(cl.PublishersURL(10, 0), []upstream.Publisher{}, "")

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		newScraper(st, cl).Run(ctx, 10*time.Second)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}
