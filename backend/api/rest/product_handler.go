package rest

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/services"
	"github.com/avnpl/go-march/utils"
	"github.com/avnpl/go-march/utils/trace"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type ProductHandler struct {
	svc      services.ProductService
	logger  *zap.Logger
	validate *validator.Validate
}

func NewProductHandler(svc services.ProductService, logger *zap.Logger, validate *validator.Validate) ProductHandler {
	return ProductHandler{svc: svc, logger: logger, validate: validate}
}

func (h ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		trace.Error(ctx, h.logger, "error reading the request body", zap.Error(err))
		utils.SendJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	reqString := string(bodyBytes)
	trace.Debug(ctx, h.logger, "raw body",
		zap.Int("length", len(bodyBytes)),
		zap.String("body", strings.ReplaceAll(reqString, " ", "")),
	)
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var req models.CreateProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		trace.Error(ctx, h.logger, "invalid JSON", zap.Error(err))
		utils.SendJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	err = h.validate.Struct(req)
	if err != nil {
		message := utils.FormatValidationErrors(err)
		utils.SendJSONError(w, http.StatusBadRequest, message)
		return
	}

	prod, err := h.svc.CreateProduct(ctx, &req)
	if err != nil {
		trace.Error(ctx, h.logger, "CreateProduct failed", zap.Error(err))
		if errors.Is(err, utils.ErrConflict) {
			utils.SendJSONError(w, http.StatusConflict, "")
			return
		}
		utils.SendInternalError(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(prod)
}

func (h ProductHandler) FetchProduct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := r.PathValue("id")
	if idStr == "" {
		trace.Error(ctx, h.logger, "no ID provided in request")
		utils.SendJSONError(w, http.StatusBadRequest, "No ID provided in the request")
		return
	}

	trace.Debug(ctx, h.logger, "fetching product", zap.String("id", idStr))

	prod, err := h.svc.GetProductByID(ctx, idStr)
	if err != nil {
		trace.Error(ctx, h.logger, "GetProductByID failed", zap.Error(err))
		if errors.Is(err, sql.ErrNoRows) {
			utils.SendJSONError(w, http.StatusNotFound, "Record with given ID not found")
			return
		}
		utils.SendInternalError(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(prod)
}

func (h ProductHandler) FetchAllProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	queryParams := r.URL.Query()
	limit := 0
	offset := 0
	limitStr := queryParams.Get("limit")
	offsetStr := queryParams.Get("offset")
	var err error

	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			trace.Error(ctx, h.logger, "Invalid query param, limit", zap.String("limit", limitStr), zap.Error(err))
			utils.SendJSONError(w, http.StatusBadRequest, "Invalid query param, limit")
			return
		}
	}

	if offsetStr != "" {
		offset, err = strconv.Atoi(limitStr)
		if err != nil {
			trace.Error(ctx, h.logger, "Invalid query param, offset", zap.String("offset", offsetStr), zap.Error(err))
			utils.SendJSONError(w, http.StatusBadRequest, "Invalid query param, offset")
			return
		}
	}

	prods, err := h.svc.GetAllProducts(ctx, limit, offset)
	if err != nil {
		trace.Error(ctx, h.logger, "FetchAllProducts failed", zap.Error(err))
		utils.SendInternalError(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(prods)
}

func (h ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := r.PathValue("id")
	if idStr == "" {
		trace.Error(ctx, h.logger, "no ID provided in request")
		utils.SendJSONError(w, http.StatusBadRequest, "No ID provided in the request")
		return
	}

	trace.Debug(ctx, h.logger, "updating product", zap.String("id", idStr))

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		trace.Error(ctx, h.logger, "error reading the request body", zap.Error(err))
		utils.SendInternalError(w)
		return
	}

	reqString := string(bodyBytes)
	trace.Debug(ctx, h.logger, "raw body",
		zap.Int("length", len(bodyBytes)),
		zap.String("body", strings.ReplaceAll(reqString, " ", "")),
	)
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var req models.UpdateProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		trace.Error(ctx, h.logger, "invalid JSON", zap.Error(err))
		utils.SendJSONError(w, http.StatusBadRequest, "Invalid JSON in the Request Body")
		return
	}

	req.ProductID = idStr

	err = h.validate.Struct(req)
	if err != nil {
		message := utils.FormatValidationErrors(err)
		utils.SendJSONError(w, http.StatusBadRequest, message)
		return
	}

	prod, err := h.svc.UpdateProduct(ctx, &req)
	if err != nil {
		if errors.Is(err, utils.ErrConflict) {
			utils.SendJSONError(w, http.StatusConflict, "")
			return
		}
		utils.SendInternalError(w)
		trace.Error(ctx, h.logger, "UpdateProduct failed", zap.Error(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(prod)
}

func (h ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := r.PathValue("id")
	if idStr == "" {
		trace.Error(ctx, h.logger, "no ID provided in request")
		utils.SendJSONError(w, http.StatusBadRequest, "No ID provided in the request")
		return
	}

	trace.Debug(ctx, h.logger, "deleting product", zap.String("id", idStr))

	prod, err := h.svc.DeleteProduct(ctx, idStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			utils.SendJSONError(w, http.StatusNotFound, "Record with given ID not found")
			return
		}
		trace.Error(ctx, h.logger, "DeleteProductByID failed", zap.Error(err))
		utils.SendInternalError(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(prod)
}
