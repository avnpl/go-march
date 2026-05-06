package graphql

import (
	"github.com/avnpl/go-march/services"
	"github.com/graphql-go/graphql"
	"go.uber.org/zap"
)

var (
	QueryType    *graphql.Object
	MutationType *graphql.Object
	Schema       graphql.Schema
)

func NewSchema(productService services.ProductService, logger *zap.Logger) error {
	resolver := NewResolver(productService, logger)

	QueryType = graphql.NewObject(graphql.ObjectConfig{
		Name:   "Query",
		Fields: GetQueryFields(resolver),
	})

	MutationType = graphql.NewObject(graphql.ObjectConfig{
		Name:   "Mutation",
		Fields: GetMutationFields(resolver),
	})

	var err error
	Schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query:    QueryType,
		Mutation: MutationType,
	})

	return err
}
