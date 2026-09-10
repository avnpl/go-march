package services

import (
	"context"
	"time"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/repos"
	"github.com/avnpl/go-march/utils/log"
	"go.uber.org/zap"
)

type AnalyticsService interface {
	GetTotalSales(ctx context.Context, start, end time.Time) (int, float64, error)
	GetAverageOrderValue(ctx context.Context, start, end time.Time) (float64, error)
	GetTopProducts(ctx context.Context, limit int) ([]models.ProductStat, error)
	GetLowStockProducts(ctx context.Context, threshold int) ([]models.Product, error)
}

type analyticsService struct {
	orderRepo   repos.OrderRepo
	productRepo repos.ProductRepo
	log         *zap.Logger
}

func NewAnalyticsService(orderRepo repos.OrderRepo, productRepo repos.ProductRepo, logger *zap.Logger) AnalyticsService {
	return &analyticsService{orderRepo: orderRepo, productRepo: productRepo, log: logger}
}

func (s *analyticsService) GetTotalSales(ctx context.Context, start, end time.Time) (int, float64, error) {
	totalOrders, totalRevenue, err := s.orderRepo.GetTotalSales(ctx, start, end)
	if err != nil {
		log.Error(ctx, s.log, "failed to get total sales", zap.Time("start", start), zap.Time("end", end), zap.Error(err))
		return 0, 0, err
	}
	return totalOrders, totalRevenue, nil
}

func (s *analyticsService) GetAverageOrderValue(ctx context.Context, start, end time.Time) (float64, error) {
	result, err := s.orderRepo.GetAvgOrderValue(ctx, start, end)
	if err != nil {
		log.Error(ctx, s.log, "failed to get avg order value", zap.Time("start", start), zap.Time("end", end), zap.Error(err))
		return 0, err
	}
	return result, nil
}

func (s *analyticsService) GetLowStockProducts(ctx context.Context, threshold int) ([]models.Product, error) {
	products, err := s.productRepo.GetLowStockProducts(ctx, threshold)
	if err != nil {
		log.Error(ctx, s.log, "failed to get low stock products", zap.Int("threshold", threshold), zap.Error(err))
		return nil, err
	}
	return products, nil
}

func (s *analyticsService) GetTopProducts(ctx context.Context, limit int) ([]models.ProductStat, error) {
	stats, err := s.orderRepo.GetTopProducts(ctx, limit)
	if err != nil {
		log.Error(ctx, s.log, "failed to get top products", zap.Int("limit", limit), zap.Error(err))
		return nil, err
	}
	return stats, nil
}
