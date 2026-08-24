package response

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"go-commerce/domain"
)

// Query parameter names, in one place so the handlers and the links they emit
// cannot disagree about what they are called.
const (
	QueryPage     = "page"
	QueryPageSize = "pageSize"
	QuerySort     = "sort"
)

// PageFromRequest reads ?page and ?pageSize.
//
// A value that will not parse is treated as absent rather than as an error: a
// client that sends page=abc gets the first page, which is more useful than a
// rejected request and cannot be mistaken for real data.
func PageFromRequest(r *http.Request) domain.Page {
	query := r.URL.Query()
	return domain.NewPage(
		intOrZero(query.Get(QueryPage)),
		intOrZero(query.Get(QueryPageSize)),
	)
}

// SortFromRequest reads ?sort, where a leading "-" means descending:
// ?sort=price ascends, ?sort=-createdAt descends.
//
// The field is passed through as written. It is the repository that maps it onto
// a column from a fixed list, so an unknown -- or hostile -- field name yields
// the default order rather than reaching the SQL.
func SortFromRequest(r *http.Request) domain.Sort {
	raw := strings.TrimSpace(r.URL.Query().Get(QuerySort))
	if raw == "" {
		return domain.Sort{}
	}

	if descending := strings.HasPrefix(raw, "-"); descending {
		return domain.Sort{Field: strings.TrimPrefix(raw, "-"), Descending: true}
	}
	return domain.Sort{Field: raw}
}

// FloatParam reads an optional numeric query parameter, returning nil when it is
// absent or unusable -- which the domain filters read as "do not filter on this".
func FloatParam(r *http.Request, name string) *float64 {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &value
}

func intOrZero(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}

// PageLinks builds the navigation set for a collection: self, first, last, and
// prev/next where they exist.
//
// Every other query parameter the client sent is preserved, so paging through a
// filtered, sorted listing keeps the filter and the sort. A client following
// these links never has to know how the URL is assembled -- which is the point
// of returning them.
func PageLinks(r *http.Request, result interface {
	TotalPages() int
	HasNext() bool
	HasPrev() bool
}, page domain.Page) Links {
	links := Links{
		"self": pageURL(r, page.Number, page.Size),
	}

	totalPages := result.TotalPages()
	if totalPages == 0 {
		return links
	}

	links["first"] = pageURL(r, 1, page.Size)
	links["last"] = pageURL(r, totalPages, page.Size)

	if result.HasPrev() {
		links["prev"] = pageURL(r, page.Number-1, page.Size)
	}
	if result.HasNext() {
		links["next"] = pageURL(r, page.Number+1, page.Size)
	}

	return links
}

// PageMeta is the counts half of a paged reply.
func PageMeta(result interface{ TotalPages() int }, page domain.Page, total int) Meta {
	return Meta{
		Page:       page.Number,
		PageSize:   page.Size,
		Total:      total,
		TotalPages: result.TotalPages(),
	}
}

// pageURL rewrites the current URL's page parameters, keeping the rest.
func pageURL(r *http.Request, number, size int) string {
	query := cloneQuery(r.URL.Query())
	query.Set(QueryPage, strconv.Itoa(number))
	query.Set(QueryPageSize, strconv.Itoa(size))

	return r.URL.Path + "?" + query.Encode()
}

func cloneQuery(original url.Values) url.Values {
	clone := make(url.Values, len(original))
	for key, values := range original {
		clone[key] = append([]string(nil), values...)
	}
	return clone
}

// WithLinks returns v as a JSON object with a "links" member added, so each item
// in a collection can carry its own self link.
//
// It re-marshals through a map rather than embedding a links field in every
// entity, which would put a transport concern into the domain types. The cost is
// one extra encode per item; the benefit is that domain.Product knows nothing
// about URLs.
func WithLinks(v any, links Links) any {
	if len(links) == 0 {
		return v
	}

	encoded, err := json.Marshal(v)
	if err != nil {
		return v
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		// Not a JSON object -- nothing to add a member to.
		return v
	}

	encodedLinks, err := json.Marshal(links)
	if err != nil {
		return v
	}
	object["links"] = encodedLinks

	return object
}

// Collect maps a page of entities into link-carrying resources.
//
// The slice is always non-nil, so an empty page encodes as [] rather than null:
// a client iterating the result should not have to special-case "no items".
func Collect[T any](items []T, selfLink func(T) Links) []any {
	resources := make([]any, 0, len(items))
	for _, item := range items {
		resources = append(resources, WithLinks(item, selfLink(item)))
	}
	return resources
}

// Path joins a base path with escaped segments: Path("/users", id, "password")
// gives "/users/<id>/password".
//
// It takes segments rather than a format string on purpose. A format-based
// version has to stringify each argument before escaping it, which leaves any
// verb other than %s receiving a string and rendering as "%!d(string=1)" -- a
// broken link that nothing complains about. There are no verbs here to get
// wrong.
//
// base is a constant from the calling package and is written through unchanged;
// the segments are values, so each is escaped.
func Path(base string, segments ...string) string {
	var built strings.Builder
	built.WriteString(strings.TrimSuffix(base, "/"))

	for _, segment := range segments {
		built.WriteByte('/')
		built.WriteString(url.PathEscape(segment))
	}
	return built.String()
}
