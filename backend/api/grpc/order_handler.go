package grpc

import (
	"context"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/services"
	"github.com/avnpl/go-march/utils"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/avnpl/go-march/api/grpc/proto"
)

type OrderHandler struct {
	pb.UnimplementedOrderServiceServer
	svc      services.OrderService
	logger   *zap.Logger
	validate *validator.Validate
}

func NewOrderHandler(svc services.OrderService, logger *zap.Logger, validate *validator.Validate) *OrderHandler {
	return &OrderHandler{svc: svc, logger: logger, validate: validate}
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.Order, error) {
	h.logger.Info("CreateOrder called")

	createReq := models.CreateOrderReq{
		ProductID:       req.GetProdId(),
		Quantity:        int(req.GetQuantity()),
		Amount:          req.GetAmount(),
		ShippingAddress: req.GetShippingAddress(),
		Notes:           req.GetNotes(),
		CardNumber:      req.GetCardNum(),
	}
	if err := h.validate.Struct(createReq); err != nil {
		return nil, status.Error(codes.InvalidArgument, utils.FormatValidationErrors(err))
	}

	order, err := h.svc.Create(ctx, createReq)
	if err != nil {
		h.logger.Error("CreateOrder failed", zap.Error(err))
		return nil, grpcError(err)
	}
	return orderToProto(order), nil
}

func (h *OrderHandler) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.Order, error) {
	h.logger.Info("GetOrder called")

	id := req.GetOrderId()
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := h.svc.FetchByID(ctx, id)
	if err != nil {
		h.logger.Error("GetOrder failed", zap.Error(err), zap.String("id", id))
		return nil, grpcError(err)
	}
	return orderToProto(order), nil
}

func (h *OrderHandler) ListOrders(ctx context.Context, req *pb.ListOrdersRequest) (*pb.ListOrdersResponse, error) {
	h.logger.Info("ListOrders called")

	limit := int(req.GetLimit())
	offset := int(req.GetOffset())
	if limit < 0 || offset < 0 {
		return nil, status.Error(codes.InvalidArgument, "limit and offset must be non-negative")
	}

	orders, err := h.svc.FetchAll(ctx, limit, offset)
	if err != nil {
		h.logger.Error("ListOrders failed", zap.Error(err))
		return nil, grpcError(err)
	}
	return &pb.ListOrdersResponse{Orders: ordersToProto(orders)}, nil
}
