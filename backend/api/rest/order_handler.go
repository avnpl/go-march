package rest

import (
	"net/http"

	"github.com/avnpl/go-march/services"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type OrderHandler struct {
	svc      services.OrderService
	logger   *zap.Logger
	validate *validator.Validate
}

func NewOrderHandler(svc services.OrderService, logger *zap.Logger, validate *validator.Validate) OrderHandler {
	return OrderHandler{svc: svc, logger: logger, validate: validate}
}

func (h OrderHandler) RegisterRoutes(mux *http.ServeMux) {
	// TODO
}
