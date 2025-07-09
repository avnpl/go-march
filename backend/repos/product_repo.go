package repos

import (
	"context"
	"fmt"

	"strings"

	"github.com/avnpl/go-march/models"
	"github.com/jmoiron/sqlx"
)

type ProductRepo interface {
	Create(ctx context.Context, p *models.Product) error
	FetchByID(ctx context.Context, id int64) (models.Product, error)
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
