package graphql

import (
	"github.com/graphql-go/graphql"
)

func GetQueryFields(resolver *Resolver) graphql.Fields {
	return graphql.Fields{
		"product": &graphql.Field{
			Type:    ProductType,
			Resolve: resolver.Product,
		},
	}
}
