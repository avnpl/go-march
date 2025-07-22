package models

type Product struct {
	ProductID string  `db:"prod_id" json:"prod_id"`
	Name      string  `db:"prod_name" json:"prod_name"`
	Price     float64 `db:"price" json:"price"`
	Stock     int     `db:"stock" json:"stock"`
	CreatedAt string  `db:"created_at" json:"created_at"`
	UpdatedAt string  `db:"updated_at" json:"updated_at"`
}

type Orders struct {
	OrderID   string `db:"order_id"`
	Quantity  int    `db:"quantity"`
	ProductID string `db:"product_id"`
	Amount    string `db:"total_price"`
	CreatedAt string `db:"order_time"`
}

type CreateProductReq struct {
	Name  string  `json:"name" validate:"required"`
	Price float64 `json:"price" validate:"required"`
	Stock int     `json:"stock" validate:"required"`
}

type UpdateProductReq struct {
	ProductID *int64   `json:"prod_id" validate:"required"`
	Name      *string  `json:"name,omitempty"`
	Price     *float64 `json:"price,omitempty"`
	Stock     *int     `json:"stock,omitempty"`
}
