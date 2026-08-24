package productrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"go-commerce/domain"
	"go-commerce/infra/db/dbquery"
	"go-commerce/product"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ProductRepo is the Postgres implementation of product.Repository.
//
// It holds the pool, not the port: embedding product.Repository here would make
// every method it forgot to write compile anyway and panic on the first call,
// which is exactly the mistake the assertion below is meant to catch.
type ProductRepo struct {
	dbCon *sqlx.DB
}

// Compile-time proof that this adapter still satisfies the port. Without it, a
// method added to the interface would only fail at the composition root, far
// from the file that needs the new query.
var _ product.Repository = (*ProductRepo)(nil)

func NewProductRepo(dbCon *sqlx.DB) *ProductRepo {
	return &ProductRepo{dbCon: dbCon}
}

const productColumns = `id, title, price, img_url, description, created_at, updated_at`

// productSortFields maps what a client may ask to sort by onto columns. The
// client names a key from this map and never a column, so no request can reach
// a column that is not listed here.
var productSortFields = map[string]string{
	"createdAt": "created_at",
	"updatedAt": "updated_at",
	"price":     "price",
	"title":     "lower(title)",
}

const productDefaultOrder = "created_at DESC, id ASC"

func (r *ProductRepo) All(ctx context.Context, page domain.Page, filter *domain.ProductFilter) (*domain.PageResult[*domain.Product], error) {
	if filter == nil {
		filter = &domain.ProductFilter{}
	}

	conds := productConditions(filter)
	where := conds.SQL()
	args := conds.Args()

	query := fmt.Sprintf(`
		SELECT %s, COUNT(*) OVER () AS total_count
		FROM products
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`,
		productColumns,
		where,
		dbquery.OrderBy(filter.Sort.Field, filter.Sort.Descending, productSortFields, productDefaultOrder),
		len(args)+1, len(args)+2,
	)

	var rows []struct {
		domain.Product
		TotalCount int `db:"total_count"`
	}
	if err := r.dbCon.SelectContext(ctx, &rows, query, append(args, page.Limit(), page.Offset())...); err != nil {
		return nil, fmt.Errorf("listing products: %w", err)
	}

	items := make([]*domain.Product, 0, len(rows))
	total := 0
	for _, row := range rows {
		// row is a fresh copy each iteration, but the embedded product is copied
		// out explicitly so what escapes to the caller is a product and not a
		// pointer into a scan row that also carries the count.
		found := row.Product
		items = append(items, &found)
		total = row.TotalCount
	}

	// An empty page still has to report the size of the collection: without a
	// separate count, page 5 of a 3-page result would claim the collection is
	// empty rather than that the page is past the end.
	if len(rows) == 0 {
		var err error
		if total, err = dbquery.CountRows(ctx, r.dbCon, "products", where, args); err != nil {
			return nil, err
		}
	}

	return &domain.PageResult[*domain.Product]{Items: items, Total: total, Page: page}, nil
}

func productConditions(filter *domain.ProductFilter) *dbquery.Conditions {
	conds := &dbquery.Conditions{}

	if search := strings.TrimSpace(filter.Search); search != "" {
		// One bound value, matched against two columns.
		marker := conds.Placeholder("%" + strings.ToLower(search) + "%")
		conds.Where(fmt.Sprintf(
			"(lower(title) LIKE %s OR lower(coalesce(description, '')) LIKE %s)", marker, marker))
	}
	if filter.MinPrice != nil {
		conds.Where("price >= " + conds.Placeholder(*filter.MinPrice))
	}
	if filter.MaxPrice != nil {
		conds.Where("price <= " + conds.Placeholder(*filter.MaxPrice))
	}

	return conds
}

func (r *ProductRepo) Create(ctx context.Context, p *domain.Product) (*domain.Product, error) {
	query := fmt.Sprintf(`
		INSERT INTO products (title, price, img_url, description)
		VALUES ($1, $2, $3, $4)
		RETURNING %s`, productColumns)

	var created domain.Product
	if err := r.dbCon.GetContext(ctx, &created, query,
		p.Title, p.Price, p.ImgUrl, p.Description); err != nil {
		return nil, fmt.Errorf("creating product: %w", err)
	}
	return &created, nil
}

func (r *ProductRepo) ByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	query := fmt.Sprintf(`SELECT %s FROM products WHERE id = $1`, productColumns)

	var found domain.Product
	if err := r.dbCon.GetContext(ctx, &found, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrProductNotFound
		}
		return nil, fmt.Errorf("fetching product %s: %w", id, err)
	}
	return &found, nil
}

func (r *ProductRepo) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.dbCon.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting product %s: %w", id, err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking delete result for %s: %w", id, err)
	}
	if rows == 0 {
		return domain.ErrProductNotFound
	}
	return nil
}

// Update reads the row, lets the caller change what it cares about, and writes
// it back.
//
// The read and the write are one transaction with the row locked between them.
// Two concurrent PATCHes would otherwise both read the old row, and whichever
// wrote second would silently undo the other's change -- a client would get 200
// and see its edit disappear.
func (r *ProductRepo) Update(ctx context.Context, id uuid.UUID, apply func(*domain.Product)) (*domain.Product, error) {
	tx, err := r.dbCon.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("updating product %s: %w", id, err)
	}
	defer tx.Rollback()

	var current domain.Product
	lockRow := fmt.Sprintf(`SELECT %s FROM products WHERE id = $1 FOR UPDATE`, productColumns)
	if err := tx.GetContext(ctx, &current, lockRow, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrProductNotFound
		}
		return nil, fmt.Errorf("locking product %s: %w", id, err)
	}

	apply(&current)

	query := fmt.Sprintf(`
		UPDATE products
		SET title = $1, price = $2, img_url = $3, description = $4, updated_at = now()
		WHERE id = $5
		RETURNING %s`, productColumns)

	var updated domain.Product
	if err := tx.GetContext(ctx, &updated, query,
		current.Title, current.Price, current.ImgUrl, current.Description, id); err != nil {
		return nil, fmt.Errorf("updating product %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing product %s: %w", id, err)
	}
	return &updated, nil
}
