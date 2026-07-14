package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/utils/customErrors"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

// mockProductService is a hand-rolled mock implementing services.ProductService.
type mockProductService struct {
	createProductFn  func(ctx context.Context, req *models.CreateProductReq) (models.Product, error)
	getProductByIDFn func(ctx context.Context, id string) (models.Product, error)
	getAllProductsFn func(ctx context.Context, limit int, offset int) ([]models.Product, error)
	updateProductFn  func(ctx context.Context, req *models.UpdateProductReq) (models.Product, error)
	deleteProductFn  func(ctx context.Context, id string) (models.Product, error)
}

func (m *mockProductService) CreateProduct(ctx context.Context, req *models.CreateProductReq) (models.Product, error) {
	return m.createProductFn(ctx, req)
}

func (m *mockProductService) GetProductByID(ctx context.Context, id string) (models.Product, error) {
	return m.getProductByIDFn(ctx, id)
}

func (m *mockProductService) GetAllProducts(ctx context.Context, limit int, offset int) ([]models.Product, error) {
	return m.getAllProductsFn(ctx, limit, offset)
}

func (m *mockProductService) UpdateProduct(ctx context.Context, req *models.UpdateProductReq) (models.Product, error) {
	return m.updateProductFn(ctx, req)
}

func (m *mockProductService) DeleteProduct(ctx context.Context, id string) (models.Product, error) {
	return m.deleteProductFn(ctx, id)
}

func newTestHandler(svc *mockProductService) ProductHandler {
	return NewProductHandler(svc, zap.NewNop(), validator.New())
}

func newTestMux(h ProductHandler) *http.ServeMux {
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux
}

var errGeneric = errors.New("boom")

func TestProductHandler_CreateProduct(t *testing.T) {
	validProd := models.Product{ProductID: "PR-1234567", Name: "Widget", Price: 10.5, Stock: 3}

	tests := []struct {
		name       string
		body       string
		svc        *mockProductService
		wantStatus int
	}{
		{
			name: "valid body creates product",
			body: `{"name":"Widget","price":10.5,"stock":3}`,
			svc: &mockProductService{
				createProductFn: func(ctx context.Context, req *models.CreateProductReq) (models.Product, error) {
					return validProd, nil
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid JSON body",
			body:       `{"name":`,
			svc:        &mockProductService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "validation failure missing name",
			body:       `{"price":10.5,"stock":3}`,
			svc:        &mockProductService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "service returns conflict",
			body: `{"name":"Widget","price":10.5,"stock":3}`,
			svc: &mockProductService{
				createProductFn: func(ctx context.Context, req *models.CreateProductReq) (models.Product, error) {
					return models.Product{}, customErrors.Conflict
				},
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "service returns generic error",
			body: `{"name":"Widget","price":10.5,"stock":3}`,
			svc: &mockProductService{
				createProductFn: func(ctx context.Context, req *models.CreateProductReq) (models.Product, error) {
					return models.Product{}, errGeneric
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := newTestMux(newTestHandler(tc.svc))
			req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(tc.body))
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.wantStatus, rec.Body.String())
			}

			if tc.wantStatus == http.StatusCreated {
				var got models.Product
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("failed to decode response body: %v", err)
				}
				if got.ProductID != validProd.ProductID {
					t.Errorf("ProductID = %q, want %q", got.ProductID, validProd.ProductID)
				}
			}
		})
	}
}

