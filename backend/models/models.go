package models

type Product struct {
	ProductID string `db:"prod_id"`
	Name      string `db:"prod_name"`
	Price     int    `db:"price"`
	Stock     int    `db:"stock"`
	CreatedAt string `db:"created_at"`
}

type Orders struct {
	OrderID   string `db:"order_id"`
	Quantity  int    `db:"quantity"`
	ProductID string `db:"product_id"`
	Amount    string `db:"total_price"`
	CreatedAt string `db:"order_time"`
}

type CreateProductReq struct {
	Name  string `json:"name" validate:"required"`
	Price int    `json:"price" validate:"required"`
	Stock int    `json:"stock" validate:"required"`
}
