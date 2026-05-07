package services

import (
	"context"
	"fmt"
	"time"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/repos"
	"github.com/avnpl/go-march/utils"
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
	repo repos.OrderRepo
	log  *zap.Logger
}

func NewOrderService(r repos.OrderRepo, l *zap.Logger) OrderService {
	return &orderService{repo: r, log: l}
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
	}

	cardNumEnd := req.CardNumber[len(req.CardNumber)-4:]
	if cardNumEnd == "6969" {
		order.Status = utils.OrderStatusFailed
	} else {
		order.Status = utils.OrderStatusSuccess
	}

	res, err := o.repo.Create(ctx, order)
	if err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: %w", err)
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
