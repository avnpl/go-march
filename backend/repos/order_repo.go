package repos

import (
	"context"
	"fmt"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/utils/log"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

type OrderRepo interface {
	Create(ctx context.Context, order models.Order) (models.Order, error)
	Fetch()
	FetchAll()
	Delete()
}

type pgOrderRepo struct {
	db     *sqlx.DB
	logger *zap.Logger
}

func NewPGOrderRepo(db *sqlx.DB, logger *zap.Logger) OrderRepo {
	return pgOrderRepo{db: db, logger: logger}
}

func (p pgOrderRepo) Create(ctx context.Context, order models.Order) (models.Order, error) {
	const query = "insert into orders (order_id, product_id, quantity, amount, created_at, status, shipping_address, card_number, notes) values ($1, $2, $3, $4, $5, $6, $7, $8, $9) returning *"

	var res models.Order
	if err := p.db.GetContext(ctx, &res, query, order.OrderID, order.ProductID, order.Quantity, order.Amount, order.CreatedAt, order.Status, order.ShippingAddress, order.CardNumber, order.Notes); err != nil {
		log.Error(ctx, p.logger, "failed to create order", zap.Error(err))
		return models.Order{}, fmt.Errorf("order_repo.Create: %w", err)
	}
	return res, nil
}

func (p pgOrderRepo) Delete() {
	panic("unimplemented")
}

func (p pgOrderRepo) Fetch() {
	panic("unimplemented")
}

func (p pgOrderRepo) FetchAll() {
	panic("unimplemented")
}
