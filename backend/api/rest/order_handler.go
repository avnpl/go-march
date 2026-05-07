package rest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/services"
	"github.com/avnpl/go-march/utils"
	"github.com/avnpl/go-march/utils/log"
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
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.createOrder(w, r)
		default:
			utils.SendJSONError(w, http.StatusMethodNotAllowed, "Invalid HTTP Method")
		}
	})
}

func (h OrderHandler) createOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error(ctx, h.logger, "error reading the request body", zap.Error(err))
		utils.SendJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	reqString := string(bodyBytes)
	log.Debug(ctx, h.logger, "raw body",
		zap.Int("length", len(bodyBytes)),
		zap.String("body", strings.ReplaceAll(reqString, " ", "")),
	)
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var req models.CreateOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, h.logger, "invalid JSON", zap.Error(err))
		utils.SendJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	err = h.validate.Struct(req)
	if err != nil {
		message := utils.FormatValidationErrors(err)
		utils.SendJSONError(w, http.StatusBadRequest, message)
		return
	}

	order, err := h.svc.Create(ctx, req)
	if err != nil {
		log.Error(ctx, h.logger, "CreateOrder failed", zap.Error(err))
		utils.SendInternalError(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}
