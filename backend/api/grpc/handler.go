package grpc

import (
	"context"
	"log"

	pb "github.com/avnpl/go-march/api/grpc/proto"
	"github.com/avnpl/go-march/services"
)

type AnalyticsHandler struct {
	pb.UnimplementedAnalyticsServiceServer
	analyticsService services.AnalyticsService
}

func NewAnalyticsHandler(analyticsService services.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{analyticsService: analyticsService}
}

// TODO: use pointer receiver (*AnalyticsHandler) — constructor already returns a pointer
func (s AnalyticsHandler) GetTotalSales(ctx context.Context, req *pb.GetTotalSalesRequest) (*pb.GetTotalSalesResponse, error) {
	// TODO: use zap logger instead of std log
	log.Println("🔥 GetTotalSales called")

	// TODO: reject nil start_date / end_date with status.Error(codes.InvalidArgument, ...)
	totalSales, totalRevenue, err := s.analyticsService.GetTotalSales(ctx, req.StartDate.AsTime(), req.EndDate.AsTime())
	if err != nil {
		return nil, err
	}

	return &pb.GetTotalSalesResponse{TotalOrders: int64(totalSales), TotalRevenue: totalRevenue}, nil
}
