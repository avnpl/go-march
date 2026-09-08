package grpc

import (
	"context"

	pb "github.com/avnpl/go-march/api/grpc/proto"
	"github.com/avnpl/go-march/services"
	"go.uber.org/zap"
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

	// TODO: reject nil start_date / end_date with status.Error(codes.InvalidArgument, ...)
	totalSales, totalRevenue, err := s.analyticsService.GetTotalSales(ctx, req.StartDate.AsTime(), req.EndDate.AsTime())
	if err != nil {
		s.logger.Error("GetTotalSales failed", zap.Error(err))
		return nil, err
	}

	return &pb.GetTotalSalesResponse{TotalOrders: int64(totalSales), TotalRevenue: totalRevenue}, nil
}
