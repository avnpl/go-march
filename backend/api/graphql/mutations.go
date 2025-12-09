package graphql

import (
	"github.com/graphql-go/graphql"
)

func GetMutationFields(resolver *Resolver) graphql.Fields {
	return graphql.Fields{
		"updateProduct": &graphql.Field{
			Type: ProductType,
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{
					Type:        graphql.NewNonNull(UpdateProductInput),
					Description: "Input data for updating a product",
				},
			},
			Resolve:     resolver.UpdateProduct,
			Description: "Update an existing product",
		},
	}
}
