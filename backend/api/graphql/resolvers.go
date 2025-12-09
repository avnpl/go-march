package graphql

import (
	"context"
	"strconv"

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

func (r *Resolver) UpdateProduct(p graphql.ResolveParams) (interface{}, error) {
	input, ok := p.Args["input"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	prodIDRaw, ok := input["prod_id"]
	if !ok {
		return nil, nil
	}

	var prodID int64
	switch v := prodIDRaw.(type) {
	case int:
		prodID = int64(v)
	case int64:
		prodID = v
	default:
		return nil, nil
	}

	req := &models.UpdateProductReq{
		ProductID: &prodID,
	}

	if name, ok := input["name"].(string); ok && name != "" {
		req.Name = &name
	}

	if price, ok := input["price"].(float64); ok {
		req.Price = &price
	}

	if stock, ok := input["stock"].(int); ok {
		req.Stock = &stock
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
