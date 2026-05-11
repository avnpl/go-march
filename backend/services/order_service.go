package services

import (
	"context"
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
	Fetch()
	FetchAll()
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

func (o *orderService) Create(ctx context.Context, req models.CreateOrderReq) (models.Order, error) {

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

	txn, err := o.productRepo.BeginTransaction()
	defer txn.Rollback()

	product, err := o.productRepo.FetchByID(txn, ctx, order.ProductID)
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

	err = o.productRepo.UpdateProductStock(txn, ctx, product.ProductID, product.Stock-order.Quantity)
	if err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: %w", err)
	}

	res, err := o.orderRepo.Create(txn, ctx, order)
	if err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: %w", err)
	}

	if err := txn.Commit(); err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: commit failed: %w", err)
	}

	log.Info(ctx, o.log, "created order", zap.String("order_id", res.OrderID))
	return res, nil
}

func (o *orderService) Delete() {
	panic("unimplemented")
}

func (o *orderService) Fetch() {
	panic("unimplemented")
}

func (o *orderService) FetchAll() {
	panic("unimplemented")
}
