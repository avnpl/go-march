package graphql

import (
	"context"

	"github.com/avnpl/go-march/models"
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

func (r *Resolver) GetProductByID(p graphql.ResolveParams) (interface{}, error) {

	idStr, ok := p.Args["id"].(string)
	if !ok {
		return nil, nil
	}

	ctx := p.Context
	if ctx == nil {
		ctx = context.Background()
	}

	product, err := r.productService.GetProductByID(ctx, idStr)
	if err != nil {
		return nil, err
	}

	return product, nil
}

func (r *Resolver) GetAllProducts(p graphql.ResolveParams) (interface{}, error) {
	limit := 10
	offset := 0

	if limitVal, ok := p.Args["limit"].(int); ok {
		limit = limitVal
	}
	if offsetVal, ok := p.Args["offset"].(int); ok {
		offset = offsetVal
	}

	ctx := p.Context
	if ctx == nil {
		ctx = context.Background()
	}

	products, err := r.productService.GetAllProducts(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	return products, nil
}

func (r *Resolver) UpdateProduct(p graphql.ResolveParams) (interface{}, error) {
	input, ok := p.Args["input"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	prodID, ok := input["prod_id"].(string)
	if !ok {
		return nil, nil
	}

	req := &models.UpdateProductReq{
		ProductID: prodID,
	}

	if name, ok := input["name"].(string); ok && name != "" {
		req.Name = name
	}

	if price, ok := input["price"].(float64); ok {
		req.Price = price
	}

	if stock, ok := input["stock"].(int); ok {
		req.Stock = stock
	}

	ctx := p.Context
	if ctx == nil {
		ctx = context.Background()
	}

	product, err := r.productService.UpdateProduct(ctx, req)
	if err != nil {
		return nil, err
	}

	return product, nil
}

func (r *Resolver) DeleteProduct(p graphql.ResolveParams) (interface{}, error) {

	ctx := p.Context
	if ctx == nil {
		ctx = context.Background()
	}

	input, ok := p.Args["input"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	productID, ok := input["prod_id"].(string)
	if !ok {
		return nil, nil
	}

	product, err := r.productService.DeleteProduct(ctx, productID)
	if err != nil {
		return nil, err
	}

	return product, nil
}
