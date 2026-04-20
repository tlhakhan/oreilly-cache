// Package scraper fetches upstream data and writes it to the on-disk cache.
// It owns the scrape lifecycle: one-shot Scrape and the recurring Run loop.
package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"oreilly-cache/internal/store"
	"oreilly-cache/internal/transform"
	"oreilly-cache/internal/upstream"
)

const defaultWorkers  = 5
const defaultPageSize = 100

// upstreamClient is the subset of upstream.Client the scraper needs.
// Defined here so the scraper can be tested without a real HTTP client.
type upstreamClient interface {
	PublishersURL(limit, offset int) string
	FetchPublishers(ctx context.Context, url string) ([]byte, *upstream.PublisherPage, error)
	PublisherItemsURL(publisherUUID string, limit, offset int) string
	FetchPublisherItems(ctx context.Context, url string) ([]byte, *upstream.ItemPage, error)
}

// diskStore is the subset of store.Store the scraper needs.
type diskStore interface {
	WriteAtomic(relPath string, data []byte) error
	Exists(relPath string) bool
	Read(relPath string) ([]byte, error)
}

// ScrapeResult is written to meta/last-scrape.json after each cycle.
type ScrapeResult struct {
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	PublisherCount int       `json:"publisher_count"`
	ItemCount      int       `json:"item_count"`
	Errors         []string  `json:"errors,omitempty"`
}

// Config holds tunables for the Scraper.
type Config struct {
	Workers  int // max concurrent publisher-item scrapes; 0 → 5
	PageSize int // items per upstream page request; 0 → 100
}

// Scraper fetches upstream data and writes it to the disk cache.
type Scraper struct {
	store    diskStore
	client   upstreamClient
	log      *slog.Logger
	workers  int
	pageSize int
}

func New(st diskStore, cl upstreamClient, log *slog.Logger, cfg Config) *Scraper {
	if cfg.Workers <= 0 {
		cfg.Workers = defaultWorkers
	}
	if cfg.PageSize <= 0 {
		cfg.PageSize = defaultPageSize
	}
	return &Scraper{
		store:    st,
		client:   cl,
		log:      log,
		workers:  cfg.Workers,
		pageSize: cfg.PageSize,
	}
}

// Run performs an initial scrape then repeats on interval until ctx is cancelled.
func (s *Scraper) Run(ctx context.Context, interval time.Duration) {
	s.log.Info("scraper starting", "interval", interval)
	s.runOnce(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.log.Info("scraper stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scraper) runOnce(ctx context.Context) {
	if err := s.Scrape(ctx); err != nil {
		s.log.Error("scrape failed", "err", err)
	}
}

// Scrape performs one full scrape cycle: all publishers, then their items.
// Per-publisher item errors are logged and recorded but do not abort other
// publishers. Returns non-nil only when the publisher list itself cannot be
// fetched (the cache would be left in its last-good state).
func (s *Scraper) Scrape(ctx context.Context) error {
	result := &ScrapeResult{StartedAt: time.Now()}

	publishers, err := s.scrapePublishers(ctx)
	if err != nil {
		return fmt.Errorf("scraper: publishers: %w", err)
	}
	result.PublisherCount = len(publishers)

	s.scrapeAllItems(ctx, publishers, result)

	result.FinishedAt = time.Now()
	s.writeResult(result)
	s.log.Info("scrape complete",
		"publishers", result.PublisherCount,
		"items", result.ItemCount,
		"errors", len(result.Errors),
		"duration", result.FinishedAt.Sub(result.StartedAt))
	return nil
}

// scrapePublishers fetches all publisher pages, writes per-publisher files,
// and writes the publisher index as the commit point (written last so a
// partial scrape leaves the previous index intact).
// scrapePublishers fetches all publisher pages, writes per-publisher files,
// and writes the publisher index as the commit point (written last so a
// partial scrape leaves the previous index intact).
// Returns only the active publishers; inactive ones are written to disk but
// excluded from the items-scrape queue.
func (s *Scraper) scrapePublishers(ctx context.Context) ([]upstream.Publisher, error) {
	var active []upstream.Publisher
	pageURL := s.client.PublishersURL(s.pageSize, 0)

	for pageURL != "" {
		raw, page, err := s.client.FetchPublishers(ctx, pageURL)
		if err != nil {
			return nil, err
		}

		rawResults, err := extractRawResults(raw)
		if err != nil {
			return nil, fmt.Errorf("extract publisher results: %w", err)
		}

		for i, p := range page.Results {
			// Skip publishers that fail any eligibility check.
			if p.IsActive != nil && !*p.IsActive {
				s.log.Info("skipping inactive publisher", "uuid", p.UUID)
				continue
			}
			if p.URL == "" {
				s.log.Info("skipping publisher without url", "uuid", p.UUID)
				continue
			}
			if p.IsWhiteListed != nil && !*p.IsWhiteListed {
				s.log.Info("skipping non-whitelisted publisher", "uuid", p.UUID)
				continue
			}
			if i < len(rawResults) {
				if err := s.store.WriteAtomic(store.PublisherRawPath(p.UUID), rawResults[i]); err != nil {
					s.log.Error("write publisher raw", "uuid", p.UUID, "err", err)
				}
			}
			b, _ := json.Marshal(transform.OnePublisher(p))
			if err := s.store.WriteAtomic(store.PublisherPath(p.UUID), b); err != nil {
				s.log.Error("write publisher", "uuid", p.UUID, "err", err)
			}
			active = append(active, p)
		}
		pageURL = page.Next
	}

	// Index includes all publishers regardless of active status.
	// Written last as the commit point.
	b, _ := json.Marshal(transform.Publishers(active))
	if err := s.store.WriteAtomic(store.PublisherIndexPath(), b); err != nil {
		return nil, fmt.Errorf("write publisher index: %w", err)
	}

	return active, nil
}

// scrapeAllItems fans out one goroutine per publisher, bounded by s.workers.
func (s *Scraper) scrapeAllItems(ctx context.Context, publishers []upstream.Publisher, result *ScrapeResult) {
	sem := make(chan struct{}, s.workers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, pub := range publishers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			n, err := s.scrapePublisherItems(ctx, pub.UUID)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				s.log.Error("items scrape failed", "publisher", pub.UUID, "err", err)
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", pub.UUID, err))
				return
			}
			result.ItemCount += n
		}()
	}
	wg.Wait()
}

