package domain

// Pagination bounds. A list endpoint with no ceiling is a way for a client to
// ask for the entire table, so the size is clamped here rather than trusted.
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Page is a requested slice of a collection, expressed the way a client asks
// for it -- a page number and a size -- rather than as a raw offset.
type Page struct {
	Number int // 1-based
	Size   int
}

// NewPage clamps whatever arrived into something safe to run.
func NewPage(number, size int) Page {
	if number < 1 {
		number = 1
	}
	switch {
	case size < 1:
		size = DefaultPageSize
	case size > MaxPageSize:
		size = MaxPageSize
	}
	return Page{Number: number, Size: size}
}

func (p Page) Limit() int  { return p.Size }
func (p Page) Offset() int { return (p.Number - 1) * p.Size }

// Sort names a field to order by. It is a field name from the domain, not a
// column: the repository maps it onto a column from a fixed list, so no client
// string ever reaches the SQL.
type Sort struct {
	Field      string
	Descending bool
}

// PageResult pairs one page of items with the size of the whole collection.
// The total is what lets a caller say how many pages there are, and it travels
// with the items because computing it separately can disagree with them.
type PageResult[T any] struct {
	Items []T
	Total int
	Page  Page
}

// TotalPages is how many pages the collection covers at this page size.
func (r PageResult[T]) TotalPages() int {
	if r.Page.Size <= 0 || r.Total <= 0 {
		return 0
	}
	// Integer ceiling: a trailing partial page still counts as a page.
	return (r.Total + r.Page.Size - 1) / r.Page.Size
}

func (r PageResult[T]) HasNext() bool { return r.Page.Number < r.TotalPages() }
func (r PageResult[T]) HasPrev() bool { return r.Page.Number > 1 }
