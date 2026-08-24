package product

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-commerce/domain"

	"github.com/google/uuid"
)

// fakeService satisfies the Service port declared in port.go. Nothing here
// imports the product aggregate: the handler is tested against the shape it
// asked for, which is what that port is for.
type fakeService struct {
	product *domain.Product
	err     error

	gotPatch   *domain.ProductPatch
	gotProduct *domain.Product
	gotFilter  *domain.ProductFilter
	gotPage    domain.Page
}

func (f *fakeService) List(_ context.Context, page domain.Page, filter *domain.ProductFilter) (*domain.PageResult[*domain.Product], error) {
	f.gotPage, f.gotFilter = page, filter
	if f.err != nil {
		return nil, f.err
	}
	return &domain.PageResult[*domain.Product]{Items: []*domain.Product{f.product}, Total: 1, Page: page}, nil
}

func (f *fakeService) Get(context.Context, uuid.UUID) (*domain.Product, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.product, nil
}

func (f *fakeService) Create(_ context.Context, p *domain.Product) (*domain.Product, error) {
	f.gotProduct = p
	if f.err != nil {
		return nil, f.err
	}
	return f.product, nil
}

func (f *fakeService) Update(_ context.Context, _ uuid.UUID, patch *domain.ProductPatch) (*domain.Product, error) {
	f.gotPatch = patch
	if f.err != nil {
		return nil, f.err
	}
	return f.product, nil
}

func (f *fakeService) Replace(_ context.Context, _ uuid.UUID, p *domain.Product) (*domain.Product, error) {
	f.gotProduct = p
	if f.err != nil {
		return nil, f.err
	}
	return f.product, nil
}

func (f *fakeService) Delete(context.Context, uuid.UUID) error { return f.err }

// newTestServer mounts the real routes on a real mux, so the paths and methods
// under test are the ones the server actually registers. The nil pipeline is the
// contract NewHandler documents: a module can be built without middleware.
func newTestServer(t *testing.T) (*http.ServeMux, *fakeService) {
	t.Helper()

	svc := &fakeService{product: &domain.Product{
		ID:    uuid.MustParse("11111111-2222-3333-4444-555555555555"),
		Title: "Desk lamp",
		Price: 42.5,
	}}

	mux := http.NewServeMux()
	NewHandler(svc, nil).RegisterRoutes(mux)
	return mux, svc
}

func do(t *testing.T, mux *http.ServeMux, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding %q: %v", rec.Body.String(), err)
	}
	return got
}

func TestGetProductRepliesWithAnEnvelopeAndLinks(t *testing.T) {
	mux, svc := newTestServer(t)

	rec := do(t, mux, http.MethodGet, "/products/"+svc.product.ID.String(), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}

	body := decode(t, rec)
	if _, ok := body["data"]; !ok {
		t.Error("no data member; every reply uses the same envelope")
	}

	links, ok := body["links"].(map[string]any)
	if !ok {
		t.Fatalf("links = %v, want the relations a client navigates by", body["links"])
	}
	for _, rel := range []string{"self", "update", "replace", "delete", "collection"} {
		if links[rel] == "" || links[rel] == nil {
			t.Errorf("missing %q relation; got %v", rel, links)
		}
	}
}

func TestCreateRepliesWith201AndLocation(t *testing.T) {
	mux, svc := newTestServer(t)

	rec := do(t, mux, http.MethodPost, "/products", `{"title":"Desk lamp","price":42.5}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Location"); got != "/products/"+svc.product.ID.String() {
		t.Errorf("Location = %q, want the new resource's own path", got)
	}
	if svc.gotProduct.Title != "Desk lamp" {
		t.Errorf("service saw title %q", svc.gotProduct.Title)
	}
}

// The request type has no id field, so a body cannot set one. This is the test
// that keeps it that way.
func TestCreateRefusesFieldsTheServerOwns(t *testing.T) {
	mux, _ := newTestServer(t)

	rec := do(t, mux, http.MethodPost, "/products",
		`{"title":"Desk lamp","price":42.5,"id":"11111111-2222-3333-4444-555555555555"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unknown field: %s", rec.Code, rec.Body)
	}
}

// A patch that mentions no price and one that sets price to 0 must not arrive
// identical, which is the only reason the fields are pointers.
func TestPatchDistinguishesOmittedFromZero(t *testing.T) {
	mux, svc := newTestServer(t)
	id := svc.product.ID.String()

	do(t, mux, http.MethodPatch, "/products/"+id, `{"title":"Renamed"}`)
	if svc.gotPatch.Price != nil {
		t.Errorf("price = %v, want nil for a field the body never mentioned", *svc.gotPatch.Price)
	}

	do(t, mux, http.MethodPatch, "/products/"+id, `{"price":0}`)
	if svc.gotPatch.Price == nil || *svc.gotPatch.Price != 0 {
		t.Errorf("price = %v, want an explicit 0 to reach the service", svc.gotPatch.Price)
	}
}

func TestListPassesPagingAndFiltersThrough(t *testing.T) {
	mux, svc := newTestServer(t)

	rec := do(t, mux, http.MethodGet, "/products?page=2&pageSize=5&q=lamp&minPrice=10&sort=-price", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}

	if svc.gotPage.Number != 2 || svc.gotPage.Size != 5 {
		t.Errorf("page = %+v, want page 2 of size 5", svc.gotPage)
	}
	if svc.gotFilter.Search != "lamp" {
		t.Errorf("search = %q", svc.gotFilter.Search)
	}
	if svc.gotFilter.MinPrice == nil || *svc.gotFilter.MinPrice != 10 {
		t.Errorf("minPrice = %v", svc.gotFilter.MinPrice)
	}
	if svc.gotFilter.Sort.Field != "price" || !svc.gotFilter.Sort.Descending {
		t.Errorf("sort = %+v, want a descending price sort", svc.gotFilter.Sort)
	}

	body := decode(t, rec)
	if _, ok := body["meta"]; !ok {
		t.Error("no meta member; a client cannot page without knowing the total")
	}

	// Items travel as pointers now, and they still have to come out of the
	// envelope as objects carrying their own links rather than as bare entities.
	data, ok := body["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("data = %v, want one item", body["data"])
	}
	item, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("item = %v, want an object", data[0])
	}
	if _, ok := item["links"]; !ok {
		t.Errorf("item has no links: %v", item)
	}
}

func TestNotFoundBecomes404(t *testing.T) {
	mux, svc := newTestServer(t)
	svc.err = domain.ErrProductNotFound

	rec := do(t, mux, http.MethodGet, "/products/"+uuid.New().String(), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body)
	}
	if _, ok := decode(t, rec)["error"]; !ok {
		t.Error("a failure reply must use the error envelope")
	}
}

func TestMalformedIDBecomes400(t *testing.T) {
	mux, _ := newTestServer(t)

	rec := do(t, mux, http.MethodGet, "/products/not-a-uuid", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body)
	}
}

func TestDeleteRepliesWith204AndNoBody(t *testing.T) {
	mux, svc := newTestServer(t)

	rec := do(t, mux, http.MethodDelete, "/products/"+svc.product.ID.String(), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want nothing", rec.Body.String())
	}
}
