package graphql

import (
	"github.com/avnpl/go-march/services"
	"github.com/graphql-go/graphql"
	"go.uber.org/zap"
)

func CreateNewSchema(productService services.ProductService, logger *zap.Logger) (graphql.Schema, error) {
	resolver := NewResolver(productService, logger)

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name:   "Query",
		Fields: GetQueryFields(resolver),
	})

	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name:   "Mutation",
		Fields: GetMutationFields(resolver),
	})

	return graphql.NewSchema(graphql.SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
	})
}
