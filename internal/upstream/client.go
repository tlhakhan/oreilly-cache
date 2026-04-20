// Package upstream provides typed HTTP access to the O'Reilly Learning API.
// All functions accept a context.Context and return raw bytes alongside a
// parsed struct so callers can store both the .raw.json sidecar and the
// transformed shape without a second parse.
package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// Client calls the O'Reilly upstream API. BaseURL is configurable so tests
// can point at an httptest.Server without hitting the network.
type Client struct {
	base string
	http *http.Client
}

// New returns a Client. If httpClient is nil, http.DefaultClient is used.
func New(base string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{base: base, http: httpClient}
}

// PublishersURL builds the URL for a publishers page.
func (c *Client) PublishersURL(limit, offset int) string {
	u, _ := url.Parse(c.base + "/api/v1/publishers")
	q := u.Query()
	q.Set("limit", strconv.Itoa(limit))
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// PublisherItemsURL builds the URL for a publisher's items page.
func (c *Client) PublisherItemsURL(publisherUUID string, limit, offset int) string {
	u, _ := url.Parse(c.base + "/api/v2/metadata/")
	q := url.Values{}
	q.Set("publisher_uuid", publisherUUID)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	q.Set("sort", "-publication_date")
	q.Set("language", "en")
	u.RawQuery = q.Encode()
	return u.String()
}

// CoverURL builds the URL for a cover image.
func (c *Client) CoverURL(identifier, size string) string {
	return c.base + "/library/cover/" + identifier + "/" + size
}

// FetchPublishers fetches one page of publishers from pageURL.
// Returns raw response bytes (for the .raw.json sidecar) and the parsed page.
func (c *Client) FetchPublishers(ctx context.Context, pageURL string) ([]byte, *PublisherPage, error) {
	raw, err := c.get(ctx, pageURL)
	if err != nil {
		return nil, nil, err
	}
	var page PublisherPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return raw, nil, fmt.Errorf("upstream: parse publishers: %w", err)
	}
	return raw, &page, nil
}

// FetchPublisherItems fetches one page of items for a publisher from pageURL.
// Returns raw response bytes and the parsed page.
func (c *Client) FetchPublisherItems(ctx context.Context, pageURL string) ([]byte, *ItemPage, error) {
	raw, err := c.get(ctx, pageURL)
	if err != nil {
		return nil, nil, err
	}
	var page ItemPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return raw, nil, fmt.Errorf("upstream: parse items: %w", err)
	}
	return raw, &page, nil
}

// GetCover fetches a cover image by identifier and size.
// Returns the image bytes and the upstream Content-Type header.
// Returns ErrNotFound when the upstream responds with 404.
func (c *Client) GetCover(ctx context.Context, identifier, size string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.CoverURL(identifier, size), nil)
	if err != nil {
		return nil, "", fmt.Errorf("upstream: cover request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("upstream: cover fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("upstream: cover %s/%s: HTTP %d", identifier, size, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("upstream: read cover: %w", err)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("upstream: build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return nil, fmt.Errorf("upstream: %s: HTTP 400: %w", rawURL, ErrBadRequest)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream: %s: HTTP %d", rawURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("upstream: read body: %w", err)
	}
	return body, nil
}
