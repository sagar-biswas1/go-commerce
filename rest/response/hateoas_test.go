package response

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	"go-commerce/domain"
)

func TestPageClamping(t *testing.T) {
	cases := []struct {
		url              string
		wantNum, wantLen int
	}{
		{"/products", 1, domain.DefaultPageSize},
		{"/products?page=3&pageSize=50", 3, 50},
		{"/products?page=0&pageSize=-4", 1, domain.DefaultPageSize},
		{"/products?page=abc&pageSize=xyz", 1, domain.DefaultPageSize},
		// A client asking for the whole table gets the ceiling instead.
		{"/products?pageSize=100000", 1, domain.MaxPageSize},
	}

	for _, c := range cases {
		got := PageFromRequest(httptest.NewRequest("GET", c.url, nil))
		if got.Number != c.wantNum || got.Size != c.wantLen {
			t.Errorf("%s -> %+v, want {%d %d}", c.url, got, c.wantNum, c.wantLen)
		}
	}
}

func TestPageLinksPreserveFilters(t *testing.T) {
	r := httptest.NewRequest("GET", "/products?q=lamp&sort=-price&page=2&pageSize=10", nil)
	page := PageFromRequest(r)
	result := domain.PageResult[int]{Items: []int{1}, Total: 25, Page: page}

	if result.TotalPages() != 3 {
		t.Fatalf("TotalPages = %d, want 3", result.TotalPages())
	}

	links := PageLinks(r, result, page)
	for _, rel := range []string{"self", "first", "last", "prev", "next"} {
		if links[rel] == "" {
			t.Errorf("missing %q link", rel)
		}
	}
	// Paging must not drop the filter or the sort the client asked for.
	if want := "/products?page=3&pageSize=10&q=lamp&sort=-price"; links["next"] != want {
		t.Errorf("next = %q, want %q", links["next"], want)
	}
	if links["last"] != "/products?page=3&pageSize=10&q=lamp&sort=-price" {
		t.Errorf("last = %q", links["last"])
	}

	// The first page has no previous, the last no next.
	firstPage := domain.NewPage(1, 10)
	if got := PageLinks(r, domain.PageResult[int]{Total: 25, Page: firstPage}, firstPage); got["prev"] != "" {
		t.Errorf("first page offered a prev link: %q", got["prev"])
	}
}

func TestSortParsing(t *testing.T) {
	for url, want := range map[string]domain.Sort{
		"/p":             {},
		"/p?sort=price":  {Field: "price"},
		"/p?sort=-price": {Field: "price", Descending: true},
	} {
		if got := SortFromRequest(httptest.NewRequest("GET", url, nil)); got != want {
			t.Errorf("%s -> %+v, want %+v", url, got, want)
		}
	}
}

func TestCollectAddsSelfLinks(t *testing.T) {
	type item struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	items := []item{{1, "a"}, {2, "b"}}

	resources := Collect(items, func(i item) Links {
		return Links{"self": Path("/things", strconv.Itoa(i.ID))}
	})

	encoded, err := json.Marshal(resources)
	if err != nil {
		t.Fatal(err)
	}

	var decoded []map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 2 {
		t.Fatalf("got %d resources", len(decoded))
	}
	// The entity's own fields survive alongside the added links.
	if decoded[0]["name"] != "a" {
		t.Errorf("lost entity fields: %v", decoded[0])
	}
	links, ok := decoded[0]["links"].(map[string]any)
	if !ok || links["self"] != "/things/1" {
		t.Errorf("self link = %v", decoded[0]["links"])
	}
}

func TestEmptyCollectionEncodesAsArray(t *testing.T) {
	// A client iterating the result should not have to special-case null.
	encoded, _ := json.Marshal(Collect([]int{}, func(int) Links { return nil }))
	if string(encoded) != "[]" {
		t.Errorf("empty collection encoded as %s, want []", encoded)
	}
}
