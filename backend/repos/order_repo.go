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
	Create(txn *sqlx.Tx, ctx context.Context, order models.Order) (models.Order, error)
	FetchByID(ctx context.Context, id string) (models.Order, error)
	FetchAll(ctx context.Context, limit int, offset int) ([]models.Order, error)
	Delete()
}

type orderRepo struct {
	db     *sqlx.DB
	logger *zap.Logger
}

func NewOrderRepo(db *sqlx.DB, logger *zap.Logger) OrderRepo {
	return orderRepo{db: db, logger: logger}
}

func (or orderRepo) Create(txn *sqlx.Tx, ctx context.Context, order models.Order) (models.Order, error) {
	const insertOrderQuery = "insert into orders (order_id, product_id, quantity, amount, created_at, status, shipping_address, card_number, notes) values ($1, $2, $3, $4, $5, $6, $7, $8, $9) returning *"

	var res models.Order
	if err := txn.GetContext(ctx, &res, insertOrderQuery, order.OrderID, order.ProductID, order.Quantity, order.Amount, order.CreatedAt, order.Status, order.ShippingAddress, order.CardNumber, order.Notes); err != nil {
		log.Error(ctx, or.logger, "failed to create order", zap.Error(err))
		return models.Order{}, fmt.Errorf("order_repo.Create: %w", err)
	}
	return res, nil
}

func (or orderRepo) FetchByID(ctx context.Context, id string) (models.Order, error) {
	const query = "select * from orders where order_id = $1"

	var res models.Order
	if err := or.db.GetContext(ctx, &res, query, id); err != nil {
		log.Error(ctx, or.logger, "failed to fetch order: %w", zap.Error(err))
		return models.Order{}, err
	}
	return res, nil
}

func (or orderRepo) FetchAll(ctx context.Context, limit int, offset int) ([]models.Order, error) {
	query := "select * from orders order by created_at desc limit $1"

	args := []interface{}{limit}

	if offset != 0 {
		query += " offset $2"
		args = append(args, offset)
	}

	log.Debug(ctx, or.logger, "fetching all orders", zap.Int("limit", limit), zap.Int("offset", offset))

	var result []models.Order
	err := or.db.SelectContext(ctx, &result, query, args...)
	if err != nil {
		log.Error(ctx, or.logger, "failed to fetch all orders", zap.Error(err))
		return result, fmt.Errorf("order_repo.FetchAllOrders: %w", err)
	}
	return result, nil
}

func (or orderRepo) Delete() {
	panic("unimplemented")
}
