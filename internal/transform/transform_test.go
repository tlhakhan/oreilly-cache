package transform_test

import (
	"encoding/json"
	"os"
	"testing"

	"oreilly-cache/internal/transform"
	"oreilly-cache/internal/upstream"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// --- OnePublisher ---

func TestOnePublisher(t *testing.T) {
	cases := []struct {
		name  string
		input upstream.Publisher
		want  transform.Publisher
	}{
		{
			name:  "full fields",
			input: upstream.Publisher{UUID: "u1", Name: "O'Reilly", Slug: json.RawMessage(`"oreilly-media"`)},
			want:  transform.Publisher{UUID: "u1", Name: "O'Reilly", Slug: "oreilly-media"},
		},
		{
			name:  "null slug",
			input: upstream.Publisher{UUID: "u2", Name: "Pub", Slug: json.RawMessage(`null`)},
			want:  transform.Publisher{UUID: "u2", Name: "Pub", Slug: ""},
		},
		{
			name:  "missing slug",
			input: upstream.Publisher{UUID: "u3", Name: "Pub"},
			want:  transform.Publisher{UUID: "u3", Name: "Pub", Slug: ""},
		},
		{
			name:  "empty name",
			input: upstream.Publisher{UUID: "u4", Name: "", Slug: json.RawMessage(`"slug"`)},
			want:  transform.Publisher{UUID: "u4", Name: "", Slug: "slug"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := transform.OnePublisher(tc.input)
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestOnePublisherFromFixture loads publisher.json and verifies the transform
// only keeps uuid, name, slug and drops extra upstream fields.
func TestOnePublisherFromFixture(t *testing.T) {
	var p upstream.Publisher
	if err := json.Unmarshal(readFixture(t, "publisher.json"), &p); err != nil {
		t.Fatal(err)
	}

	got := transform.OnePublisher(p)

	if got.UUID != "aa1e312d-b9f6-46fa-9a82-e7e4b41d5c72" {
		t.Errorf("uuid: %q", got.UUID)
	}
	if got.Name != "O'Reilly Media, Inc." {
		t.Errorf("name: %q", got.Name)
	}
	if got.Slug != "oreilly-media" {
		t.Errorf("slug: %q", got.Slug)
	}

	// Verify extra upstream fields are not present in the marshaled output.
	b, _ := json.Marshal(got)
	var m map[string]any
	json.Unmarshal(b, &m) //nolint:errcheck
	for _, unwanted := range []string{"url", "logo"} {
		if _, ok := m[unwanted]; ok {
			t.Errorf("transformed output contains unwanted field %q", unwanted)
		}
	}
}

// --- Publishers (index) ---

func TestPublishersFromFixture(t *testing.T) {
	var page upstream.PublisherPage
	if err := json.Unmarshal(readFixture(t, "publisher_page.json"), &page); err != nil {
		t.Fatal(err)
	}

	got := transform.Publishers(page.Results)

	if len(got.Publishers) != 2 {
		t.Fatalf("publishers count: got %d, want 2", len(got.Publishers))
	}

	cases := []struct {
		uuid string
		name string
		slug string
	}{
		{"aa1e312d-b9f6-46fa-9a82-e7e4b41d5c72", "O'Reilly Media, Inc.", "oreilly-media"},
		{"bb2f423e-c0a7-57gb-ab93-f8f5c52e6d83", "No Starch Press", "no-starch-press"},
	}
	for i, tc := range cases {
		p := got.Publishers[i]
		if p.UUID != tc.uuid {
			t.Errorf("[%d] uuid: got %q, want %q", i, p.UUID, tc.uuid)
		}
		if p.Name != tc.name {
			t.Errorf("[%d] name: got %q, want %q", i, p.Name, tc.name)
		}
		if p.Slug != tc.slug {
			t.Errorf("[%d] slug: got %q, want %q", i, p.Slug, tc.slug)
		}
	}
}

func TestPublishersEmpty(t *testing.T) {
	got := transform.Publishers(nil)
	if len(got.Publishers) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

// --- OneItem ---

func TestOneItem(t *testing.T) {
	cases := []struct {
		name  string
		input upstream.Item
		check func(t *testing.T, got transform.Item)
	}{
		{
			name: "full fields",
			input: upstream.Item{
				OURN:            "urn:orm:book:123",
				Name:            "My Book",
				PublicationDate: "2024-01-01",
				Authors:         json.RawMessage(`[{"name":"Alice","uuid":"a1"},{"name":"Bob","uuid":"a2"}]`),
				Subjects:        json.RawMessage(`[{"name":"Go","uuid":"s1"}]`),
				PublisherUUID:   "pub-1",
			},
			check: func(t *testing.T, got transform.Item) {
				if got.OURN != "urn:orm:book:123" {
					t.Errorf("ourn: %q", got.OURN)
				}
				if got.Name != "My Book" {
					t.Errorf("name: %q", got.Name)
				}
				if len(got.Authors) != 2 || got.Authors[0] != "Alice" || got.Authors[1] != "Bob" {
					t.Errorf("authors: %v", got.Authors)
				}
				if len(got.Subjects) != 1 || got.Subjects[0] != "Go" {
					t.Errorf("subjects: %v", got.Subjects)
				}
				if got.PublisherUUID != "pub-1" {
					t.Errorf("publisher_uuid: %q", got.PublisherUUID)
				}
			},
		},
		{
			name:  "null authors and subjects",
			input: upstream.Item{OURN: "x", Authors: json.RawMessage(`null`), Subjects: json.RawMessage(`null`)},
			check: func(t *testing.T, got transform.Item) {
				if got.Authors != nil {
					t.Errorf("expected nil authors, got %v", got.Authors)
				}
				if got.Subjects != nil {
					t.Errorf("expected nil subjects, got %v", got.Subjects)
				}
			},
		},
		{
			name:  "missing authors and subjects",
			input: upstream.Item{OURN: "y"},
			check: func(t *testing.T, got transform.Item) {
				if got.Authors != nil {
					t.Errorf("expected nil authors, got %v", got.Authors)
				}
			},
		},
		{
			name:  "malformed authors JSON",
			input: upstream.Item{OURN: "z", Authors: json.RawMessage(`not-json`)},
			check: func(t *testing.T, got transform.Item) {
				if got.Authors != nil {
					t.Errorf("expected nil on parse error, got %v", got.Authors)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, transform.OneItem(tc.input))
		})
	}
}

// TestOneItemFromFixture loads item.json and checks the full transform.
func TestOneItemFromFixture(t *testing.T) {
	var item upstream.Item
	if err := json.Unmarshal(readFixture(t, "item.json"), &item); err != nil {
		t.Fatal(err)
	}

	got := transform.OneItem(item)

	if got.OURN != "urn:orm:book:9781098128944" {
		t.Errorf("ourn: %q", got.OURN)
	}
	if got.Name != "Learning Go, 2nd Edition" {
		t.Errorf("name: %q", got.Name)
	}
	if got.PublicationDate != "2023-03-01" {
		t.Errorf("publication_date: %q", got.PublicationDate)
	}
	if len(got.Authors) != 1 || got.Authors[0] != "Jon Bodner" {
		t.Errorf("authors: %v", got.Authors)
	}
	if len(got.Subjects) != 2 {
		t.Errorf("subjects count: %d", len(got.Subjects))
	}

	// Extra upstream fields must not appear in marshaled output.
	b, _ := json.Marshal(got)
	var m map[string]any
	json.Unmarshal(b, &m) //nolint:errcheck
	for _, unwanted := range []string{"isbn", "description", "cover_image_url"} {
		if _, ok := m[unwanted]; ok {
			t.Errorf("transformed output contains unwanted field %q", unwanted)
		}
	}
}

// --- Items (list) ---

func TestItemsFromFixture(t *testing.T) {
	var page upstream.ItemPage
	if err := json.Unmarshal(readFixture(t, "item_page.json"), &page); err != nil {
		t.Fatal(err)
	}

	got := transform.Items(page.Results)

	if len(got.Items) != 2 {
		t.Fatalf("items count: got %d, want 2", len(got.Items))
	}

	cases := []struct {
		ourn     string
		title    string
		authors  []string
		subjects []string
	}{
		{
			"urn:orm:book:9781098128944",
			"Learning Go, 2nd Edition",
			[]string{"Jon Bodner"},
			[]string{"Go"},
		},
		{
			"urn:orm:book:9781718502550",
			"The Linux Command Line, 2nd Edition",
			[]string{"William E. Shotts"},
			[]string{"Linux", "Shell"},
		},
	}

	for i, tc := range cases {
		item := got.Items[i]
		if item.OURN != tc.ourn {
			t.Errorf("[%d] ourn: got %q, want %q", i, item.OURN, tc.ourn)
		}
		if item.Name != tc.title {
			t.Errorf("[%d] name: got %q, want %q", i, item.Name, tc.title)
		}
		for j, want := range tc.authors {
			if j >= len(item.Authors) || item.Authors[j] != want {
				t.Errorf("[%d] author[%d]: got %v, want %q", i, j, item.Authors, want)
			}
		}
		for j, want := range tc.subjects {
			if j >= len(item.Subjects) || item.Subjects[j] != want {
				t.Errorf("[%d] subject[%d]: got %v, want %q", i, j, item.Subjects, want)
			}
		}
	}
}

func TestItemsEmpty(t *testing.T) {
	got := transform.Items(nil)
	if len(got.Items) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}
