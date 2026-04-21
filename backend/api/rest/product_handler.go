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
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type ProductHandler struct {
	svc      services.ProductService
	log      *zap.Logger
	validate *validator.Validate
}

func NewProductHandler(svc services.ProductService, log *zap.Logger, validate *validator.Validate) ProductHandler {
	return ProductHandler{svc: svc, log: log, validate: validate}
}

func (h ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.log.Error("error reading the request body", zap.Error(err))
		utils.SendJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	reqString := string(bodyBytes)
	h.log.Debug("raw body",
		zap.Int("length", len(bodyBytes)),
		zap.String("body", strings.ReplaceAll(reqString, " ", "")),
	)
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var req models.CreateProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("invalid JSON", zap.Error(err))
		utils.SendJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	err = h.validate.Struct(req)
	if err != nil {
		message := utils.FormatValidationErrors(err)
		utils.SendJSONError(w, http.StatusBadRequest, message)
		return
	}

	prod, err := h.svc.CreateProduct(r.Context(), &req)
	if err != nil {
		h.log.Error("CreateProduct failed", zap.Error(err))
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
	idStr := r.PathValue("id")
	if idStr == "" {
		h.log.Error("no ID provided in request")
		utils.SendJSONError(w, http.StatusBadRequest, "No ID provided in the request")
		return
	}

	h.log.Debug("fetching product", zap.String("id", idStr))

	prod, err := h.svc.GetProductByID(r.Context(), idStr)
	if err != nil {
		h.log.Error("GetProductByID failed", zap.Error(err))
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

	queryParams := r.URL.Query()
	limit := 0
	offset := 0
	limitStr := queryParams.Get("limit")
	offsetStr := queryParams.Get("offset")
	var err error

	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			h.log.Error("Invalid query param, limit", zap.String("limit", limitStr), zap.Error(err))
			utils.SendJSONError(w, http.StatusBadRequest, "Invalid query param, limit")
			return
		}
	}

	if offsetStr != "" {
		offset, err = strconv.Atoi(limitStr)
		if err != nil {
			h.log.Error("Invalid query param, offset", zap.String("offset", offsetStr), zap.Error(err))
			utils.SendJSONError(w, http.StatusBadRequest, "Invalid query param, offset")
			return
		}
	}

	prods, err := h.svc.GetAllProducts(r.Context(), limit, offset)
	if err != nil {
		h.log.Error("FetchAllProducts failed", zap.Error(err))
		utils.SendInternalError(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(prods)
}

func (h ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		h.log.Error("no ID provided in request")
		utils.SendJSONError(w, http.StatusBadRequest, "No ID provided in the request")
		return
	}

	h.log.Debug("updating product", zap.String("id", idStr))

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.log.Error("error reading the request body", zap.Error(err))
		utils.SendInternalError(w)
		return
	}

	reqString := string(bodyBytes)
	h.log.Debug("raw body",
		zap.Int("length", len(bodyBytes)),
		zap.String("body", strings.ReplaceAll(reqString, " ", "")),
	)
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var req models.UpdateProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("invalid JSON", zap.Error(err))
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

	prod, err := h.svc.UpdateProduct(r.Context(), &req)
	if err != nil {
		if errors.Is(err, utils.ErrConflict) {
			utils.SendJSONError(w, http.StatusConflict, "")
			return
		}
		utils.SendInternalError(w)
		h.log.Error("UpdateProduct failed", zap.Error(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(prod)
}

func (h ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		h.log.Error("no ID provided in request")
		utils.SendJSONError(w, http.StatusBadRequest, "No ID provided in the request")
		return
	}

	h.log.Debug("deleting product", zap.String("id", idStr))

	prod, err := h.svc.DeleteProduct(r.Context(), idStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			utils.SendJSONError(w, http.StatusNotFound, "Record with given ID not found")
			return
		}
		h.log.Error("DeleteProductByID failed", zap.Error(err))
		utils.SendInternalError(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(prod)
}
