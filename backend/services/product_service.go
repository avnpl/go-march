package services

import (
	"context"
	"fmt"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/repos"
	"go.uber.org/zap"
)

type ProductService interface {
	CreateProduct(ctx context.Context, req *models.CreateProductReq) (*models.Product, error)
	GetProductByID(ctx context.Context, id int64) (models.Product, error)
	//DeleteProduct(ctx context.Context, id int64) error
	// UpdateProduct(ctx context.Context, id int64, req *models.UpdateProductRequest) (*models.Product, error)
}

type productService struct {
	repo repos.ProductRepo
	log  *zap.Logger
}

func NewProductService(r repos.ProductRepo, l *zap.Logger) ProductService {
	return &productService{repo: r, log: l}
}

func (s *productService) CreateProduct(ctx context.Context, req *models.CreateProductReq) (*models.Product, error) {
	// TODO Validate request

	p := &models.Product{
		Name:  req.Name,
		Price: req.Price,
		Stock: req.Stock,
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("service create: %w", err)
	}

	s.log.Info("created product", zap.String("prod_id", p.ProductID))
	return p, nil
}

func (s *productService) GetProductByID(ctx context.Context, id int64) (models.Product, error) {
	var res models.Product
	res, err := s.repo.FetchByID(ctx, id)
	if err != nil {
		return res, fmt.Errorf("fetch products: %w", err)
	}

	return res, nil
}
