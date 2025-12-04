package graphql

import (
	"github.com/graphql-go/graphql"
)

var ProductType = graphql.NewObject(graphql.ObjectConfig{
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
