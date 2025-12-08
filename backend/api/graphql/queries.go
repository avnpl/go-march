package graphql

import (
	"github.com/graphql-go/graphql"
)

func GetQueryFields(resolver *Resolver) graphql.Fields {
	return graphql.Fields{
		"product": &graphql.Field{
			Type: ProductType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type:        graphql.NewNonNull(graphql.String),
					Description: "The ID of the product to fetch",
				},
			},
			Resolve:     resolver.Product,
			Description: "Fetch a single product by ID",
		},
		"products": &graphql.Field{
			Type:        graphql.NewList(ProductType),
			Resolve:     resolver.Products,
			Description: "Fetch all products",
		},
	}
}
