package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/avnpl/go-march/api/grpc/proto"
	"github.com/avnpl/go-march/services"
)

type AnalyticsHandler struct {
	pb.UnimplementedAnalyticsServiceServer
	analyticsService services.AnalyticsService
	logger           *zap.Logger
}

func NewAnalyticsHandler(analyticsService services.AnalyticsService, logger *zap.Logger) *AnalyticsHandler {
	return &AnalyticsHandler{analyticsService: analyticsService, logger: logger}
}

func (s *AnalyticsHandler) GetTotalSales(ctx context.Context, req *pb.GetTotalSalesRequest) (*pb.GetTotalSalesResponse, error) {
	s.logger.Info("GetTotalSales called")

	if req.GetStartDate() == nil || req.GetEndDate() == nil {
		return nil, status.Error(codes.InvalidArgument, "start_date and end_date are required")
	}

	totalSales, totalRevenue, err := s.analyticsService.GetTotalSales(ctx, req.StartDate.AsTime(), req.EndDate.AsTime())
	if err != nil {
		s.logger.Error("GetTotalSales failed", zap.Error(err))
		return nil, err
	}

	return &pb.GetTotalSalesResponse{TotalOrders: int64(totalSales), TotalRevenue: totalRevenue}, nil
}

func (s *AnalyticsHandler) GetAverageOrderValue(ctx context.Context, req *pb.GetAverageOrderValueRequest) (*pb.GetAverageOrderValueResponse, error) {
	s.logger.Info("GetAverageOrderValue called")
	if req.GetStartDate() == nil || req.GetEndDate() == nil {
		return nil, status.Error(codes.InvalidArgument, "start_date and end_date are required")
	}

	result, err := s.analyticsService.GetAverageOrderValue(ctx, req.StartDate.AsTime(), req.EndDate.AsTime())
	if err != nil {
		s.logger.Error("GetAverageOrderValue failed", zap.Error(err))
		return nil, err
	}

	return &pb.GetAverageOrderValueResponse{AverageOrderValue: result}, nil
}

func (s *AnalyticsHandler) GetTopProducts(ctx context.Context, req *pb.GetTopProductsRequest) (*pb.GetTopProductsResponse, error) {
	s.logger.Info("GetTopProducts called")

	limit := int(req.GetLimit())
	if limit < 0 {
		return nil, status.Error(codes.InvalidArgument, "limit must be non-negative")
	}
	if limit == 0 {
		limit = 10
	}

	stats, err := s.analyticsService.GetTopProducts(ctx, limit)
	if err != nil {
		s.logger.Error("GetTopProducts failed", zap.Error(err))
		return nil, err
	}

	products := make([]*pb.ProductStat, 0, len(stats))
	for _, st := range stats {
		products = append(products, &pb.ProductStat{
			ProductId:   st.ProductID,
			ProductName: st.ProductName,
			UnitsSold:   st.UnitsSold,
			Revenue:     st.Revenue,
			Stock:       int32(st.Stock),
		})
	}
	return &pb.GetTopProductsResponse{Products: products}, nil
}

func (s *AnalyticsHandler) GetLowStockProducts(ctx context.Context, req *pb.GetLowStockProductsRequest) (*pb.GetLowStockProductsResponse, error) {
	s.logger.Info("GetLowStockProducts called")

	threshold := int(req.GetThreshold())
	if threshold < 0 {
		return nil, status.Error(codes.InvalidArgument, "threshold must be non-negative")
	}

	products, err := s.analyticsService.GetLowStockProducts(ctx, threshold)
	if err != nil {
		s.logger.Error("GetLowStockProducts failed", zap.Error(err))
		return nil, err
	}

	stats := make([]*pb.ProductStat, 0, len(products))
	for _, p := range products {
		stats = append(stats, &pb.ProductStat{
			ProductId:   p.ProductID,
			ProductName: p.Name,
			Stock:       int32(p.Stock),
		})
	}
	return &pb.GetLowStockProductsResponse{Products: stats}, nil
}
