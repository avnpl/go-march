package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/repos"
	"github.com/avnpl/go-march/utils/customErrors"
)

// mockOrderRepo implements repos.OrderRepo using function fields so each
// test case can stub only the behaviour it cares about.
type mockOrderRepo struct {
	createFn    func(txn *sqlx.Tx, ctx context.Context, order models.Order) (models.Order, error)
	fetchByIDFn func(ctx context.Context, id string) (models.Order, error)
	fetchAllFn  func(ctx context.Context, limit int, offset int) ([]models.Order, error)
}

func (m mockOrderRepo) Create(txn *sqlx.Tx, ctx context.Context, order models.Order) (models.Order, error) {
	return m.createFn(txn, ctx, order)
}

func (m mockOrderRepo) FetchByID(ctx context.Context, id string) (models.Order, error) {
	return m.fetchByIDFn(ctx, id)
}

func (m mockOrderRepo) FetchAll(ctx context.Context, limit int, offset int) ([]models.Order, error) {
	return m.fetchAllFn(ctx, limit, offset)
}

func (m mockOrderRepo) Delete() {
	panic("unimplemented")
}

// orderTestProductRepo implements repos.ProductRepo using function fields, so
// each test case can stub only the methods it exercises.
type orderTestProductRepo struct {
	createFn           func(ctx context.Context, p *models.Product) (models.Product, error)
	fetchByIDFn        func(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error)
	fetchAllFn         func(ctx context.Context, limit int, offset int) ([]models.Product, error)
	updateByIDFn       func(ctx context.Context, p *models.UpdateProductReq) (models.Product, error)
	deleteByIDFn       func(ctx context.Context, id string) (models.Product, error)
	decrementStockFn   func(txn *sqlx.Tx, ctx context.Context, id string, qty int) (int, error)
	beginTransactionFn func() (*sqlx.Tx, error)
}

func (m orderTestProductRepo) Create(ctx context.Context, p *models.Product) (models.Product, error) {
	return m.createFn(ctx, p)
}

func (m orderTestProductRepo) FetchByID(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error) {
	return m.fetchByIDFn(txn, ctx, id)
}

func (m orderTestProductRepo) FetchAll(ctx context.Context, limit int, offset int) ([]models.Product, error) {
	return m.fetchAllFn(ctx, limit, offset)
}

func (m orderTestProductRepo) UpdateByID(ctx context.Context, p *models.UpdateProductReq) (models.Product, error) {
	return m.updateByIDFn(ctx, p)
}

func (m orderTestProductRepo) DeleteByID(ctx context.Context, id string) (models.Product, error) {
	return m.deleteByIDFn(ctx, id)
}

func (m orderTestProductRepo) DecrementStock(txn *sqlx.Tx, ctx context.Context, id string, qty int) (int, error) {
	return m.decrementStockFn(txn, ctx, id, qty)
}

func (m orderTestProductRepo) BeginTransaction() (*sqlx.Tx, error) {
	return m.beginTransactionFn()
}

// newTxn builds a real *sqlx.Tx backed by sqlmock so that the production
// code's direct calls to txn.Commit()/txn.Rollback() succeed against
// expectations set by the caller.
func newTxn(t *testing.T) (*sqlx.Tx, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mock.ExpectBegin()

	tx, err := sqlxDB.Beginx()
	if err != nil {
		t.Fatalf("failed to begin sqlmock tx: %v", err)
	}

	return tx, mock, func() { _ = db.Close() }
}

