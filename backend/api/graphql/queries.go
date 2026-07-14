package graphql

import (
	"github.com/graphql-go/graphql"
)

func GetQueryFields(resolver *Resolver) graphql.Fields {
	return graphql.Fields{
		"getProductByID": &graphql.Field{
			Type: ProductType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type:        graphql.NewNonNull(graphql.String),
					Description: "The ID of the product to fetch",
				},
			},
			Resolve:     resolver.GetProductByID,
			Description: "Fetch a single product by ID",
		},
		"getAllProducts": &graphql.Field{
			Type:        graphql.NewList(ProductType),
			Resolve:     resolver.GetAllProducts,
			Description: "Fetch all products",
			Args: graphql.FieldConfigArgument{
				"limit": &graphql.ArgumentConfig{
					Type:         graphql.Int,
					DefaultValue: 10,
				},
				"offset": &graphql.ArgumentConfig{
					Type:         graphql.Int,
					DefaultValue: 0,
				},
			},
		},
	}
}
