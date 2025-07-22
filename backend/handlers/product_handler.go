package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	utilErrs "github.com/avnpl/go-march/utils/errors"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/services"
	"go.uber.org/zap"
)

type ProductHandler struct {
	svc services.ProductService
	log *zap.Logger
}

func NewProductHandler(svc services.ProductService, log *zap.Logger) *ProductHandler {
	return &ProductHandler{svc: svc, log: log}
}

func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.log.Error(fmt.Errorf("error reading the request body : %w", err).Error(),
			zap.Error(err))
		http.Error(w, utilErrs.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	reqString := string(bodyBytes)
	h.log.Debug("raw body",
		zap.Int("length", len(bodyBytes)),
		zap.String("body", reqString),
	)
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var req models.CreateProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error(fmt.Errorf("invalid JSON : %w", err).Error())
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	prod, err := h.svc.CreateProduct(r.Context(), &req)
	if err != nil {
		if errors.Is(err, utilErrs.ErrConflict) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "Internal error", http.StatusInternalServerError)
		h.log.Error("CreateProduct failed", zap.Error(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(prod)
}

func (h *ProductHandler) FetchProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		h.log.Error(fmt.Errorf("no ID provided in request").Error())
		http.Error(w, utilErrs.ErrInvalidRequest.Error(), http.StatusBadRequest)
		return
	}

	h.log.Debug("received ID => ", zap.String("request param", idStr))

	id, err := strconv.ParseInt(idStr, 10, 64)
	prod, err := h.svc.GetProductByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, utilErrs.ErrConflict) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "Internal error", http.StatusInternalServerError)
		h.log.Error("GetProductByID failed", zap.Error(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(prod)
}

func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.log.Error(fmt.Errorf("error reading the request body : %w", err).Error(),
			zap.Error(err))
		http.Error(w, utilErrs.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	reqString := string(bodyBytes)
	h.log.Debug("raw body",
		zap.Int("length", len(bodyBytes)),
		zap.String("body", reqString),
	)
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var req models.UpdateProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error(fmt.Errorf("invalid JSON : %w", err).Error())
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	prod, err := h.svc.UpdateProduct(r.Context(), *req.ProductID, &req)
	if err != nil {
		if errors.Is(err, utilErrs.ErrConflict) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "Internal error", http.StatusInternalServerError)
		h.log.Error("UpdateProduct failed", zap.Error(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(prod)
}
