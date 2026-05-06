package services

import (
	"github.com/avnpl/go-march/repos"
	"go.uber.org/zap"
)

type OrderService interface {
	Create()
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

func (o *orderService) Create() {
	panic("unimplemented")
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
