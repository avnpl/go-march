## Project Context

**go-march** is a learning-focused Go backend for someone transitioning from Spring Boot / Express.js. The primary goal is to demonstrate 5 API architectures (REST, GraphQL, SOAP, gRPC, WebSocket) within a single application, all sharing the same service and repository layers.

**Design principles:**
- Minimal external dependencies — focus on core Go concepts, not framework magic
- No ORM — raw SQL via `sqlx` to keep database access explicit
- Clean layered architecture: handlers → services → repositories
- Each API style serves a distinct purpose (see below)

---

## Architecture & Tech Stack

- **Language:** Go 1.23+
- **Database:** CockroachDB (PostgreSQL-compatible) via `jackc/pgx/v5` + `jmoiron/sqlx`
- **Logging:** `go.uber.org/zap`
- **GraphQL:** `github.com/graphql-go/graphql`
- **Routing:** Standard library `http.ServeMux` (Go 1.22+ method+path patterns)
- **Entry point:** `main.go` — wires all layers, registers routes, starts HTTP server on `:8080`

---

## Project Structure

```
├── main.go
├── api/
│   ├── rest/            # HTTP handlers (ProductHandler)
│   ├── graphql/         # schema.go, types.go, queries.go, mutations.go, resolvers.go
│   ├── grpc/            # grpc.go (stub package)
│   └── soap/            # soap.go (stub package)
├── services/            # Business logic — ProductService (OrderService not wired yet)
├── repos/               # DB access — ProductRepo (order/payment repos not present)
├── models/              # models.go — Product, Orders (struct only), request structs
├── utils/               # utils.go, errors.go; validations.go (placeholder)
└── agent-reviews/       # Code review notes (date-named markdown files)
```

**Note:** There is no `handlers/` directory — handlers live under `api/`.

---

## API Styles & Purpose

| API | Purpose | Status |
|-----|---------|--------|
| REST | Product CRUD (+ future orders/payments) | 🔶 Phase 1 in progress — product endpoints work; paths differ from roadmap (`/product` vs `/products/{id}`); DELETE returns 200 with body (user preference); string `PR-…` IDs on create/fetch; Update/Delete paths still use `int64` (see `ROADMAP.md`, `TODO(id-migration)` in code) |
| GraphQL | Product queries + mutations | 🔶 Partial — `getProductByID`, `getAllProducts`; `updateProduct`, `deleteProduct`; no `createProduct`; no order API (Phase 2) |
| SOAP | Transactional order placement | 🚧 Stub package only |
| gRPC | Analytics procedures | 🚧 Stub package only |
| WebSocket | Real-time notifications | 🚧 Not implemented |

For the full implementation roadmap see [`ROADMAP.md`](./ROADMAP.md).

---

## Key Files

| File | Purpose |
|------|---------|
| `main.go` | Entry point, dependency wiring, route registration, graceful shutdown |
| `models/models.go` | `Product`, `Orders`, `CreateProductReq`, `UpdateProductReq` |
| `repos/product_repo.go` | `ProductRepo` interface + `pgProductRepo` implementation |
| `services/product_service.go` | `ProductService` interface + implementation |
| `api/rest/product_handler.go` | REST handlers: Create, Fetch, FetchAll, Update, Delete |
| `api/graphql/schema.go` | GraphQL schema init (`NewSchema`, `Schema`) |
| `api/graphql/types.go` | `ProductType`, `UpdateProductInput` |
| `api/graphql/queries.go` | `getProductByID`, `getAllProducts` |
| `api/graphql/mutations.go` | `updateProduct`, `deleteProduct` |
| `api/graphql/resolvers.go` | `Resolver` struct with resolver methods |
| `utils/utils.go` | `BuildLogger`, `GetDBPoolObject`, `SendJSONError`, `SendInternalError` |
| `utils/errors.go` | Sentinel errors: `ErrConflict`, `ErrInternal`, `ErrInvalidRequest`, `ErrRecordNotFound` |

---

## Code Conventions

### Naming

- **Files:** `snake_case.go`
- **Types / Interfaces:** `PascalCase`
- **Functions / Variables:** `camelCase`
- **Packages:** short, lowercase, no underscores (`utils`, `repos`, `services`)

### Import Order

```go
import (
    // 1. Standard library
    "context"
    "net/http"

    // 2. Third-party
    "github.com/jmoiron/sqlx"
    "go.uber.org/zap"

    // 3. Internal
    "github.com/avnpl/go-march/models"
    "github.com/avnpl/go-march/repos"
)
```

