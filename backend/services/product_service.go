package services

import (
	"context"
	"fmt"
	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/repos"
	"go.uber.org/zap"
	"strings"
)

type ProductService interface {
	CreateProduct(ctx context.Context, req *models.CreateProductReq) (*models.Product, error)
	//GetProductByID(ctx context.Context, id int64) (*models.Product, error)
	//DeleteProduct(ctx context.Context, id int64) error
	// UpdateProduct(ctx context.Context, id int64, req *models.UpdateProductRequest) (*models.Product, error)
}

// productService is our implementation.
type productService struct {
	repo repos.ProductRepo
	//validator *validate.Validator
	log *zap.Logger
}

func NewProductService(r repos.ProductRepo, l *zap.Logger) ProductService {
	return &productService{repo: r, log: l}
}

// CreateProduct applies validation then calls the repo.
func (s *productService) CreateProduct(ctx context.Context, req *models.CreateProductReq) (*models.Product, error) {
	// 1) Validate request fields
	//if err := s.validator.Struct(req); err != nil {
	//	return nil, fmt.Errorf("service validation: %w", err)
	//}

	// 2) Build the model
	p := &models.Product{
		Name:  req.Name,
		Price: req.Price,
		Stock: req.Stock,
	}

	// 3) Persist
	if err := s.repo.Create(ctx, p); err != nil {
		// If the repo signaled a uniqueness conflict:
		if strings.Contains(err.Error(), "conflict") {
			return nil, fmt.Errorf("ErrConflict")
		}
		return nil, fmt.Errorf("service create: %w", err)
	}

	s.log.Info("created product", zap.String("prod_id", p.ProductID))
	return p, nil
}
