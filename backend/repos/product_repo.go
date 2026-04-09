package repos

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/avnpl/go-march/models"
	"github.com/jmoiron/sqlx"
)

type ProductRepo interface {
	Create(ctx context.Context, p *models.Product) (models.Product, error)
	FetchByID(ctx context.Context, id string) (models.Product, error)
	FetchAll(ctx context.Context) ([]models.Product, error)
	UpdateByID(ctx context.Context, p *models.UpdateProductReq) (models.Product, error)
	DeleteByID(ctx context.Context, id int64) (models.Product, error)
}

type pgProductRepo struct {
	db *sqlx.DB
}

func NewPGProductRepo(db *sqlx.DB) ProductRepo {
	return pgProductRepo{db: db}
}

func (r pgProductRepo) Create(ctx context.Context, p *models.Product) (models.Product, error) {
	// TODO(#12): Uppercase SQL keywords — normalize to lowercase for consistency
	// e.g. "insert into products (...) values (...) returning *"
	const query = "INSERT INTO products (prod_id, prod_name, price, stock) VALUES ($1, $2, $3, $4) RETURNING *"

	var res models.Product
	if err := r.db.GetContext(ctx, &res, query, p.ProductID, p.Name, p.Price, p.Stock); err != nil {
		return models.Product{}, fmt.Errorf("product_repo.Create: %w", err)
	}
	return res, nil
}

func (r pgProductRepo) FetchByID(ctx context.Context, id string) (models.Product, error) {
	const query = "select * from products where prod_id = $1"

	var result models.Product
	err := r.db.GetContext(ctx, &result, query, id)
	if err != nil {
		return result, fmt.Errorf("product_repo.FetchByID: %w", err)
	}
	return result, nil
}

func (r pgProductRepo) FetchAll(ctx context.Context) ([]models.Product, error) {
	const query = "select * from products"

	var result []models.Product
	err := r.db.SelectContext(ctx, &result, query)
	if err != nil {
		return result, fmt.Errorf("product_repo.FetchAllProducts: %w", err)
	}
	return result, nil
}

func (r pgProductRepo) UpdateByID(ctx context.Context, p *models.UpdateProductReq) (models.Product, error) {
	// TODO(#12): Uppercase SQL keywords here and in the WHERE/RETURNING clause below
	// — normalize to lowercase like FetchByID/FetchAll/DeleteByID already are
	query := "UPDATE products SET "
	args := make(map[string]interface{})
	var fieldsToUpdate []string
	var res models.Product

	if p.Name != "" {
		fieldsToUpdate = append(fieldsToUpdate, "prod_name = :prod_name")
		args["prod_name"] = p.Name
	}

	if p.Stock != 0 {
		fieldsToUpdate = append(fieldsToUpdate, "stock = :stock")
		args["stock"] = p.Stock
	}

	if p.Price != 0.0 {
		fieldsToUpdate = append(fieldsToUpdate, "price = :price")
		args["price"] = p.Price
	}

	fieldsToUpdate = append(fieldsToUpdate, "updated_at = NOW()")
	query += strings.Join(fieldsToUpdate, ", ")
	query += " WHERE prod_id = :prod_id RETURNING *"
	args["prod_id"] = p.ProductID

	result, err := r.db.NamedQueryContext(ctx, query, args)
	if err != nil {
		return models.Product{}, fmt.Errorf("product_repo.Update: %w", err)
	}
	defer result.Close()

	if result.Next() {
		err := result.StructScan(&res)
		if err != nil {
			return models.Product{}, fmt.Errorf("product_repo.Update: %w", err)
		}
	} else {
		return models.Product{}, fmt.Errorf("product_repo.Update: %w", sql.ErrNoRows)
	}

	return res, nil
}

// TODO(id-migration): DeleteByID still takes int64 — must change to string
// to match the PR-XXXXXX ID format used by Create and FetchByID.
// Products created with string IDs cannot be deleted through this path.
func (r pgProductRepo) DeleteByID(ctx context.Context, id int64) (models.Product, error) {
	const query = "delete from products where prod_id = $1 returning *"

	var result models.Product
	err := r.db.GetContext(ctx, &result, query, id)
	if err != nil {
		return result, fmt.Errorf("product_repo.DeleteByID: %w", err)
	}
	return result, nil
}
