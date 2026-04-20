package upstream_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oreilly-cache/internal/upstream"
)

// serve returns a test server that responds with status and body for the given path prefix.
func serve(t *testing.T, path string, status int, body string, extraHeaders map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, path) {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusInternalServerError)
			return
		}
		for k, v := range extraHeaders {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		w.Write([]byte(body)) //nolint:errcheck
	}))
}

// --- URL builders ---

func TestPublishersURL(t *testing.T) {
	c := upstream.New("https://example.com", nil)

	u := c.PublishersURL(50, 0)
	if !strings.Contains(u, "/api/v1/publishers") {
		t.Errorf("missing path: %s", u)
	}
	if !strings.Contains(u, "limit=50") {
		t.Errorf("missing limit: %s", u)
	}
	// offset=0 should be omitted
	if strings.Contains(u, "offset") {
		t.Errorf("offset=0 should not appear: %s", u)
	}

	u2 := c.PublishersURL(50, 100)
	if !strings.Contains(u2, "offset=100") {
		t.Errorf("missing offset: %s", u2)
	}
}

func TestPublisherItemsURL(t *testing.T) {
	c := upstream.New("https://example.com", nil)
	u := c.PublisherItemsURL("uuid-123", 100, 0)

	for _, want := range []string{
		"/api/v2/metadata/",
		"publisher_uuid=uuid-123",
		"limit=100",
		"type=book",
		"sort=-publication_date",
		"language=en",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("URL %q missing %q", u, want)
		}
	}
}

func TestCoverURL(t *testing.T) {
	c := upstream.New("https://example.com", nil)
	u := c.CoverURL("isbn:9781234", "large")
	want := "https://example.com/library/cover/isbn:9781234/large"
	if u != want {
		t.Errorf("got %q, want %q", u, want)
	}
}

// --- FetchPublishers ---

func TestFetchPublishers(t *testing.T) {
	const fixture = `{
		"count": 2,
		"next": "https://example.com/api/v1/publishers?limit=1&offset=1",
		"results": [
			{"uuid": "pub-1", "name": "Publisher One"},
			{"uuid": "pub-2", "name": "Publisher Two"}
		]
	}`
	srv := serve(t, "/api/v1/publishers", http.StatusOK, fixture, nil)
	defer srv.Close()

	c := upstream.New(srv.URL, nil)
	raw, page, err := c.FetchPublishers(context.Background(), c.PublishersURL(2, 0))
	if err != nil {
		t.Fatalf("FetchPublishers: %v", err)
	}

	if len(raw) == 0 {
		t.Error("expected non-empty raw bytes")
	}
	if page.Count != 2 {
		t.Errorf("count: got %d, want 2", page.Count)
	}
	if page.Next == "" {
		t.Error("expected Next URL")
	}
	if len(page.Results) != 2 {
		t.Fatalf("results: got %d, want 2", len(page.Results))
	}
	if page.Results[0].UUID != "pub-1" {
		t.Errorf("uuid: got %q, want pub-1", page.Results[0].UUID)
	}
	if page.Results[1].Name != "Publisher Two" {
		t.Errorf("name: got %q", page.Results[1].Name)
	}
}

func TestFetchPublishersHTTPError(t *testing.T) {
	srv := serve(t, "/", http.StatusInternalServerError, "error", nil)
	defer srv.Close()

	c := upstream.New(srv.URL, nil)
	_, _, err := c.FetchPublishers(context.Background(), c.PublishersURL(10, 0))
	if err == nil {
		t.Fatal("expected error on non-200 response")
	}
}

func TestFetchPublishersPagination(t *testing.T) {
	pages := []string{
		`{"count":2,"next":"PAGE2","results":[{"uuid":"p1","name":"One"}]}`,
		`{"count":2,"next":"","results":[{"uuid":"p2","name":"Two"}]}`,
	}
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(pages[call])) //nolint:errcheck
		call++
	}))
	defer srv.Close()

	// Rewrite next URL placeholder to point at test server.
	pages[0] = strings.ReplaceAll(pages[0], "PAGE2", srv.URL+"/page2")

	c := upstream.New(srv.URL, nil)
	pageURL := c.PublishersURL(1, 0)
	var uuids []string
	for pageURL != "" {
		_, page, err := c.FetchPublishers(context.Background(), pageURL)
		if err != nil {
			t.Fatalf("page %d: %v", call, err)
		}
		for _, p := range page.Results {
			uuids = append(uuids, p.UUID)
		}
		pageURL = page.Next
	}
	if len(uuids) != 2 {
		t.Errorf("got %d uuids, want 2", len(uuids))
	}
}

// --- FetchPublisherItems ---

func TestFetchPublisherItems(t *testing.T) {
	const fixture = `{
		"count": 1,
		"next": "",
		"results": [
			{
				"ourn": "urn:orm:book:9781234567890",
				"name": "Learning Go",
				"publication_date": "2021-03-01",
				"archive_id": "9781234567890"
			}
		]
	}`
	srv := serve(t, "/api/v2/metadata/", http.StatusOK, fixture, nil)
	defer srv.Close()

	c := upstream.New(srv.URL, nil)
	raw, page, err := c.FetchPublisherItems(context.Background(), c.PublisherItemsURL("pub-1", 100, 0))
	if err != nil {
		t.Fatalf("FetchPublisherItems: %v", err)
	}

	if len(raw) == 0 {
		t.Error("expected non-empty raw bytes")
	}
	if len(page.Results) != 1 {
		t.Fatalf("results: got %d, want 1", len(page.Results))
	}
	item := page.Results[0]
	if item.OURN != "urn:orm:book:9781234567890" {
		t.Errorf("ourn: got %q", item.OURN)
	}
	if item.Name != "Learning Go" {
		t.Errorf("name: got %q", item.Name)
	}
	if item.PublicationDate != "2021-03-01" {
		t.Errorf("publication_date: got %q", item.PublicationDate)
	}
}

// --- GetCover ---

func TestGetCoverOK(t *testing.T) {
	imgBytes := []byte{0xFF, 0xD8, 0xFF} // JPEG magic bytes
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(imgBytes) //nolint:errcheck
	}))
	defer srv.Close()

	c := upstream.New(srv.URL, nil)
	body, ct, err := c.GetCover(context.Background(), "isbn:9781234", "large")
	if err != nil {
		t.Fatalf("GetCover: %v", err)
	}
	if ct != "image/jpeg" {
		t.Errorf("content-type: got %q, want image/jpeg", ct)
	}
	if len(body) != len(imgBytes) {
		t.Errorf("body length: got %d, want %d", len(body), len(imgBytes))
	}
}

func TestGetCoverNotFound(t *testing.T) {
	srv := serve(t, "/", http.StatusNotFound, "not found", nil)
	defer srv.Close()

	c := upstream.New(srv.URL, nil)
	_, _, err := c.GetCover(context.Background(), "isbn:9781234", "large")
	if !errors.Is(err, upstream.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetCoverServerError(t *testing.T) {
	srv := serve(t, "/", http.StatusBadGateway, "bad gateway", nil)
	defer srv.Close()

	c := upstream.New(srv.URL, nil)
	_, _, err := c.GetCover(context.Background(), "isbn:9781234", "large")
	if err == nil || errors.Is(err, upstream.ErrNotFound) {
		t.Errorf("expected non-nil, non-ErrNotFound error; got %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never responds — context should cancel first.
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	c := upstream.New(srv.URL, nil)
	_, _, err := c.FetchPublishers(ctx, c.PublishersURL(10, 0))
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