// scrapePublisherItems fetches items for one publisher using delta detection:
// upstream items are sorted -publication_date, so when we hit an identifier
// already on disk we know all subsequent items are already stored.
// Returns the number of newly written items.
func (s *Scraper) scrapePublisherItems(ctx context.Context, publisherUUID string) (int, error) {
	var newItems []upstream.Item
	var newRaw []json.RawMessage
	stoppedEarly := false

	pageURL := s.client.PublisherItemsURL(publisherUUID, s.pageSize, 0)
	for pageURL != "" {
		raw, page, err := s.client.FetchPublisherItems(ctx, pageURL)
		if err != nil {
			return 0, err
		}

		rawResults, err := extractRawResults(raw)
		if err != nil {
			return 0, fmt.Errorf("extract item results: %w", err)
		}

		for i, item := range page.Results {
			id := itemID(item)
			if s.store.Exists(store.ItemPath(id)) {
				stoppedEarly = true
				break
			}
			newItems = append(newItems, item)
			if i < len(rawResults) {
				newRaw = append(newRaw, rawResults[i])
			}
		}

		if stoppedEarly {
			break
		}
		pageURL = page.Next
	}

	for i, item := range newItems {
		id := itemID(item)
		if i < len(newRaw) {
			if err := s.store.WriteAtomic(store.ItemRawPath(id), newRaw[i]); err != nil {
				s.log.Error("write item raw", "id", id, "err", err)
			}
		}
		b, _ := json.Marshal(transform.OneItem(item))
		if err := s.store.WriteAtomic(store.ItemPath(id), b); err != nil {
			s.log.Error("write item", "id", id, "err", err)
		}
	}

	if err := s.writeItemsList(publisherUUID, newItems, stoppedEarly); err != nil {
		return len(newItems), err
	}
	return len(newItems), nil
}

// writeItemsList writes the publisher items list. On a delta scrape
// (stoppedEarly=true) it prepends new items to the existing list so the
// stored list remains complete.
func (s *Scraper) writeItemsList(publisherUUID string, newItems []upstream.Item, stoppedEarly bool) error {
	items := transform.Items(newItems).Items

	if stoppedEarly {
		if raw, err := s.store.Read(store.PublisherItemsPath(publisherUUID)); err == nil {
			var prev transform.ItemList
			if err := json.Unmarshal(raw, &prev); err == nil {
				items = append(items, prev.Items...)
			}
		}
	}

	b, _ := json.Marshal(transform.ItemList{Items: items})
	return s.store.WriteAtomic(store.PublisherItemsPath(publisherUUID), b)
}

func (s *Scraper) writeResult(result *ScrapeResult) {
	b, _ := json.Marshal(result)
	if err := s.store.WriteAtomic(store.LastScrapePath(), b); err != nil {
		s.log.Error("write last-scrape", "err", err)
	}
}

func itemID(item upstream.Item) string { return item.OURN }

// extractRawResults pulls per-element raw JSON bytes from a paginated
// upstream response body without discarding unknown fields.
func extractRawResults(body []byte) ([]json.RawMessage, error) {
	var page struct {
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, err
	}
	return page.Results, nil
}
