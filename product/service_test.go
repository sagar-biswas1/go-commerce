package product

import (
	"context"
	"errors"
	"testing"

	"go-commerce/domain"

	"github.com/google/uuid"
)

// fakeRepo is the whole point of declaring Repository in this package: the
// service's rules are testable without a database, a migration or a container.
type fakeRepo struct {
	stored  domain.Product
	created domain.Product
	applied domain.Product
	err     error

	byIDCalls  int
	updateSeen bool
}

func (f *fakeRepo) All(context.Context, domain.Page, *domain.ProductFilter) (*domain.PageResult[*domain.Product], error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.PageResult[*domain.Product]{Items: []*domain.Product{&f.stored}, Total: 1}, nil
}

func (f *fakeRepo) Create(_ context.Context, p *domain.Product) (*domain.Product, error) {
	f.created = *p
	stored := *p
	stored.ID = uuid.New()
	return &stored, f.err
}

func (f *fakeRepo) ByID(context.Context, uuid.UUID) (*domain.Product, error) {
	f.byIDCalls++
	if f.err != nil {
		return nil, f.err
	}
	// A copy, as a real repository returns: a caller must not be able to edit
	// the stored row by writing through what it was handed.
	row := f.stored
	return &row, nil
}

func (f *fakeRepo) Delete(context.Context, uuid.UUID) error { return f.err }

func (f *fakeRepo) Update(_ context.Context, _ uuid.UUID, apply func(*domain.Product)) (*domain.Product, error) {
	f.updateSeen = true
	row := f.stored
	apply(&row)
	f.applied = row
	if f.err != nil {
		return nil, f.err
	}
	return &row, nil
}

func newFixture() (*service, *fakeRepo) {
	repo := &fakeRepo{stored: domain.Product{
		ID:          uuid.New(),
		Title:       "Desk lamp",
		Price:       42.50,
		Description: "warm light",
	}}
	return &service{repo: repo}, repo
}

func TestCreateNormalizesBeforeStoring(t *testing.T) {
	svc, repo := newFixture()

	if _, err := svc.Create(context.Background(), &domain.Product{Title: "  Desk lamp  ", Price: 10}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if repo.created.Title != "Desk lamp" {
		t.Errorf("stored title = %q, want the trimmed form", repo.created.Title)
	}
}

func TestCreateRejectsInvalidBeforeTouchingStorage(t *testing.T) {
	svc, repo := newFixture()

	_, err := svc.Create(context.Background(), &domain.Product{Title: "", Price: -1})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("err = %v, want an ErrInvalid", err)
	}

	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %T, want a *domain.ValidationError naming the fields", err)
	}
	// Every broken rule at once, so a form marks both inputs in one round trip.
	for _, field := range []string{"title", "price"} {
		if _, ok := invalid.Fields[field]; !ok {
			t.Errorf("no message for %q; got %v", field, invalid.Fields)
		}
	}
	if repo.created.Title != "" {
		t.Error("an invalid product reached the repository")
	}
}

// A patch that asks for nothing and no patch at all are the same request, and
// both are a client mistake worth naming rather than a successful no-op.
func TestUpdateRejectsAPatchThatAsksForNothing(t *testing.T) {
	for name, patch := range map[string]*domain.ProductPatch{
		"empty": {},
		"nil":   nil,
	} {
		t.Run(name, func(t *testing.T) {
			svc, repo := newFixture()

			_, err := svc.Update(context.Background(), uuid.New(), patch)
			if !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("err = %v, want an ErrInvalid", err)
			}
			if repo.updateSeen {
				t.Error("a patch asking for nothing still took a write lock")
			}
		})
	}
}

// Now that entities cross layers as pointers, the service must not write through
// the one it was handed: normalizing in place would edit the request struct the
// transport still holds, and a rejected create would hand back a body that no
// longer matches what arrived.
func TestCreateLeavesTheCallersProductAlone(t *testing.T) {
	svc, _ := newFixture()
	arrived := &domain.Product{Title: "  Desk lamp  ", Price: 10}

	if _, err := svc.Create(context.Background(), arrived); err != nil {
		t.Fatalf("create: %v", err)
	}
	if arrived.Title != "  Desk lamp  " {
		t.Errorf("caller's product was edited: title = %q", arrived.Title)
	}
}

// The same rule on the read side: the preview merge is a dry run, so it must not
// reach the row the locked write is based on.
func TestUpdatePreviewDoesNotLeakIntoTheWrite(t *testing.T) {
	svc, repo := newFixture()
	title := "Renamed"

	if _, err := svc.Update(context.Background(), repo.stored.ID, &domain.ProductPatch{Title: &title}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if repo.stored.Title != "Desk lamp" {
		t.Errorf("stored row = %q, want the dry run to have left it untouched", repo.stored.Title)
	}
}

// A patch is judged by the product it produces, not by the fields it mentions:
// clearing a title is only invalid once merged with the row that had one.
func TestUpdateValidatesTheMergedResult(t *testing.T) {
	svc, repo := newFixture()

	blank := ""
	_, err := svc.Update(context.Background(), repo.stored.ID, &domain.ProductPatch{Title: &blank})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("err = %v, want the merged product to fail validation", err)
	}
	if repo.updateSeen {
		t.Error("a patch that cannot produce a valid product still took a write lock")
	}
}

// The merge runs inside the repository's locked read, so what gets written is
// based on the row as it is at write time and not on the earlier preview read.
func TestUpdateAppliesInsideTheLock(t *testing.T) {
	svc, repo := newFixture()
	price := 99.99

	updated, err := svc.Update(context.Background(), repo.stored.ID, &domain.ProductPatch{Price: &price})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !repo.updateSeen {
		t.Fatal("the change never reached the locked write")
	}
	if updated.Price != price {
		t.Errorf("price = %v, want %v", updated.Price, price)
	}
	if repo.applied.Title != "Desk lamp" {
		t.Errorf("title = %q, want the untouched field left alone", repo.applied.Title)
	}
}

// PUT and PATCH differ in what an omitted field means. This is the difference.
func TestReplaceClearsOmittedFields(t *testing.T) {
	svc, repo := newFixture()

	_, err := svc.Replace(context.Background(), repo.stored.ID, &domain.Product{Title: "Floor lamp", Price: 60})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if repo.applied.Description != "" {
		t.Errorf("description = %q, want a PUT to clear what it did not state", repo.applied.Description)
	}
}

func TestListRejectsAnImpossibleFilter(t *testing.T) {
	svc, _ := newFixture()
	min, max := 100.0, 10.0

	_, err := svc.List(context.Background(), domain.NewPage(1, 20), &domain.ProductFilter{MinPrice: &min, MaxPrice: &max})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("err = %v, want a filter that can never match to be refused", err)
	}
}
