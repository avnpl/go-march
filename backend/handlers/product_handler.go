package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/services"

	"go.uber.org/zap"
)

// ProductHandler holds our dependencies.
type ProductHandler struct {
	svc services.ProductService
	log *zap.Logger
}

func NewProductHandler(svc services.ProductService, log *zap.Logger) *ProductHandler {
	return &ProductHandler{svc: svc, log: log}
}

// CreateProduct implements POST slash products
func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.CreateProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		fmt.Println(req)
		fmt.Println(err)
		return
	}
	prod, err := h.svc.CreateProduct(r.Context(), &req)
	if err != nil {
		if errors.Is(err, fmt.Errorf("ErrConflict")) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		h.log.Error("CreateProduct failed", zap.Error(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(prod)
}
