// Package transform converts raw upstream types into the shapes this service
// serves. All functions are pure: no I/O, no side effects.
package transform

import (
	"encoding/json"

	"oreilly-cache/internal/upstream"
)

// Publisher is the shape served from GET /publishers/{uuid}.
type Publisher struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
	Slug string `json:"slug,omitempty"`
}

// PublisherIndex is the shape served from GET /publishers.
type PublisherIndex struct {
	Publishers []Publisher `json:"publishers"`
}

// Item is the shape served from GET /items/{ourn} and embedded in ItemList.
type Item struct {
	OURN            string   `json:"ourn"`
	Name            string   `json:"name"`
	Type            string   `json:"type,omitempty"`
	PublicationDate string   `json:"publication_date"`
	Popularity      float64  `json:"popularity"`
	Authors         []string `json:"authors,omitempty"`
	Subjects        []string `json:"subjects,omitempty"`
	PublisherUUID   string   `json:"publisher_uuid,omitempty"`
}

// ItemList is the shape served from GET /publishers/{uuid}/items.
type ItemList struct {
	Items []Item `json:"items"`
}

// OnePublisher transforms a single upstream Publisher.
func OnePublisher(p upstream.Publisher) Publisher {
	return Publisher{
		UUID: p.UUID,
		Name: p.Name,
		Slug: stringFromRaw(p.Slug),
	}
}

// Publishers transforms a slice of upstream publishers into a PublisherIndex.
func Publishers(pp []upstream.Publisher) PublisherIndex {
	out := make([]Publisher, len(pp))
	for i, p := range pp {
		out[i] = OnePublisher(p)
	}
	return PublisherIndex{Publishers: out}
}

// OneItem transforms a single upstream Item.
func OneItem(item upstream.Item) Item {
	return Item{
		OURN:            item.OURN,
		Name:            item.Name,
		Type:            item.Type,
		PublicationDate: item.PublicationDate,
		Popularity:      item.Popularity,
		Authors:         namesFromRaw(item.Authors),
		Subjects:        namesFromRaw(item.Subjects),
		PublisherUUID:   item.PublisherUUID,
	}
}

// Items transforms a slice of upstream items into an ItemList.
func Items(items []upstream.Item) ItemList {
	out := make([]Item, len(items))
	for i, item := range items {
		out[i] = OneItem(item)
	}
	return ItemList{Items: out}
}

// stringFromRaw unmarshals a JSON string RawMessage. Returns "" on null or error.
func stringFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// namesFromRaw extracts the "name" field from each element of a JSON array.
// Returns nil on null, empty, or parse error — callers treat nil as "no data".
func namesFromRaw(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		if item.Name != "" {
			names = append(names, item.Name)
		}
	}
	return names
}
