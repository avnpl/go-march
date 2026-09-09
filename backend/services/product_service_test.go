package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/avnpl/go-march/models"
	"github.com/avnpl/go-march/utils/customErrors"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// mockProductRepo is a hand-rolled mock implementing repos.ProductRepo.
// Each test stubs only the func fields it needs.
type mockProductRepo struct {
	CreateFunc           func(ctx context.Context, p *models.Product) (models.Product, error)
	FetchByIDFunc        func(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error)
	FetchAllFunc         func(ctx context.Context, limit int, offset int) ([]models.Product, error)
	UpdateByIDFunc       func(ctx context.Context, p *models.UpdateProductReq) (models.Product, error)
	DeleteByIDFunc       func(ctx context.Context, id string) (models.Product, error)
	DecrementStockFunc   func(txn *sqlx.Tx, ctx context.Context, id string, qty int) (int, error)
	BeginTransactionFunc func() (*sqlx.Tx, error)
}

func (m *mockProductRepo) Create(ctx context.Context, p *models.Product) (models.Product, error) {
	return m.CreateFunc(ctx, p)
}

func (m *mockProductRepo) FetchByID(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error) {
	return m.FetchByIDFunc(txn, ctx, id)
}

func (m *mockProductRepo) FetchAll(ctx context.Context, limit int, offset int) ([]models.Product, error) {
	return m.FetchAllFunc(ctx, limit, offset)
}

func (m *mockProductRepo) UpdateByID(ctx context.Context, p *models.UpdateProductReq) (models.Product, error) {
	return m.UpdateByIDFunc(ctx, p)
}

func (m *mockProductRepo) DeleteByID(ctx context.Context, id string) (models.Product, error) {
	return m.DeleteByIDFunc(ctx, id)
}

func (m *mockProductRepo) DecrementStock(txn *sqlx.Tx, ctx context.Context, id string, qty int) (int, error) {
	if m.DecrementStockFunc == nil {
		return 0, nil
	}
	return m.DecrementStockFunc(txn, ctx, id, qty)
}

func (m *mockProductRepo) GetLowStockProducts(ctx context.Context, threshold int) ([]models.Product, error) {
	return nil, nil
}

func (m *mockProductRepo) BeginTransaction() (*sqlx.Tx, error) {
	if m.BeginTransactionFunc == nil {
		return nil, nil
	}
	return m.BeginTransactionFunc()
}

var errRepoBoom = errors.New("repo boom")

