package upstream

import (
	"encoding/json"
	"errors"
)

// ErrNotFound is returned when the upstream responds with 404.
var ErrNotFound = errors.New("upstream: not found")

// PublisherPage is one paginated response from the publishers endpoint.
type PublisherPage struct {
	Count   int         `json:"count"`
	Next    string      `json:"next"`
	Results []Publisher `json:"results"`
}

// Publisher holds the fields the scraper and transform layer actively use.
// Everything else is preserved byte-for-byte in the .raw.json sidecar.
type Publisher struct {
	UUID     string          `json:"uuid"`
	Name     string          `json:"name"`
	Slug     json.RawMessage `json:"slug"`
	URL      string          `json:"url"`
	IsActive      *bool `json:"is_active"`       // nil = field absent, treat as active
	IsWhiteListed *bool `json:"is_white_listed"` // nil = field absent, treat as whitelisted
}

// ItemPage is one paginated response from the metadata endpoint.
type ItemPage struct {
	Count   int    `json:"count"`
	Next    string `json:"next"`
	Results []Item `json:"results"`
}

// Item holds the fields needed for delta detection and transformation.
// PublicationDate is used by the scraper to stop paging on re-scrapes once
// it reaches an already-stored OURN.
type Item struct {
	OURN            string          `json:"ourn"`
	Name            string          `json:"name"`
	PublicationDate string          `json:"publication_date"`
	Authors         json.RawMessage `json:"authors"`
	Subjects        json.RawMessage `json:"subjects"`
	PublisherUUID   string          `json:"publisher_uuid"`
}
