package graphql

import (
	"github.com/avnpl/go-march/models"
	"github.com/graphql-go/graphql"
)

var productType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Product",
	Fields: graphql.Fields{
		"prod_id":    &graphql.Field{Type: graphql.String},
		"prod_name":  &graphql.Field{Type: graphql.String},
		"price":      &graphql.Field{Type: graphql.Float},
		"stock":      &graphql.Field{Type: graphql.Int},
		"created_at": &graphql.Field{Type: graphql.String},
		"updated_at": &graphql.Field{Type: graphql.String},
	},
})

var FetchQuery = graphql.NewObject(graphql.ObjectConfig{
	Name: "Query",
	Fields: graphql.Fields{
		"product": &graphql.Field{
			Type: productType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return &models.Product{
					ProductID: "1234567890",
					Name:      "AirPods",
					Price:     99,
					Stock:     33,
					CreatedAt: "2024-01-01",
					UpdatedAt: "2024-01-02",
				}, nil
			},
		},
	},
})

var Schema, _ = graphql.NewSchema(graphql.SchemaConfig{
	Query: FetchQuery,
})