func TestProductHandler_FetchProduct(t *testing.T) {
	found := models.Product{ProductID: "PR-1234567", Name: "Widget", Price: 10.5, Stock: 3}

	tests := []struct {
		name       string
		svc        *mockProductService
		wantStatus int
	}{
		{
			name: "found returns product",
			svc: &mockProductService{
				getProductByIDFn: func(ctx context.Context, id string) (models.Product, error) {
					return found, nil
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "record not found",
			svc: &mockProductService{
				getProductByIDFn: func(ctx context.Context, id string) (models.Product, error) {
					return models.Product{}, customErrors.RecordNotFound
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "generic error",
			svc: &mockProductService{
				getProductByIDFn: func(ctx context.Context, id string) (models.Product, error) {
					return models.Product{}, errGeneric
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := newTestMux(newTestHandler(tc.svc))
			req := httptest.NewRequest(http.MethodGet, "/products/PR-1234567", nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.wantStatus, rec.Body.String())
			}

			if tc.wantStatus == http.StatusOK {
				var got models.Product
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("failed to decode response body: %v", err)
				}
				if got.ProductID != found.ProductID {
					t.Errorf("ProductID = %q, want %q", got.ProductID, found.ProductID)
				}
			}
		})
	}
}

func TestProductHandler_FetchAllProducts(t *testing.T) {
	prods := []models.Product{
		{ProductID: "PR-1111111", Name: "Widget", Price: 10.5, Stock: 3},
		{ProductID: "PR-2222222", Name: "Gadget", Price: 5.0, Stock: 1},
	}

	tests := []struct {
		name       string
		query      string
		svc        *mockProductService
		wantStatus int
	}{
		{
			name:  "success returns list",
			query: "",
			svc: &mockProductService{
				getAllProductsFn: func(ctx context.Context, limit int, offset int) ([]models.Product, error) {
					return prods, nil
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid limit param",
			query:      "?limit=abc",
			svc:        &mockProductService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid offset param",
			query:      "?offset=xyz",
			svc:        &mockProductService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "service error",
			query: "",
			svc: &mockProductService{
				getAllProductsFn: func(ctx context.Context, limit int, offset int) ([]models.Product, error) {
					return nil, errGeneric
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := newTestMux(newTestHandler(tc.svc))
			req := httptest.NewRequest(http.MethodGet, "/products"+tc.query, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.wantStatus, rec.Body.String())
			}

			if tc.wantStatus == http.StatusOK {
				var got []models.Product
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("failed to decode response body: %v", err)
				}
				if len(got) != len(prods) {
					t.Errorf("len(got) = %d, want %d", len(got), len(prods))
				}
			}
		})
	}
}

func TestProductHandler_UpdateProduct(t *testing.T) {
	updated := models.Product{ProductID: "PR-1234567", Name: "Widget", Price: 20, Stock: 5}

	tests := []struct {
		name       string
		body       string
		svc        *mockProductService
		wantStatus int
	}{
		{
			name: "valid body updates product",
			body: `{"price":20,"stock":5}`,
			svc: &mockProductService{
				updateProductFn: func(ctx context.Context, req *models.UpdateProductReq) (models.Product, error) {
					return updated, nil
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid JSON body",
			body:       `{"price":`,
			svc:        &mockProductService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "validation failure negative price",
			body:       `{"price":-1}`,
			svc:        &mockProductService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "record not found",
			body: `{"price":20}`,
			svc: &mockProductService{
				updateProductFn: func(ctx context.Context, req *models.UpdateProductReq) (models.Product, error) {
					return models.Product{}, customErrors.RecordNotFound
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "generic error",
			body: `{"price":20}`,
			svc: &mockProductService{
				updateProductFn: func(ctx context.Context, req *models.UpdateProductReq) (models.Product, error) {
					return models.Product{}, errGeneric
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := newTestMux(newTestHandler(tc.svc))
			req := httptest.NewRequest(http.MethodPatch, "/products/PR-1234567", bytes.NewBufferString(tc.body))
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.wantStatus, rec.Body.String())
			}

			if tc.wantStatus == http.StatusOK {
				var got models.Product
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("failed to decode response body: %v", err)
				}
				if got.ProductID != updated.ProductID {
					t.Errorf("ProductID = %q, want %q", got.ProductID, updated.ProductID)
				}
			}
		})
	}
}

func TestProductHandler_DeleteProduct(t *testing.T) {
	deleted := models.Product{ProductID: "PR-1234567", Name: "Widget", Price: 10.5, Stock: 3}

	tests := []struct {
		name       string
		svc        *mockProductService
		wantStatus int
	}{
		{
			name: "found deletes product",
			svc: &mockProductService{
				deleteProductFn: func(ctx context.Context, id string) (models.Product, error) {
					return deleted, nil
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "record not found",
			svc: &mockProductService{
				deleteProductFn: func(ctx context.Context, id string) (models.Product, error) {
					return models.Product{}, customErrors.RecordNotFound
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "generic error",
			svc: &mockProductService{
				deleteProductFn: func(ctx context.Context, id string) (models.Product, error) {
					return models.Product{}, errGeneric
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := newTestMux(newTestHandler(tc.svc))
			req := httptest.NewRequest(http.MethodDelete, "/products/PR-1234567", nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.wantStatus, rec.Body.String())
			}

			if tc.wantStatus == http.StatusOK {
				var got models.Product
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("failed to decode response body: %v", err)
				}
				if got.ProductID != deleted.ProductID {
					t.Errorf("ProductID = %q, want %q", got.ProductID, deleted.ProductID)
				}
			}
		})
	}
}
