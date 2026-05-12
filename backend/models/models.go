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

type Order struct {
	OrderID         string       `db:"order_id" json:"order_id"`
	ProductID       string       `db:"product_id" json:"product_id"`
	Quantity        int          `db:"quantity" json:"quantity"`
	Amount          float64      `db:"amount" json:"amount"`
	CreatedAt       time.Time    `db:"created_at" json:"created_at"`
	Status          string       `db:"status" json:"status"`
	ShippingAddress string       `db:"shipping_address" json:"shipping_address"`
	CardNumber      string       `db:"card_number" json:"card_number"`
	Notes           string       `db:"notes" json:"notes"`
	ExpiresAt       sql.NullTime `db:"ttl_expires_at" json:"-"`
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

type CreateOrderReq struct {
	ProductID       string  `json:"prod_id" validate:"required"`
	Quantity        int     `json:"quantity" validate:"required,gt=0"`
	Amount          float64 `json:"amount" validate:"required,gt=0"`
	ShippingAddress string  `json:"shippingAddress" validate:"required"`
	Notes           string  `json:"notes,omitempty"`
	CardNumber      string  `json:"card_num" validate:"required,numeric,len=16"`
}
