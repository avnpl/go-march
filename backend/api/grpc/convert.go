package grpc

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/avnpl/go-march/api/grpc/proto"
	"github.com/avnpl/go-march/models"
)

func productToProto(p models.Product) *pb.Product {
	return &pb.Product{
		ProdId:    p.ProductID,
		ProdName:  p.Name,
		Price:     p.Price,
		Stock:     int32(p.Stock),
		CreatedAt: timestamppb.New(p.CreatedAt),
		UpdatedAt: timestamppb.New(p.UpdatedAt),
	}
}

func productsToProto(products []models.Product) []*pb.Product {
	out := make([]*pb.Product, 0, len(products))
	for _, p := range products {
		out = append(out, productToProto(p))
	}
	return out
}

func orderToProto(o models.Order) *pb.Order {
	return &pb.Order{
		OrderId:         o.OrderID,
		ProductId:       o.ProductID,
		Quantity:        int32(o.Quantity),
		Amount:          o.Amount,
		CreatedAt:       timestamppb.New(o.CreatedAt),
		Status:          o.Status,
		ShippingAddress: o.ShippingAddress,
		CardNumber:      o.CardNumber,
		Notes:           o.Notes,
	}
}

func ordersToProto(orders []models.Order) []*pb.Order {
	out := make([]*pb.Order, 0, len(orders))
	for _, o := range orders {
		out = append(out, orderToProto(o))
	}
	return out
}
