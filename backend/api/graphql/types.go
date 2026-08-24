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
			Type:        graphql.NewNonNull(graphql.String),
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

var DeleteProductInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "DeleteProductInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"prod_id": &graphql.InputObjectFieldConfig{
			Type:        graphql.String,
			Description: "The ID of the product to be deleted",
		},
	},
})

func NewOrderType(r *Resolver) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Order",
		Fields: graphql.Fields{
			"order_id":         &graphql.Field{Type: graphql.String},
			"product_id":       &graphql.Field{Type: graphql.String},
			"quantity":         &graphql.Field{Type: graphql.Int},
			"amount":           &graphql.Field{Type: graphql.Float},
			"status":           &graphql.Field{Type: graphql.String},
			"shipping_address": &graphql.Field{Type: graphql.String},
			"notes":            &graphql.Field{Type: graphql.String},
			"created_at":       &graphql.Field{Type: graphql.String},
			"product": &graphql.Field{
				Type:    ProductType,
				Resolve: r.ResolveOrderProduct,
			},
		},
	})
}
