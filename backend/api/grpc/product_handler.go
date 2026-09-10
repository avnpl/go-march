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

type ProductHandler struct {
	pb.UnimplementedProductServiceServer
	svc      services.ProductService
	logger   *zap.Logger
	validate *validator.Validate
}

func NewProductHandler(svc services.ProductService, logger *zap.Logger, validate *validator.Validate) *ProductHandler {
	return &ProductHandler{svc: svc, logger: logger, validate: validate}
}

func (h *ProductHandler) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.Product, error) {
	h.logger.Info("CreateProduct called")

	createReq := models.CreateProductReq{
		Name:  req.GetName(),
		Price: req.GetPrice(),
		Stock: int(req.GetStock()),
	}
	if err := h.validate.Struct(createReq); err != nil {
		return nil, status.Error(codes.InvalidArgument, utils.FormatValidationErrors(err))
	}

	prod, err := h.svc.CreateProduct(ctx, &createReq)
	if err != nil {
		h.logger.Error("CreateProduct failed", zap.Error(err))
		return nil, grpcError(err)
	}
	return productToProto(prod), nil
}

func (h *ProductHandler) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.Product, error) {
	h.logger.Info("GetProduct called")

	id := req.GetProdId()
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "prod_id is required")
	}

	prod, err := h.svc.GetProductByID(ctx, id)
	if err != nil {
		h.logger.Error("GetProduct failed", zap.Error(err), zap.String("id", id))
		return nil, grpcError(err)
	}
	return productToProto(prod), nil
}

func (h *ProductHandler) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	h.logger.Info("ListProducts called")

	limit := int(req.GetLimit())
	offset := int(req.GetOffset())
	if limit < 0 || offset < 0 {
		return nil, status.Error(codes.InvalidArgument, "limit and offset must be non-negative")
	}

	prods, err := h.svc.GetAllProducts(ctx, limit, offset)
	if err != nil {
		h.logger.Error("ListProducts failed", zap.Error(err))
		return nil, grpcError(err)
	}
	return &pb.ListProductsResponse{Products: productsToProto(prods)}, nil
}

func (h *ProductHandler) UpdateProduct(ctx context.Context, req *pb.UpdateProductRequest) (*pb.Product, error) {
	h.logger.Info("UpdateProduct called")

	id := req.GetProdId()
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "prod_id is required")
	}

	updateReq := models.UpdateProductReq{ProductID: id}
	if req.Name != nil {
		updateReq.Name = *req.Name
	}
	if req.Price != nil {
		price := *req.Price
		updateReq.Price = &price
	}
	if req.Stock != nil {
		stock := int(*req.Stock)
		updateReq.Stock = &stock
	}

	if err := h.validate.Struct(updateReq); err != nil {
		return nil, status.Error(codes.InvalidArgument, utils.FormatValidationErrors(err))
	}

	prod, err := h.svc.UpdateProduct(ctx, &updateReq)
	if err != nil {
		h.logger.Error("UpdateProduct failed", zap.Error(err), zap.String("id", id))
		return nil, grpcError(err)
	}
	return productToProto(prod), nil
}

func (h *ProductHandler) DeleteProduct(ctx context.Context, req *pb.DeleteProductRequest) (*pb.Product, error) {
	h.logger.Info("DeleteProduct called")

	id := req.GetProdId()
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "prod_id is required")
	}

	prod, err := h.svc.DeleteProduct(ctx, id)
	if err != nil {
		h.logger.Error("DeleteProduct failed", zap.Error(err), zap.String("id", id))
		return nil, grpcError(err)
	}
	return productToProto(prod), nil
}