func TestOrderService_Create(t *testing.T) {
	validReq := models.CreateOrderReq{
		ProductID:       "PR-abc123",
		Quantity:        2,
		Amount:          20.0,
		ShippingAddress: "123 Main St",
		Notes:           "leave at door",
		CardNumber:      "1234",
	}

	product := models.Product{
		ProductID: "PR-abc123",
		Name:      "Widget",
		Price:     10.0,
		Stock:     5,
	}

	t.Run("happy path", func(t *testing.T) {
		tx, mock, closeDB := newTxn(t)
		defer closeDB()
		mock.ExpectCommit()

		orderRepo := mockOrderRepo{
			createFn: func(txn *sqlx.Tx, ctx context.Context, order models.Order) (models.Order, error) {
				order.OrderID = "OR-xyz"
				return order, nil
			},
		}
		productRepo := orderTestProductRepo{
			beginTransactionFn: func() (*sqlx.Tx, error) { return tx, nil },
			fetchByIDFn: func(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error) {
				return product, nil
			},
			decrementStockFn: func(txn *sqlx.Tx, ctx context.Context, id string, qty int) (int, error) {
				return product.Stock - qty, nil
			},
		}

		svc := NewOrderService(orderRepo, productRepo, zap.NewNop())
		res, err := svc.Create(context.Background(), validReq)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if res.OrderID != "OR-xyz" {
			t.Errorf("expected order id OR-xyz, got %q", res.OrderID)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("product not found", func(t *testing.T) {
		tx, mock, closeDB := newTxn(t)
		defer closeDB()
		mock.ExpectRollback()

		orderRepo := mockOrderRepo{}
		productRepo := orderTestProductRepo{
			beginTransactionFn: func() (*sqlx.Tx, error) { return tx, nil },
			fetchByIDFn: func(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error) {
				return models.Product{}, fmt.Errorf("product_repo.FetchByID: %w", sql.ErrNoRows)
			},
		}

		svc := NewOrderService(orderRepo, productRepo, zap.NewNop())
		_, err := svc.Create(context.Background(), validReq)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, customErrors.RecordNotFound) {
			t.Errorf("expected RecordNotFound, got: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("amount mismatch", func(t *testing.T) {
		tx, mock, closeDB := newTxn(t)
		defer closeDB()
		mock.ExpectRollback()

		req := validReq
		req.Amount = 999.99

		orderRepo := mockOrderRepo{}
		productRepo := orderTestProductRepo{
			beginTransactionFn: func() (*sqlx.Tx, error) { return tx, nil },
			fetchByIDFn: func(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error) {
				return product, nil
			},
		}

		svc := NewOrderService(orderRepo, productRepo, zap.NewNop())
		_, err := svc.Create(context.Background(), req)
		if !errors.Is(err, customErrors.IncorrectAmount) {
			t.Fatalf("expected IncorrectAmount error, got: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("card number 6969 fails transaction", func(t *testing.T) {
		tx, mock, closeDB := newTxn(t)
		defer closeDB()
		mock.ExpectRollback()

		req := validReq
		req.CardNumber = "6969"

		orderRepo := mockOrderRepo{}
		productRepo := orderTestProductRepo{
			beginTransactionFn: func() (*sqlx.Tx, error) { return tx, nil },
			fetchByIDFn: func(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error) {
				return product, nil
			},
		}

		svc := NewOrderService(orderRepo, productRepo, zap.NewNop())
		_, err := svc.Create(context.Background(), req)
		if !errors.Is(err, customErrors.FailedTransaction) {
			t.Fatalf("expected FailedTransaction error, got: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("out of stock", func(t *testing.T) {
		tx, mock, closeDB := newTxn(t)
		defer closeDB()
		mock.ExpectRollback()

		orderRepo := mockOrderRepo{}
		productRepo := orderTestProductRepo{
			beginTransactionFn: func() (*sqlx.Tx, error) { return tx, nil },
			fetchByIDFn: func(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error) {
				return product, nil
			},
			decrementStockFn: func(txn *sqlx.Tx, ctx context.Context, id string, qty int) (int, error) {
				return 0, customErrors.OutOfStock
			},
		}

		svc := NewOrderService(orderRepo, productRepo, zap.NewNop())
		_, err := svc.Create(context.Background(), validReq)
		if !errors.Is(err, customErrors.OutOfStock) {
			t.Fatalf("expected OutOfStock error, got: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("order create fails", func(t *testing.T) {
		tx, mock, closeDB := newTxn(t)
		defer closeDB()
		mock.ExpectRollback()

		wantErr := errors.New("insert failed")
		orderRepo := mockOrderRepo{
			createFn: func(txn *sqlx.Tx, ctx context.Context, order models.Order) (models.Order, error) {
				return models.Order{}, wantErr
			},
		}
		productRepo := orderTestProductRepo{
			beginTransactionFn: func() (*sqlx.Tx, error) { return tx, nil },
			fetchByIDFn: func(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error) {
				return product, nil
			},
			decrementStockFn: func(txn *sqlx.Tx, ctx context.Context, id string, qty int) (int, error) {
				return product.Stock - qty, nil
			},
		}

		svc := NewOrderService(orderRepo, productRepo, zap.NewNop())
		_, err := svc.Create(context.Background(), validReq)
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected wrapped %q, got: %v", wantErr, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("commit fails", func(t *testing.T) {
		tx, mock, closeDB := newTxn(t)
		defer closeDB()
		commitErr := errors.New("commit boom")
		mock.ExpectCommit().WillReturnError(commitErr)
		mock.ExpectRollback()

		orderRepo := mockOrderRepo{
			createFn: func(txn *sqlx.Tx, ctx context.Context, order models.Order) (models.Order, error) {
				order.OrderID = "OR-xyz"
				return order, nil
			},
		}
		productRepo := orderTestProductRepo{
			beginTransactionFn: func() (*sqlx.Tx, error) { return tx, nil },
			fetchByIDFn: func(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error) {
				return product, nil
			},
			decrementStockFn: func(txn *sqlx.Tx, ctx context.Context, id string, qty int) (int, error) {
				return product.Stock - qty, nil
			},
		}

		svc := NewOrderService(orderRepo, productRepo, zap.NewNop())
		_, err := svc.Create(context.Background(), validReq)
		if !errors.Is(err, commitErr) {
			t.Fatalf("expected wrapped commit error, got: %v", err)
		}
	})
}

func TestOrderService_FetchByID(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		want := models.Order{OrderID: "OR-1", ProductID: "PR-1"}
		orderRepo := mockOrderRepo{
			fetchByIDFn: func(ctx context.Context, id string) (models.Order, error) {
				return want, nil
			},
		}
		svc := NewOrderService(orderRepo, orderTestProductRepo{}, zap.NewNop())

		got, err := svc.FetchByID(context.Background(), "OR-1")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if got != want {
			t.Errorf("expected %+v, got %+v", want, got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		orderRepo := mockOrderRepo{
			fetchByIDFn: func(ctx context.Context, id string) (models.Order, error) {
				return models.Order{}, sql.ErrNoRows
			},
		}
		svc := NewOrderService(orderRepo, orderTestProductRepo{}, zap.NewNop())

		_, err := svc.FetchByID(context.Background(), "OR-missing")
		if !errors.Is(err, customErrors.RecordNotFound) {
			t.Fatalf("expected RecordNotFound, got: %v", err)
		}
	})

	t.Run("other repo error", func(t *testing.T) {
		wantErr := errors.New("db down")
		orderRepo := mockOrderRepo{
			fetchByIDFn: func(ctx context.Context, id string) (models.Order, error) {
				return models.Order{}, wantErr
			},
		}
		svc := NewOrderService(orderRepo, orderTestProductRepo{}, zap.NewNop())

		_, err := svc.FetchByID(context.Background(), "OR-1")
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected wrapped %q, got: %v", wantErr, err)
		}
	})
}

func TestOrderService_FetchAll(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		want := []models.Order{{OrderID: "OR-1"}, {OrderID: "OR-2"}}
		orderRepo := mockOrderRepo{
			fetchAllFn: func(ctx context.Context, limit int, offset int) ([]models.Order, error) {
				return want, nil
			},
		}
		svc := NewOrderService(orderRepo, orderTestProductRepo{}, zap.NewNop())

		got, err := svc.FetchAll(context.Background(), 5, 0)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(got) != len(want) {
			t.Errorf("expected %d orders, got %d", len(want), len(got))
		}
	})

	t.Run("zero limit uses default without error", func(t *testing.T) {
		var receivedLimit int
		orderRepo := mockOrderRepo{
			fetchAllFn: func(ctx context.Context, limit int, offset int) ([]models.Order, error) {
				receivedLimit = limit
				return nil, nil
			},
		}
		svc := NewOrderService(orderRepo, orderTestProductRepo{}, zap.NewNop())

		_, err := svc.FetchAll(context.Background(), 0, 0)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if receivedLimit <= 0 {
			t.Errorf("expected a positive default limit to be passed to repo, got %d", receivedLimit)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		wantErr := errors.New("query failed")
		orderRepo := mockOrderRepo{
			fetchAllFn: func(ctx context.Context, limit int, offset int) ([]models.Order, error) {
				return nil, wantErr
			},
		}
		svc := NewOrderService(orderRepo, orderTestProductRepo{}, zap.NewNop())

		_, err := svc.FetchAll(context.Background(), 5, 0)
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected wrapped %q, got: %v", wantErr, err)
		}
	})
}

var _ repos.OrderRepo = mockOrderRepo{}
var _ repos.ProductRepo = orderTestProductRepo{}
