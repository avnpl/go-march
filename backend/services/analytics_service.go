package services

import (
	"context"
	"time"

	"github.com/avnpl/go-march/repos"
	"github.com/avnpl/go-march/utils/log"
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
	totalOrders, totalRevenue, err := s.orderRepo.GetTotalSales(ctx, start, end)
	if err != nil {
		log.Error(ctx, s.log, "failed to get total sales", zap.Time("start", start), zap.Time("end", end), zap.Error(err))
		return 0, 0, err
	}
	return totalOrders, totalRevenue, nil
}
