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
	"github.com/avnpl/go-march/utils/customErrors"
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
			SendErrorResponse(r.Context(), w, customErrors.InvalidHTTPMethod)
		}
	})
	mux.HandleFunc("/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.fetchOrderByID(w, r)
		default:
			SendErrorResponse(r.Context(), w, customErrors.InvalidHTTPMethod)
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
		SendErrorResponse(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

func (oh OrderHandler) fetchOrderByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := r.PathValue("id")
	if idStr == "" {
		utils.SendJSONError(w, http.StatusBadRequest, "No ID provided in the request")
		return
	}

	log.Debug(ctx, oh.logger, "fetching order", zap.String("id", idStr))
	order, err := oh.svc.FetchByID(ctx, idStr)
	if err != nil {
		log.Error(ctx, oh.logger, "fetchOrderByID failed", zap.Error(err))
		SendErrorResponse(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(order)
}
