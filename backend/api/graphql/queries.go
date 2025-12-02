package graphql

import (
	"github.com/avnpl/go-march/models"
)

type Resolver struct{}

func (r *Resolver) Product() (*models.Product, error) {
	return &models.Product{
		ProductID: "1234567890",
		Name:      "AirPods",
		Price:     99,
		Stock:     33,
		CreatedAt: "2024-01-01",
		UpdatedAt: "2024-01-02",
	}, nil
}
