package graphql

import (
	"context"
	"strconv"

	"github.com/avnpl/go-march/services"
	"github.com/graphql-go/graphql"
)

type Resolver struct {
	productService services.ProductService
}

func NewResolver(productService services.ProductService) *Resolver {
	return &Resolver{
		productService: productService,
	}
}

func (r *Resolver) Product(p graphql.ResolveParams) (interface{}, error) {

	idStr, ok := p.Args["id"].(string)
	if !ok {
		return nil, nil
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, err
	}

	ctx := p.Context
	if ctx == nil {
		ctx = context.Background()
	}

	product, err := r.productService.GetProductByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return product, nil
}

func (r *Resolver) Products(p graphql.ResolveParams) (interface{}, error) {

	ctx := p.Context
	if ctx == nil {
		ctx = context.Background()
	}

	products, err := r.productService.GetAllProducts(ctx)
	if err != nil {
		return nil, err
	}

	return products, nil
}
