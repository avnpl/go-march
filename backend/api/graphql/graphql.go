package graphql

import (
	"fmt"

	"github.com/avnpl/go-march/repos"
	"github.com/avnpl/go-march/utils"
	"github.com/graphql-go/graphql"
)

func createProductType() *graphql.Object {
	return graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Product",
			Fields: graphql.Fields{
				"prod_id": &graphql.Field{
					Type: graphql.String,
				},
				"prod_name": &graphql.Field{
					Type: graphql.String,
				},
				"price": &graphql.Field{
					Type: graphql.Float,
				},
				"stock": &graphql.Field{
					Type: graphql.Int,
				},
				"created_at": &graphql.Field{
					Type: graphql.DateTime,
				},
				"updated_at": &graphql.Field{
					Type: graphql.DateTime,
				},
			},
		})
}

func queryType(productType *graphql.Object) *graphql.Object {
	logger := utils.BuildLogger()
	defer logger.Sync()

	db := utils.GetDBPoolObject(logger)
	defer db.Close()

	// Initialize the layers
	repo := repos.NewPGProductRepo(db)

	return graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"products": &graphql.Field{
					Type: graphql.NewList(productType),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						res, err := repo.FetchAllProducts(nil)
						if err != nil {
							return res, fmt.Errorf("product_service.GetAll: %w", err)
						}
						return res, nil
					},
				},
			},
		})
}
