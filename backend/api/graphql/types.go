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

var UpdateProductInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "UpdateProductInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"prod_id": &graphql.InputObjectFieldConfig{
			Type:        graphql.NewNonNull(graphql.Int),
			Description: "The ID of the product to update",
		},
		"name": &graphql.InputObjectFieldConfig{
			Type:        graphql.String,
			Description: "The new name of the product (optional)",
		},
		"price": &graphql.InputObjectFieldConfig{
			Type:        graphql.Float,
			Description: "The new price of the product (optional)",
		},
		"stock": &graphql.InputObjectFieldConfig{
			Type:        graphql.Int,
			Description: "The new stock quantity (optional)",
		},
	},
})