### Error Handling

- Wrap errors with context: `fmt.Errorf("productRepo.UpdateByID: %w", err)`
- Sentinel errors in `utils/errors.go` — use `errors.Is()` to match
- Handle errors at the handler layer; log with `zap.Error(err)`
- Never expose internal error details to API consumers

```go
// Correct
logger.Error("failed to fetch product", zap.Error(err), zap.String("id", id))

// Wrong — loses error chain, redundant
logger.Error(fmt.Errorf("error: %w", err).Error(), zap.Error(err))
```

### Context

- Always pass `context.Context` as the **first parameter**, by value — never as a pointer
- Propagate `r.Context()` from HTTP handlers through service and repo calls

### Types

- Use interfaces for the service layer to allow testing
- Use concrete types (`pgProductRepo`) for repos
- Use `time.Time` for timestamps — not `string`

```go
type ProductService interface {
    CreateProduct(ctx context.Context, req *models.CreateProductReq) (models.Product, error)
    GetProductByID(ctx context.Context, id string) (models.Product, error)
    DeleteProduct(ctx context.Context, id int64) (models.Product, error) // migrate id to string — see TODO(id-migration)
}
```

### Database

- Parameterized queries only — no string concatenation in SQL
- Use lowercase SQL keywords consistently
- Configure the connection pool (`SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime`)

### REST Handler Pattern

```go
func (h ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
    // 1. Decode request body — always check the error
    // 2. Validate input
    // 3. Call service layer
    // 4. Map errors to HTTP status codes (404, 409, 500...)
    // 5. Write JSON response with correct status (201 for POST, 200 for GET/PATCH, 204 for DELETE)
}
```

### GraphQL Pattern

- Types → `types.go`
- Queries → `queries.go`
- Mutations → `mutations.go`
- Resolvers → `resolvers.go`
- Schema init → `schema.go`
- Use `graphql.NewNonNull()` for required fields
- Type-assert args explicitly: `p.Args["id"].(string)`

### Logging

```go
logger.Info("product created", zap.String("id", product.ProductID))
logger.Error("failed to fetch product", zap.Error(err), zap.String("id", id))
```

Never log request bodies — they may contain PII or secrets.

---

## Build & Run

```bash
# Run
go run main.go

# Build
go build -o bin/server main.go

# Test
go test ./...
go test -v -run TestName ./path/to/package
go test -cover ./...

# Lint & format
golangci-lint run
go fmt ./...
go vet ./...
```

### Environment

Create `.env` in this directory (variable names must match `utils.GetDBPoolObject`):
```
DB_URL=postgresql://user:pass@host:26257/dbname?sslmode=disable
LOG_LEVEL=debug
```

Load `.env` once at startup in `main()` — not inside utility functions called repeatedly.

---

## Known Technical Debt

See [`ROADMAP.md`](./ROADMAP.md) Phase 0–1 and [`agent-reviews/2026-03-12.md`](./agent-reviews/2026-03-12.md) for detail. Summary (search `TODO` in code for exact locations):

1. **ID migration (blocking):** Create and `GetProductByID` use string `PR-…` IDs; `UpdateProduct`, `DeleteProduct`, and GraphQL update/delete still use `int64` — breaks update/delete for string IDs until aligned end-to-end.
2. **REST paths:** Target `/products` and `/products/{id}`; current routes use `/product`, `/products`, `/product/{id}`.
3. **Validation:** `go-playground/validator` is wired; `validate:"required"` on numeric fields rejects zero-values — adjust tags (`gt=0`, `min=0`) per ROADMAP.
4. **SQL style:** `Create` and `UpdateByID` in `product_repo.go` still use uppercase keywords; other queries are lowercase.
5. **Logging:** Request bodies logged at debug (REST + `/graphql`) — remove or gate for production.
6. **Misc:** `GenerateID` suffix length vs ROADMAP (6 vs 7 chars); `Orders.Amount` field name vs `TotalPrice`; GraphQL: no `createProduct`; no order queries (Phase 2).
7. No database migration files in-repo.
8. **Resolved (do not re-report):** `time.Time` on models; `.env` once in `main()`; DB pool configured; error logging uses static message + `zap.Error(err)`; `errors.Is` for sentinels.

---

## Testing

- Test files: `*_test.go` in the same package
- Table-driven tests preferred
- Mock service interfaces for handler tests
- Naming: `TestProductService_CreateProduct`
