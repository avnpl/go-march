package services

import (
	"context"
	"fmt"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/repos"
	"go.uber.org/zap"
)

type ProductService interface {
	CreateProduct(ctx context.Context, req *models.CreateProductReq) (models.Product, error)
	GetProductByID(ctx context.Context, id int64) (models.Product, error)
	GetAllProducts(ctx context.Context) ([]models.Product, error)
	UpdateProduct(ctx context.Context, req *models.UpdateProductReq) (models.Product, error)
	DeleteProduct(ctx context.Context, id int64) (models.Product, error)
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
		Name:  req.Name,
		Price: req.Price,
		Stock: req.Stock,
	}

	res, err := s.repo.Create(ctx, &p)
	if err != nil {
		return models.Product{}, fmt.Errorf("product_service.Create: %w", err)
	}

	s.log.Info("created product", zap.String("prod_id", p.ProductID))
	return res, nil
}

func (s *productService) GetProductByID(ctx context.Context, id int64) (models.Product, error) {
	var res models.Product
	res, err := s.repo.FetchByID(ctx, id)
	if err != nil {
		return res, fmt.Errorf("product_service.Get: %w", err)
	}

	return res, nil
}

func (s *productService) GetAllProducts(ctx context.Context) ([]models.Product, error) {
	res, err := s.repo.FetchAllProducts(ctx)
	if err != nil {
		return res, fmt.Errorf("product_service.GetAll: %w", err)
	}
	return res, nil
}

func (s *productService) UpdateProduct(ctx context.Context, req *models.UpdateProductReq) (models.Product, error) {
	res, err := s.repo.UpdateByID(&ctx, req)
	if err != nil {
		return models.Product{}, fmt.Errorf("product_service.Update: %w", err)
	}
	s.log.Info("updated product", zap.String("prod_id", res.ProductID))
	return res, nil
}

func (s *productService) DeleteProduct(ctx context.Context, id int64) (models.Product, error) {
	res, err := s.repo.DeleteByID(ctx, id)
	if err != nil {
		return models.Product{}, fmt.Errorf("prod_service.Delete: %w", err)
	}

	s.log.Info("deleted product", zap.String("prod_id", res.ProductID))
	return res, nil
}
