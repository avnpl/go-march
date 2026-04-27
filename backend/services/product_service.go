package services

import (
	"context"
	"fmt"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/repos"
	"github.com/avnpl/go-march/utils"
	"github.com/avnpl/go-march/utils/trace"
	"go.uber.org/zap"
)

type ProductService interface {
	CreateProduct(ctx context.Context, req *models.CreateProductReq) (models.Product, error)
	GetProductByID(ctx context.Context, id string) (models.Product, error)
	GetAllProducts(ctx context.Context, limit int, offset int) ([]models.Product, error)
	UpdateProduct(ctx context.Context, req *models.UpdateProductReq) (models.Product, error)
	DeleteProduct(ctx context.Context, id string) (models.Product, error)
}

type productService struct {
	repo repos.ProductRepo
	log  *zap.Logger
}

func NewProductService(r repos.ProductRepo, l *zap.Logger) ProductService {
	return &productService{repo: r, log: l}
}

func (s *productService) CreateProduct(ctx context.Context, req *models.CreateProductReq) (models.Product, error) {
	p := models.Product{
		Name:      req.Name,
		Price:     req.Price,
		Stock:     req.Stock,
		ProductID: utils.GenerateID("PR"),
	}

	res, err := s.repo.Create(ctx, &p)
	if err != nil {
		return models.Product{}, fmt.Errorf("product_service.Create: %w", err)
	}

	trace.Info(ctx, s.log, "created product", zap.String("prod_id", p.ProductID))
	return res, nil
}

func (s *productService) GetProductByID(ctx context.Context, id string) (models.Product, error) {
	var res models.Product
	res, err := s.repo.FetchByID(ctx, id)
	if err != nil {
		trace.Error(ctx, s.log, "failed to fetch product", zap.String("id", id), zap.Error(err))
		return res, fmt.Errorf("product_service.Get: %w", err)
	}

	return res, nil
}

func (s *productService) GetAllProducts(ctx context.Context, limit int, offset int) ([]models.Product, error) {
	res, err := s.repo.FetchAll(ctx, limit, offset)
	if err != nil {
		trace.Error(ctx, s.log, "failed to fetch products", zap.Error(err))
		return res, fmt.Errorf("product_service.GetAll: %w", err)
	}
	return res, nil
}

func (s *productService) UpdateProduct(ctx context.Context, req *models.UpdateProductReq) (models.Product, error) {
	res, err := s.repo.UpdateByID(ctx, req)
	if err != nil {
		trace.Error(ctx, s.log, "failed to update product", zap.Error(err))
		return models.Product{}, fmt.Errorf("product_service.Update: %w", err)
	}
	trace.Info(ctx, s.log, "updated product", zap.String("prod_id", res.ProductID))
	return res, nil
}

func (s *productService) DeleteProduct(ctx context.Context, id string) (models.Product, error) {
	res, err := s.repo.DeleteByID(ctx, id)
	if err != nil {
		trace.Error(ctx, s.log, "failed to delete product", zap.String("id", id), zap.Error(err))
		return models.Product{}, fmt.Errorf("prod_service.Delete: %w", err)
	}

	trace.Info(ctx, s.log, "deleted product", zap.String("prod_id", res.ProductID))
	return res, nil
}
