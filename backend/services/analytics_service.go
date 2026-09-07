package services

import (
	"context"
	"time"

	"github.com/avnpl/go-march/repos"
	"go.uber.org/zap"
)

type AnalyticsService interface {
	GetTotalSales(ctx context.Context, start, end time.Time) (int, float64, error)
}

type analyticsService struct {
	orderRepo repos.OrderRepo
	log       *zap.Logger
}

func NewAnalyticsService(orderRepo repos.OrderRepo, logger *zap.Logger) AnalyticsService {
	return &analyticsService{orderRepo: orderRepo, log: logger}
}

func (s *analyticsService) GetTotalSales(ctx context.Context, start, end time.Time) (int, float64, error) {
	return s.orderRepo.GetTotalSales(ctx, start, end)
}
