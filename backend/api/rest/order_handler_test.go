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

// mockOrderService is a hand-rolled mock implementing services.OrderService.
type mockOrderService struct {
	createFn    func(ctx context.Context, req models.CreateOrderReq) (models.Order, error)
	fetchByIDFn func(ctx context.Context, id string) (models.Order, error)
	fetchAllFn  func(ctx context.Context, limit int, offset int) ([]models.Order, error)
}

func (m *mockOrderService) Create(ctx context.Context, req models.CreateOrderReq) (models.Order, error) {
	return m.createFn(ctx, req)
}

func (m *mockOrderService) FetchByID(ctx context.Context, id string) (models.Order, error) {
	return m.fetchByIDFn(ctx, id)
}

func (m *mockOrderService) FetchAll(ctx context.Context, limit int, offset int) ([]models.Order, error) {
	return m.fetchAllFn(ctx, limit, offset)
}

func (m *mockOrderService) Delete() {}

func newOrderTestMux(svc *mockOrderService) *http.ServeMux {
	h := NewOrderHandler(svc, zap.NewNop(), validator.New())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux
}

var errOrderBoom = errors.New("order boom")

func validCreateOrderBody() []byte {
	body, _ := json.Marshal(map[string]interface{}{
		"prod_id":         "PR-abc123",
		"quantity":        2,
		"amount":          19.98,
		"shippingAddress": "123 Main St",
		"card_num":        "1234",
	})
	return body
}

func TestOrderHandler_CreateOrder(t *testing.T) {
	t.Parallel()

	t.Run("valid body creates order", func(t *testing.T) {
		t.Parallel()

		want := models.Order{OrderID: "OR-abc123", ProductID: "PR-abc123", Quantity: 2, Amount: 19.98}
		svc := &mockOrderService{
			createFn: func(ctx context.Context, req models.CreateOrderReq) (models.Order, error) {
				return want, nil
			},
		}
		mux := newOrderTestMux(svc)

		req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(validCreateOrderBody()))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
		}
		var got models.Order
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got != want {
			t.Errorf("response = %+v, want %+v", got, want)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		t.Parallel()

		svc := &mockOrderService{}
		mux := newOrderTestMux(svc)

		req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader([]byte("{invalid")))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})

	t.Run("validation failure returns 400", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			body map[string]interface{}
		}{
			{
				name: "missing shippingAddress",
				body: map[string]interface{}{
					"prod_id":  "PR-abc123",
					"quantity": 2,
					"amount":   19.98,
					"card_num": "1234",
				},
			},
			{
				name: "card_num not 4 digits",
				body: map[string]interface{}{
					"prod_id":         "PR-abc123",
					"quantity":        2,
					"amount":          19.98,
					"shippingAddress": "123 Main St",
					"card_num":        "12345",
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				svc := &mockOrderService{}
				mux := newOrderTestMux(svc)

				bodyBytes, _ := json.Marshal(tc.body)
				req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(bodyBytes))
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req)

				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
				}
			})
		}
	})

	t.Run("service errors map to correct status", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			svcErr     error
			wantStatus int
		}{
			{"out of stock", customErrors.OutOfStock, http.StatusBadRequest},
			{"incorrect amount", customErrors.IncorrectAmount, http.StatusBadRequest},
			{"failed transaction", customErrors.FailedTransaction, http.StatusBadRequest},
			{"generic error", errOrderBoom, http.StatusInternalServerError},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				svc := &mockOrderService{
					createFn: func(ctx context.Context, req models.CreateOrderReq) (models.Order, error) {
						return models.Order{}, tc.svcErr
					},
				}
				mux := newOrderTestMux(svc)

				req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(validCreateOrderBody()))
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req)

				if rec.Code != tc.wantStatus {
					t.Fatalf("status = %d, want %d, body=%s", rec.Code, tc.wantStatus, rec.Body.String())
				}
			})
		}
	})
}

func TestOrderHandler_FetchOrderByID(t *testing.T) {
	t.Parallel()

	t.Run("found returns 200", func(t *testing.T) {
		t.Parallel()

		want := models.Order{OrderID: "OR-abc123", ProductID: "PR-abc123"}
		svc := &mockOrderService{
			fetchByIDFn: func(ctx context.Context, id string) (models.Order, error) {
				return want, nil
			},
		}
		mux := newOrderTestMux(svc)

		req := httptest.NewRequest(http.MethodGet, "/orders/OR-abc123", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		var got models.Order
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got != want {
			t.Errorf("response = %+v, want %+v", got, want)
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		t.Parallel()

		svc := &mockOrderService{
			fetchByIDFn: func(ctx context.Context, id string) (models.Order, error) {
				return models.Order{}, customErrors.RecordNotFound
			},
		}
		mux := newOrderTestMux(svc)

		req := httptest.NewRequest(http.MethodGet, "/orders/OR-missing", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
		}
	})

	t.Run("generic error returns 500", func(t *testing.T) {
		t.Parallel()

		svc := &mockOrderService{
			fetchByIDFn: func(ctx context.Context, id string) (models.Order, error) {
				return models.Order{}, errOrderBoom
			},
		}
		mux := newOrderTestMux(svc)

		req := httptest.NewRequest(http.MethodGet, "/orders/OR-abc123", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
		}
	})
}

func TestOrderHandler_FetchAllOrders(t *testing.T) {
	t.Parallel()

	t.Run("success returns 200", func(t *testing.T) {
		t.Parallel()

		want := []models.Order{{OrderID: "OR-1"}, {OrderID: "OR-2"}}
		svc := &mockOrderService{
			fetchAllFn: func(ctx context.Context, limit int, offset int) ([]models.Order, error) {
				return want, nil
			},
		}
		mux := newOrderTestMux(svc)

		req := httptest.NewRequest(http.MethodGet, "/orders", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		var got []models.Order
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(got) != len(want) {
			t.Errorf("response = %+v, want %+v", got, want)
		}
	})

	t.Run("invalid limit returns 400", func(t *testing.T) {
		t.Parallel()

		svc := &mockOrderService{}
		mux := newOrderTestMux(svc)

		req := httptest.NewRequest(http.MethodGet, "/orders?limit=abc", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})

	t.Run("invalid offset returns 400", func(t *testing.T) {
		t.Parallel()

		svc := &mockOrderService{}
		mux := newOrderTestMux(svc)

		req := httptest.NewRequest(http.MethodGet, "/orders?offset=abc", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})

	t.Run("service error returns 500 via SendInternalError", func(t *testing.T) {
		t.Parallel()

		svc := &mockOrderService{
			fetchAllFn: func(ctx context.Context, limit int, offset int) ([]models.Order, error) {
				return nil, errOrderBoom
			},
		}
		mux := newOrderTestMux(svc)

		req := httptest.NewRequest(http.MethodGet, "/orders", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
		}
	})
}
