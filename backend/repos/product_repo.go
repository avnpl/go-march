package repos

import (
	"context"
	"fmt"

	"github.com/avnpl/go-march/models"
	"github.com/jmoiron/sqlx"
	"strings"
)

// ProductRepo defines DB actions for Products.
type ProductRepo interface {
	Create(ctx context.Context, p *models.Product) error
}

// postgresProductRepo is the PostgreSQL implementation.
type postgresProductRepo struct {
	db *sqlx.DB
}

// NewPostgresProductRepo wires the repo with a DB connection.
func NewPostgresProductRepo(db *sqlx.DB) ProductRepo {
	return &postgresProductRepo{db: db}
}

// Create inserts a product and scans back prod_id and created_at.
func (r *postgresProductRepo) Create(ctx context.Context, p *models.Product) error {
	const query = "INSERT INTO products (prod_name, price, stock) VALUES ($1, $2, $3) RETURNING prod_id, created_at"

	// QueryRowContext returns a single row; Scan writes into p.
	if err := r.db.QueryRowContext(ctx, query, p.Name, p.Price, p.Stock).
		Scan(&p.ProductID, &p.CreatedAt); err != nil {

		// Basic unique-violation check if name must be unique:
		if strings.Contains(err.Error(), "unique") {
			return fmt.Errorf("repo.Create conflict: %w", err)
		}
		return fmt.Errorf("repo.Create: %w", err)
	}
	return nil
}
