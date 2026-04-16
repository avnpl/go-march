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
	Amount    float64   `db:"amount"`
	CreatedAt time.Time `db:"order_time"`
}

type CreateProductReq struct {
	Name  string  `json:"name" validate:"required"`
	Price float64 `json:"price" validate:"gt=0"`
	Stock int     `json:"stock" validate:"min=0"`
}

type UpdateProductReq struct {
	ProductID string  `json:"prod_id" validate:"required"`
	Name      string  `json:"name,omitempty"`
	Price     float64 `json:"price,omitempty" validate:"omitempty,gt=0"`
	Stock     int     `json:"stock,omitempty" validate:"omitempty,min=0"`
}
