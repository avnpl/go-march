package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
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

func (s *orderService) Create(ctx context.Context, req models.CreateOrderReq) (models.Order, error) {

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

	txn, err := s.productRepo.BeginTransaction()
	if err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: begin txn: %w", err)
	}
	defer func() {
		_ = txn.Rollback()
	}()

	product, err := s.productRepo.FetchByID(txn, ctx, order.ProductID)
	if err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: %w", err)
	}

	const epsilon = 0.005
	expected := product.Price * float64(order.Quantity)
	if math.Abs(order.Amount - expected) >= epsilon {
		return models.Order{}, customErrors.IncorrectAmount
	}

	if req.CardNumber == "6969" {
		return models.Order{}, customErrors.FailedTransaction
	}

	_, err = s.productRepo.DecrementStock(txn, ctx, product.ProductID, order.Quantity)
	if err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: %w", err)
	}

	res, err := s.orderRepo.Create(txn, ctx, order)
	if err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: %w", err)
	}

	if err := txn.Commit(); err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: commit failed: %w", err)
	}

	log.Info(ctx, s.log, "created order", zap.String("order_id", res.OrderID))
	return res, nil
}

func (s *orderService) FetchByID(ctx context.Context, id string) (models.Order, error) {
	order, err := s.orderRepo.FetchByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Order{}, customErrors.RecordNotFound
		}

		return models.Order{}, fmt.Errorf("order_service.FetchByID failed: %w", err)
	}
	return order, nil
}

func (s *orderService) FetchAll(ctx context.Context, limit int, offset int) ([]models.Order, error) {
	if limit == 0 {
		limit = utils.GetEnvVarInteger("FETCH_ALL_ORDERS_DEFAULT_LIMIT", 10, s.log)
	}

	res, err := s.orderRepo.FetchAll(ctx, limit, offset)
	if err != nil {
		log.Error(ctx, s.log, "failed to fetch orders", zap.Error(err))
		return res, fmt.Errorf("order_service.FetchAll: %w", err)
	}
	return res, nil
}

func (s *orderService) Delete() {
	panic("unimplemented")
}
