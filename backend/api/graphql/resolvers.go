package graphql

import (
	"github.com/avnpl/go-march/models"
	"github.com/graphql-go/graphql"
)

type Resolver struct{}

func (r *Resolver) Product(p graphql.ResolveParams) (interface{}, error) {
	// TODO: Replace with actual database call
	return &models.Product{
		ProductID: "1234567890",
		Name:      "AirPods",
		Price:     99,
		Stock:     33,
		CreatedAt: "2024-01-01",
		UpdatedAt: "2024-01-02",
	}, nil
}
