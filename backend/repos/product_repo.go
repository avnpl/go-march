package repos

import (
	"context"
	"fmt"
	utilErrs "github.com/avnpl/go-march/utils"
	"strings"

	"github.com/avnpl/go-march/models"
	"github.com/jmoiron/sqlx"
)

type ProductRepo interface {
	Create(ctx context.Context, p *models.Product) error
	FetchByID(ctx context.Context, id int64) (models.Product, error)
	UpdateByID(ctx *context.Context, id int64, p *models.UpdateProductReq) (*models.Product, error)
}

type pgProductRepo struct {
	db *sqlx.DB
}

func NewPGProductRepo(db *sqlx.DB) ProductRepo {
	return &pgProductRepo{db: db}
}

func (r *pgProductRepo) Create(ctx context.Context, p *models.Product) error {
	const query = "INSERT INTO products (prod_name, price, stock) VALUES ($1, $2, $3) RETURNING prod_id, created_at"

	if err := r.db.QueryRowContext(ctx, query, p.Name, p.Price, p.Stock).
		Scan(&p.ProductID, &p.CreatedAt); err != nil {

		if strings.Contains(err.Error(), "unique") {
			return fmt.Errorf("repo.Create conflict: %w", err)
		}
		return fmt.Errorf("repo.Create: %w", err)
	}
	return nil
}

func (r pgProductRepo) FetchByID(ctx context.Context, id int64) (models.Product, error) {
	const query = "SELECT * FROM PRODUCTS WHERE PROD_ID = $1"

	var result models.Product
	err := r.db.GetContext(ctx, &result, query, id)
	if err != nil {
		return result, fmt.Errorf("repo.FetchByID: %w", err)
	}
	return result, nil
}

func (r pgProductRepo) UpdateByID(ctx *context.Context, id int64, p *models.UpdateProductReq) (*models.Product, error) {
	query := "UPDATE products SET "
	args := make(map[string]interface{})
	var fieldsToUpdate []string

	if p.Name != nil && *p.Name != "" {
		fieldsToUpdate = append(fieldsToUpdate, "prod_name = :prod_name")
		args["prod_name"] = p.Name
	}

	if p.Stock != nil && *p.Stock != 0 {
		fieldsToUpdate = append(fieldsToUpdate, "stock = :stock")
		args["stock"] = p.Stock
	}

	if p.Price != nil && *p.Price != 0 {
		fieldsToUpdate = append(fieldsToUpdate, "price = :price")
		args["price"] = p.Price
	}

	fieldsToUpdate = append(fieldsToUpdate, "updated_at = NOW()")
	query += strings.Join(fieldsToUpdate, ", ")
	query += " WHERE prod_id = :prod_id"
	args["prod_id"] = p.ProductID

	result, err := r.db.NamedExecContext(*ctx, query, args)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return nil, fmt.Errorf("repo.Update conflict: %w", err)
		}
		return nil, fmt.Errorf("repo.Update: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("repo.Update: could not get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("repo.Update: %w", utilErrs.ErrRecordNotFound)
	}

	updatedProduct, err := r.FetchByID(*ctx, id)

	return &updatedProduct, nil
}
