package graphql

import (
	"github.com/avnpl/go-march/services"
	"github.com/graphql-go/graphql"
)

var (
	QueryType    *graphql.Object
	MutationType *graphql.Object
	Schema       graphql.Schema
)

func NewSchema(productService services.ProductService) error {
	resolver := NewResolver(productService)

	QueryType = graphql.NewObject(graphql.ObjectConfig{
		Name:   "Query",
		Fields: GetQueryFields(resolver),
	})

	// MutationType = graphql.NewObject(graphql.ObjectConfig{
	// 	Name:   "Mutation",
	// 	Fields: GetMutationFields(resolver),
	// })

	var err error
	Schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query: QueryType,
		// Mutation: MutationType,
	})

	return err
}