func TestProductService_CreateProduct(t *testing.T) {
	t.Parallel()

	t.Run("repo succeeds", func(t *testing.T) {
		t.Parallel()

		want := models.Product{ProductID: "PR-abc123", Name: "widget", Price: 9.99, Stock: 5}
		repo := &mockProductRepo{
			CreateFunc: func(ctx context.Context, p *models.Product) (models.Product, error) {
				return want, nil
			},
		}
		svc := NewProductService(repo, zap.NewNop())

		got, err := svc.CreateProduct(context.Background(), &models.CreateProductReq{Name: "widget", Price: 9.99, Stock: 5})
		if err != nil {
			t.Fatalf("CreateProduct() unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("CreateProduct() = %+v, want %+v", got, want)
		}
	})

	t.Run("repo errors", func(t *testing.T) {
		t.Parallel()

		repo := &mockProductRepo{
			CreateFunc: func(ctx context.Context, p *models.Product) (models.Product, error) {
				return models.Product{}, errRepoBoom
			},
		}
		svc := NewProductService(repo, zap.NewNop())

		_, err := svc.CreateProduct(context.Background(), &models.CreateProductReq{Name: "widget", Price: 9.99, Stock: 5})
		if err == nil {
			t.Fatal("CreateProduct() expected error, got nil")
		}
		if !errors.Is(err, errRepoBoom) {
			t.Errorf("CreateProduct() error = %v, want wrapped %v", err, errRepoBoom)
		}
	})
}

func TestProductService_GetProductByID(t *testing.T) {
	t.Parallel()

	t.Run("repo returns product", func(t *testing.T) {
		t.Parallel()

		want := models.Product{ProductID: "PR-abc123", Name: "widget"}
		repo := &mockProductRepo{
			FetchByIDFunc: func(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error) {
				return want, nil
			},
		}
		svc := NewProductService(repo, zap.NewNop())

		got, err := svc.GetProductByID(context.Background(), "PR-abc123")
		if err != nil {
			t.Fatalf("GetProductByID() unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("GetProductByID() = %+v, want %+v", got, want)
		}
	})

	t.Run("repo returns sql.ErrNoRows", func(t *testing.T) {
		t.Parallel()

		repo := &mockProductRepo{
			FetchByIDFunc: func(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error) {
				return models.Product{}, sql.ErrNoRows
			},
		}
		svc := NewProductService(repo, zap.NewNop())

		_, err := svc.GetProductByID(context.Background(), "PR-missing")
		if !errors.Is(err, customErrors.RecordNotFound) {
			t.Errorf("GetProductByID() error = %v, want %v", err, customErrors.RecordNotFound)
		}
	})

	t.Run("repo returns other error", func(t *testing.T) {
		t.Parallel()

		repo := &mockProductRepo{
			FetchByIDFunc: func(txn *sqlx.Tx, ctx context.Context, id string) (models.Product, error) {
				return models.Product{}, errRepoBoom
			},
		}
		svc := NewProductService(repo, zap.NewNop())

		_, err := svc.GetProductByID(context.Background(), "PR-abc123")
		if errors.Is(err, customErrors.RecordNotFound) {
			t.Errorf("GetProductByID() error = %v, should not be RecordNotFound", err)
		}
		if !errors.Is(err, errRepoBoom) {
			t.Errorf("GetProductByID() error = %v, want wrapped %v", err, errRepoBoom)
		}
	})
}

func TestProductService_GetAllProducts(t *testing.T) {
	t.Parallel()

	t.Run("repo succeeds", func(t *testing.T) {
		t.Parallel()

		want := []models.Product{{ProductID: "PR-1"}, {ProductID: "PR-2"}}
		repo := &mockProductRepo{
			FetchAllFunc: func(ctx context.Context, limit int, offset int) ([]models.Product, error) {
				return want, nil
			},
		}
		svc := NewProductService(repo, zap.NewNop())

		got, err := svc.GetAllProducts(context.Background(), 10, 0)
		if err != nil {
			t.Fatalf("GetAllProducts() unexpected error: %v", err)
		}
		if len(got) != len(want) {
			t.Errorf("GetAllProducts() = %+v, want %+v", got, want)
		}
	})

	t.Run("repo errors", func(t *testing.T) {
		t.Parallel()

		repo := &mockProductRepo{
			FetchAllFunc: func(ctx context.Context, limit int, offset int) ([]models.Product, error) {
				return nil, errRepoBoom
			},
		}
		svc := NewProductService(repo, zap.NewNop())

		_, err := svc.GetAllProducts(context.Background(), 10, 0)
		if !errors.Is(err, errRepoBoom) {
			t.Errorf("GetAllProducts() error = %v, want wrapped %v", err, errRepoBoom)
		}
	})
}

func TestProductService_UpdateProduct(t *testing.T) {
	t.Parallel()

	t.Run("repo returns updated product", func(t *testing.T) {
		t.Parallel()

		want := models.Product{ProductID: "PR-abc123", Name: "updated"}
		repo := &mockProductRepo{
			UpdateByIDFunc: func(ctx context.Context, p *models.UpdateProductReq) (models.Product, error) {
				return want, nil
			},
		}
		svc := NewProductService(repo, zap.NewNop())

		got, err := svc.UpdateProduct(context.Background(), &models.UpdateProductReq{ProductID: "PR-abc123", Name: "updated"})
		if err != nil {
			t.Fatalf("UpdateProduct() unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("UpdateProduct() = %+v, want %+v", got, want)
		}
	})

	t.Run("repo returns sql.ErrNoRows", func(t *testing.T) {
		t.Parallel()

		repo := &mockProductRepo{
			UpdateByIDFunc: func(ctx context.Context, p *models.UpdateProductReq) (models.Product, error) {
				return models.Product{}, sql.ErrNoRows
			},
		}
		svc := NewProductService(repo, zap.NewNop())

		_, err := svc.UpdateProduct(context.Background(), &models.UpdateProductReq{ProductID: "PR-missing"})
		if !errors.Is(err, customErrors.RecordNotFound) {
			t.Errorf("UpdateProduct() error = %v, want %v", err, customErrors.RecordNotFound)
		}
	})

	t.Run("repo returns other error", func(t *testing.T) {
		t.Parallel()

		repo := &mockProductRepo{
			UpdateByIDFunc: func(ctx context.Context, p *models.UpdateProductReq) (models.Product, error) {
				return models.Product{}, errRepoBoom
			},
		}
		svc := NewProductService(repo, zap.NewNop())

		_, err := svc.UpdateProduct(context.Background(), &models.UpdateProductReq{ProductID: "PR-abc123"})
		if errors.Is(err, customErrors.RecordNotFound) {
			t.Errorf("UpdateProduct() error = %v, should not be RecordNotFound", err)
		}
		if !errors.Is(err, errRepoBoom) {
			t.Errorf("UpdateProduct() error = %v, want wrapped %v", err, errRepoBoom)
		}
	})
}

func TestProductService_DeleteProduct(t *testing.T) {
	t.Parallel()

	t.Run("repo returns deleted product", func(t *testing.T) {
		t.Parallel()

		want := models.Product{ProductID: "PR-abc123"}
		repo := &mockProductRepo{
			DeleteByIDFunc: func(ctx context.Context, id string) (models.Product, error) {
				return want, nil
			},
		}
		svc := NewProductService(repo, zap.NewNop())

		got, err := svc.DeleteProduct(context.Background(), "PR-abc123")
		if err != nil {
			t.Fatalf("DeleteProduct() unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("DeleteProduct() = %+v, want %+v", got, want)
		}
	})

	t.Run("repo returns sql.ErrNoRows", func(t *testing.T) {
		t.Parallel()

		repo := &mockProductRepo{
			DeleteByIDFunc: func(ctx context.Context, id string) (models.Product, error) {
				return models.Product{}, sql.ErrNoRows
			},
		}
		svc := NewProductService(repo, zap.NewNop())

		_, err := svc.DeleteProduct(context.Background(), "PR-missing")
		if !errors.Is(err, customErrors.RecordNotFound) {
			t.Errorf("DeleteProduct() error = %v, want %v", err, customErrors.RecordNotFound)
		}
	})

	t.Run("repo returns other error", func(t *testing.T) {
		t.Parallel()

		repo := &mockProductRepo{
			DeleteByIDFunc: func(ctx context.Context, id string) (models.Product, error) {
				return models.Product{}, errRepoBoom
			},
		}
		svc := NewProductService(repo, zap.NewNop())

		_, err := svc.DeleteProduct(context.Background(), "PR-abc123")
		if errors.Is(err, customErrors.RecordNotFound) {
			t.Errorf("DeleteProduct() error = %v, should not be RecordNotFound", err)
		}
		if !errors.Is(err, errRepoBoom) {
			t.Errorf("DeleteProduct() error = %v, want wrapped %v", err, errRepoBoom)
		}
	})
}
