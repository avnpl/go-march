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
- **Entry point:** `main.go` — wires all layers, registers routes, starts HTTP server on port `8013` by default (configurable via `PORT` env var)

---

## Project Structure

```
├── main.go
├── api/
│   ├── rest/            # HTTP handlers (ProductHandler, OrderHandler)
│   ├── graphql/         # schema.go, types.go, queries.go, mutations.go, resolvers.go
│   ├── grpc/            # empty — Phase 3, not started
│   └── admin/           # empty — unused
├── services/            # Business logic — ProductService, OrderService
├── repos/               # DB access — ProductRepo, OrderRepo (no PaymentRepo yet)
├── models/              # models.go — Product, Order, request structs
├── migrations/          # *.up.sql — products, orders, payments, TTL (no runner wired in yet)
├── utils/
│   ├── utils.go         # BuildLogger, GetDBPoolObject, SendJSONError, GenerateID, env helpers
│   ├── middleware.go     # RequestIDMiddleware, LimitBodySize
│   ├── context.go        # request ID get/set on context.Context
│   ├── validations.go    # FormatValidationErrors
│   ├── customErrors/     # errors.go — sentinel errors + HTTPFor() status mapping
│   └── log/               # logging.go — context-aware zap helpers (Info/Error/Debug/Warn)
└── verified-audit-report.md   # latest full code audit — see Known Technical Debt below
```

**Note:** SOAP was dropped from the project plan (see `ROADMAP.md`) — `api/soap/` is a leftover empty directory.

**Note:** There is no `handlers/` directory — handlers live under `api/`.

---

## API Styles & Purpose

| API | Purpose | Status |
|-----|---------|--------|
| REST | Product + Order CRUD | ✅ Phase 1 complete — routes match `ROADMAP.md` (`/products`, `/products/{id}`, `/orders`, `/orders/{id}`); string IDs end-to-end; DELETE returns 200 with body (user preference, not 204) |
| GraphQL | Product queries + mutations | 🔶 Partial — `getProductByID`, `getAllProducts`; `updateProduct`, `deleteProduct`; no `createProduct`; no order API (Phase 2, not started) |
| gRPC | Analytics procedures | 🚧 Not started — `api/grpc/` is an empty directory (Phase 3) |
| WebSocket | Real-time notifications | 🚧 Not implemented (Phase 4) |

> SOAP was dropped from the project plan — see `ROADMAP.md`.

For the full implementation roadmap see [`ROADMAP.md`](./ROADMAP.md).

---

## Key Files

| File | Purpose |
|------|---------|
| `main.go` | Entry point, dependency wiring, route registration, graceful shutdown |
| `models/models.go` | `Product`, `Order`, `CreateProductReq`, `UpdateProductReq`, `CreateOrderReq` |
| `repos/product_repo.go` | `ProductRepo` interface + `productRepo` implementation |
| `repos/order_repo.go` | `OrderRepo` interface + `orderRepo` implementation |
| `services/product_service.go` | `ProductService` interface + implementation |
| `services/order_service.go` | `OrderService` interface + implementation (transactional `Create`) |
| `api/rest/product_handler.go` | REST handlers: Create, Fetch, FetchAll, Update, Delete |
| `api/rest/order_handler.go` | REST handlers: Create, FetchByID, FetchAll (orders) |
| `api/rest/helper.go` | `SendErrorResponse` — maps sentinel errors to HTTP status via `customErrors.HTTPFor` |
| `api/graphql/schema.go` | GraphQL schema init (`NewSchema`, `Schema`) |
| `api/graphql/types.go` | `ProductType`, `UpdateProductInput`, `DeleteProductInput` |
| `api/graphql/queries.go` | `getProductByID`, `getAllProducts` |
| `api/graphql/mutations.go` | `updateProduct`, `deleteProduct` |
| `api/graphql/resolvers.go` | `Resolver` struct with resolver methods |
| `utils/utils.go` | `BuildLogger`, `GetDBPoolObject`, `SendJSONError`, `SendInternalError`, `GenerateID` |
| `utils/customErrors/errors.go` | Sentinel errors: `InvalidRequest`, `InvalidHTTPMethod`, `RecordNotFound`, `OutOfStock`, `IncorrectAmount`, `Conflict`, `FailedTransaction`; `HTTPFor(err)` status mapping |

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
- Sentinel errors in `utils/customErrors/errors.go` — use `errors.Is()` to match
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
    DeleteProduct(ctx context.Context, id string) (models.Product, error)
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

**Service layer policy**:
- Log mutations (create/update/delete) at Info level with entity ID
- Log errors at Error level with relevant context (IDs, input values)
- Don't log read operations (get/list) unless they fail
- Use `trace.Info()`, `trace.Error()` from `utils/trace` for context propagation

This is a toy project — no real user data flows through it, so raw request bodies (including `product_handler.go`/`order_handler.go` debug logs) are logged freely for local debugging. No PII/secrets policy needed.

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

See [`ROADMAP.md`](./ROADMAP.md) and [`verified-audit-report.md`](./verified-audit-report.md) for full detail (audit dated 2026-05-19 @ `661c925` — check current code before assuming a listed finding is still open; several have since been fixed, e.g. `OrderService.Create` now maps a missing product to 404).

Open items:
1. **GraphQL:** no `createProduct` mutation; no order queries yet (Phase 2, not started).
2. **No migration runner:** `migrations/*.up.sql` are plain reference files, not applied by any tool; no `.down.sql` files exist.
3. See `verified-audit-report.md` for the broader list of open findings (performance, maintainability) not tracked individually here — the audit's request-body-logging finding no longer applies (see above: toy project, no real data).

**Resolved (do not re-report):** `time.Time` on models; `.env` loaded once in `main()`; DB pool configured; error logging uses static message + `zap.Error(err)`; `errors.Is` for sentinels; string IDs end-to-end for Product and Order (REST + GraphQL); REST paths match `/products`, `/products/{id}`, `/orders`, `/orders/{id}`; SQL keywords lowercase in `product_repo.go`; `GenerateID` uses a 7-char suffix; `migrations/002_create_orders.up.sql` synced with the live `orders` schema; `OrderService.Create` maps a missing product to `customErrors.RecordNotFound` (404).

---

## Testing

- Test files: `*_test.go` in the same package
- Table-driven tests preferred
- Mock service interfaces for handler tests
- Naming: `TestProductService_CreateProduct`
