package models

import (
	"database/sql"
	"time"
)

type Product struct {
	ProductID  string       `db:"prod_id" json:"prod_id"`
	Name       string       `db:"prod_name" json:"prod_name"`
	Price      float64      `db:"price" json:"price"`
	Stock      int          `db:"stock" json:"stock"`
	CreatedAt  time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time    `db:"updated_at" json:"updated_at"`
	TTLExpires sql.NullTime `db:"ttl_expires_at" json:"-"`
}

type Orders struct {
	OrderID   string    `db:"order_id"`
	Quantity  int       `db:"quantity"`
	ProductID string    `db:"product_id"`
	// TODO(model): ROADMAP says rename field to TotalPrice to match db column name.
	// Type was correctly changed from string to float64, but field name is still Amount.
	Amount    float64   `db:"total_price"`
	CreatedAt time.Time `db:"order_time"`
}

// TODO(#6-validation): validate:"required" on Price/Stock rejects zero-values.
// Price=0.0 and Stock=0 both fail validation because the validator treats Go
// zero-values as "empty". Per the ROADMAP spec: Price must be > 0 (use "gt=0"),
// Stock must be >= 0 (use "min=0") — and "required" only makes sense for Name (string).
type CreateProductReq struct {
	Name  string  `json:"name" validate:"required"`
	Price float64 `json:"price" validate:"required"`
	Stock int     `json:"stock" validate:"required"`
}

type UpdateProductReq struct {
	// TODO(id-migration): ProductID is still *int64 — must change to *string
	// to match PR-XXXXXX format. This breaks UpdateProduct for any product
	// created with string IDs. Also update the repo query (UpdateByID) and
	// GraphQL UpdateProductInput type accordingly.
	ProductID *int64   `json:"prod_id" validate:"required"`
	Name      *string  `json:"name,omitempty"`
	Price     *float64 `json:"price,omitempty"`
	Stock     *int     `json:"stock,omitempty"`
}
