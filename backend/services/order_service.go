package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/repos"
	"github.com/avnpl/go-march/utils"
	"github.com/avnpl/go-march/utils/customErrors"
	"github.com/avnpl/go-march/utils/log"
	"go.uber.org/zap"
)

type OrderService interface {
	Create(ctx context.Context, req models.CreateOrderReq) (models.Order, error)
	FetchByID(ctx context.Context, id string) (models.Order, error)
	FetchAll(ctx context.Context, limit int, offset int) ([]models.Order, error)
	Delete()
}

type orderService struct {
	orderRepo   repos.OrderRepo
	productRepo repos.ProductRepo
	log         *zap.Logger
}

func NewOrderService(orderRepo repos.OrderRepo, productRepo repos.ProductRepo, l *zap.Logger) OrderService {
	return &orderService{orderRepo: orderRepo, productRepo: productRepo, log: l}
}

func (os *orderService) Create(ctx context.Context, req models.CreateOrderReq) (models.Order, error) {

	order := models.Order{
		OrderID:         utils.GenerateID("OR"),
		ProductID:       req.ProductID,
		Quantity:        req.Quantity,
		Amount:          req.Amount,
		ShippingAddress: req.ShippingAddress,
		Notes:           req.Notes,
		CardNumber:      req.CardNumber,
		CreatedAt:       time.Now(),
		Status:          "success",
	}

	txn, err := os.productRepo.BeginTransaction()
	defer txn.Rollback()

	product, err := os.productRepo.FetchByID(txn, ctx, order.ProductID)
	if err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: %w", err)
	}

	if product.Stock < order.Quantity {
		return models.Order{}, customErrors.OutOfStock
	}

	if order.Amount != product.Price*float64(order.Quantity) {
		return models.Order{}, customErrors.IncorrectAmount
	}

	cardNumEnd := req.CardNumber[len(req.CardNumber)-4:]
	if cardNumEnd == "6969" {
		return models.Order{}, customErrors.FailedTransaction
	}

	err = os.productRepo.UpdateProductStock(txn, ctx, product.ProductID, product.Stock-order.Quantity)
	if err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: %w", err)
	}

	res, err := os.orderRepo.Create(txn, ctx, order)
	if err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: %w", err)
	}

	if err := txn.Commit(); err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: commit failed: %w", err)
	}

	log.Info(ctx, os.log, "created order", zap.String("order_id", res.OrderID))
	return res, nil
}

func (os *orderService) FetchByID(ctx context.Context, id string) (models.Order, error) {
	order, err := os.orderRepo.FetchByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Order{}, customErrors.RecordNotFound
		}

		return models.Order{}, fmt.Errorf("order_service.FetchByID failed: %w", err)
	}
	return order, nil
}

func (os *orderService) FetchAll(ctx context.Context, limit int, offset int) ([]models.Order, error) {
	if limit == 0 {
		limit = utils.GetEnvVarInteger("FETCH_ALL_ORDERS_DEFAULT_LIMIT", 10, os.log)
	}

	res, err := os.orderRepo.FetchAll(ctx, limit, offset)
	if err != nil {
		log.Error(ctx, os.log, "failed to fetch orders", zap.Error(err))
		return res, fmt.Errorf("order_service.FetchAll: %w", err)
	}
	return res, nil
}

func (os *orderService) Delete() {
	panic("unimplemented")
}
