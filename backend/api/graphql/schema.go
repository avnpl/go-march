package graphql

import (
	"github.com/graphql-go/graphql"
)

var (
	resolver = &Resolver{}

	QueryType = graphql.NewObject(graphql.ObjectConfig{
		Name:   "Query",
		Fields: GetQueryFields(resolver),
	})

	MutationType = graphql.NewObject(graphql.ObjectConfig{
		Name:   "Mutation",
		Fields: GetMutationFields(resolver),
	})

	Schema, _ = graphql.NewSchema(graphql.SchemaConfig{
		Query:    QueryType,
		Mutation: MutationType,
	})
)
