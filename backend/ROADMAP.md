# Go March Backend — Development Roadmap

> **Context:** See [`CLAUDE.md`](./CLAUDE.md) for project conventions, architecture, and tech stack.
> **Code Review:** See [`agent-reviews/2026-03-12.md`](./agent-reviews/2026-03-12.md) for the full issue breakdown with code snippets and explanations.

---

## Strategy

Fix critical bugs first, then harden the foundation, then build features layer by layer. Each phase is completed and reviewed before the next begins.

```
Phase 0  Critical Fixes ──────────── stop the bleeding
Phase 1  Foundation Hardening ─────── models, utils, error handling, DB
Phase 2  REST Completion ──────────── order endpoints + polish
Phase 3  GraphQL Completion ───────── missing mutations + order ops
Phase 4  SOAP API ─────────────────── transactional order placement
Phase 5  gRPC API ─────────────────── analytics procedures
Phase 6  WebSocket API ────────────── real-time streaming
Phase 7  Final Integration ────────── refactor, test, document
```

---

## Phase 0: Critical Bug Fixes

These are correctness and safety bugs. Fix them before any feature work.

### 0.1 Fix `errors.As` → `errors.Is` (Review #13)

- [ ] `api/rest/product_handler.go:53` — change `errors.As(err, &utilErrs.ErrConflict)` to `errors.Is(err, utilErrs.ErrConflict)`
- [ ] `api/rest/product_handler.go:79` — change `errors.As(err, &sql.ErrNoRows)` to `errors.Is(err, sql.ErrNoRows)`
- [ ] `api/rest/product_handler.go:153` — same as above
- **Why first:** These comparisons may silently fail, causing wrong HTTP responses for real errors.

### 0.2 Add Missing ParseInt Error Check (Review #3)

- [ ] `api/rest/product_handler.go:75-76` — add `if err != nil` check after `strconv.ParseInt` in `FetchProduct`
- [ ] `api/rest/product_handler.go:150-151` — same fix in `DeleteProduct`
- **Why:** Without the check, invalid IDs (e.g., "abc") silently become `0` and hit the database.

### 0.3 Close `*sqlx.Rows` in UpdateByID (Review #14)

- [ ] `repos/product_repo.go:87` — add `defer result.Close()` after the `NamedQueryContext` nil-error check
- **Why:** Unclosed Rows leak database connections. Under load the pool exhausts and the app hangs.

### 0.4 Fix Context Pointer Anti-Pattern (Review #1)

- [ ] `repos/product_repo.go:17` — interface: change `UpdateByID(ctx *context.Context, ...)` to `UpdateByID(ctx context.Context, ...)`
- [ ] `repos/product_repo.go:61` — implementation: same signature change, use `ctx` directly instead of `*ctx`
- [ ] `services/product_service.go:64` — call site: change `s.repo.UpdateByID(&ctx, req)` to `s.repo.UpdateByID(ctx, req)`

### 0.5 Handle GraphQL JSON Decode Error (Review #2)

- [ ] `main.go:81` — check the return value of `json.NewDecoder(r.Body).Decode(&params)`
- [ ] Return `400 Bad Request` on decode failure
- [ ] Return `400 Bad Request` if `params.Query` is empty

### 0.6 Replace `pkg/errors` with Standard Library (Review #15)

- [ ] `utils/errors.go` — change import from `"github.com/pkg/errors"` to `"errors"`
- [ ] Remove `github.com/pkg/errors` from `go.mod` / `go.sum`
- [ ] Run `go mod tidy`

---

## Phase 1: Foundation Hardening

Address medium-severity issues and prepare the data layer for order features.

### 1.1 Fix HTTP Status Codes (Review #4)

- [ ] `api/rest/product_handler.go:87` — `FetchProduct`: change `StatusCreated` → `StatusOK`
- [ ] `api/rest/product_handler.go:136` — `UpdateProduct`: change `StatusCreated` → `StatusOK`
- [ ] `api/rest/product_handler.go:162` — `DeleteProduct`: change `StatusCreated` → `StatusNoContent`, remove JSON body encoding

### 1.2 Fix Error Logging Pattern (Review #5)

Replace `fmt.Errorf(...).Error()` with plain string messages across all 6 locations:
- [ ] Line 32 — `CreateProduct` read body
- [ ] Line 46 — `CreateProduct` decode
- [ ] Line 68 — `FetchProduct` no ID
- [ ] Line 94 — `FetchAllProducts` (also add `zap.Error(err)` — currently lost entirely)
- [ ] Line 106 — `UpdateProduct` read body
- [ ] Line 120 — `UpdateProduct` decode

### 1.3 Remove Request Body Logging (Review #10 + #11)

- [ ] Remove debug body logging at `api/rest/product_handler.go:37-42` (CreateProduct)
- [ ] Remove debug body logging at `api/rest/product_handler.go:111-116` (UpdateProduct)
- [ ] Remove the `"strings"` import (line 12) — becomes unused after the above
- [ ] Remove the `reqString` variable and `bytes`/`io` body-read-and-rewrite pattern if no longer needed

### 1.4 Fix Model Types (Review #7)

- [ ] `models/models.go` — change `Product.CreatedAt` and `Product.UpdatedAt` from `string` to `time.Time`
- [ ] `models/models.go` — change `Orders.CreatedAt` from `string` to `time.Time`
- [ ] `models/models.go` — change `Orders.Amount` from `string` to `float64`
- [ ] Add JSON tags to all `Orders` fields
- [ ] Verify `sqlx` scans `time.Time` correctly from CockroachDB timestamps

### 1.5 Implement Input Validation (Review #6)

- [ ] Remove inert `validate:"required"` tags from `CreateProductReq`
- [ ] Create manual validation functions in `utils/validations.go`:
  - `ValidateCreateProductReq` — name non-empty, price > 0, stock >= 0
  - `ValidateUpdateProductReq` — at least one field set
  - `ValidateCreateOrderReq` — (for Phase 2)
- [ ] Call validators in handlers before service calls
- [ ] Return `400 Bad Request` with specific field error messages

### 1.6 Load .env Once at Startup (Review #8)

- [ ] Move `godotenv.Load(".env")` to `main.go` (top of `main()`)
- [ ] Simplify `getEnvVar` in `utils/utils.go` to just return `os.Getenv(key)`
- [ ] Handle missing `.env` gracefully (log warning, don't fatal — support system env vars)

### 1.7 Configure DB Connection Pool (Review #9)

- [ ] Add to `GetDBPoolObject` in `utils/utils.go`:
  - `db.SetMaxOpenConns(25)`
  - `db.SetMaxIdleConns(10)`
  - `db.SetConnMaxLifetime(5 * time.Minute)`
- [ ] Add `db.Ping()` check after connect
- [ ] Log pool configuration on startup

### 1.8 Normalize SQL Casing (Review #12)

- [ ] `repos/product_repo.go` — standardize all SQL to lowercase:
  - Line 40: `PRODUCTS` → `products`, `PROD_ID` → `prod_id`
  - Line 51: `PRODUCTS` → `products`
  - Line 105: `PROD_ID` → `prod_id`
- [ ] Extract repeated queries into package-level `const` block

### 1.9 Database Migrations

- [ ] Create `/migrations` directory
- [ ] `001_create_products.sql` — products table DDL
- [ ] `002_create_orders.sql` — orders table with FK to products
- [ ] Naming convention: `NNN_description.up.sql` / `NNN_description.down.sql`

### 1.10 Order Repository

- [ ] Define `OrderRepo` interface in `repos/order_repo.go`:
  - `Create(ctx context.Context, order *models.Order) (models.Order, error)`
  - `FetchByID(ctx context.Context, id int64) (models.Order, error)`
  - `FetchAll(ctx context.Context) ([]models.Order, error)`
  - `FetchByProductID(ctx context.Context, productID int64) ([]models.Order, error)`
- [ ] Implement `pgOrderRepo` with `*sqlx.DB`
- [ ] Use parameterized queries, lowercase SQL, proper error wrapping

### 1.11 Order Service

- [ ] Define `OrderService` interface in `services/order_service.go`:
  - `PlaceOrder(ctx context.Context, req *models.CreateOrderReq) (models.Order, error)`
  - `GetOrder(ctx context.Context, id int64) (models.Order, error)`
  - `GetAllOrders(ctx context.Context) ([]models.Order, error)`
  - `GetOrdersByProduct(ctx context.Context, productID int64) ([]models.Order, error)`
- [ ] Implement `orderService` with `OrderRepo` + `ProductRepo` dependencies
- [ ] `PlaceOrder` business logic: validate product exists, check stock, calculate total, decrement stock
- [ ] Add `ErrInsufficientStock` sentinel to `utils/errors.go`

---

### Checkpoint: Foundation Complete

- [ ] All 16 review issues resolved
- [ ] `go vet ./...` passes clean
- [ ] `go build ./...` succeeds
- [ ] Order repo + service implemented with consistent patterns
- [ ] No `*context.Context`, no `pkg/errors`, no unclosed Rows

---

## Phase 2: REST API Completion

### 2.1 Order REST Endpoints

- [ ] Create `OrderHandler` in `api/rest/order_handler.go`
  - Constructor: `NewOrderHandler(svc services.OrderService, log *zap.Logger)`
  - `CreateOrder` — POST `/order` → 201
  - `GetOrder` — GET `/order/{id}` → 200
  - `GetAllOrders` — GET `/orders` → 200
- [ ] Register routes in `main.go`, wire order repo → service → handler
- [ ] Apply all conventions from Phase 0/1 fixes (proper status codes, error handling, no body logging)

### 2.2 REST Polish

- [ ] Consistent JSON response envelope across all endpoints (optional)
- [ ] Verify every handler: decode → validate → service → map error → respond
- [ ] Ensure `Content-Type: application/json` set on all responses

---

### Checkpoint: REST Complete

- [ ] Product + Order CRUD fully functional via REST
- [ ] GET → 200, POST → 201, PATCH → 200, DELETE → 204
- [ ] Error responses never expose internal details
- [ ] curl test all endpoints

---

## Phase 3: GraphQL API Completion

### 3.1 Complete Product Mutations

- [x] Add `createProduct` mutation in `api/graphql/mutations.go` + input type in `types.go`
- [x] Add `deleteProduct` mutation (takes ID, returns deleted product)
- [x] Implement resolvers in `resolvers.go`

### 3.2 Order GraphQL Types

- [ ] Define `OrderType` in `api/graphql/types.go`
  - Fields: `order_id`, `product_id`, `quantity`, `total_price`, `order_time`
  - Optional: nested `product` field with resolver
- [ ] Define `CreateOrderInput` input type

### 3.3 Order Queries and Mutations

- [ ] `order(id: String!)` — single order query
- [ ] `orders` — all orders query
- [ ] `ordersByProduct(productId: String!)` — orders by product
- [ ] `placeOrder(input: CreateOrderInput!)` — mutation
- [ ] Update `NewSchema` to inject `OrderService`

---

### Checkpoint: GraphQL Complete

- [ ] All product + order operations available via GraphQL
- [ ] Schema introspection works
- [ ] Context propagated from HTTP → resolvers → services

---

## Phase 4: SOAP API

Purpose: transactional order placement with strict XML contracts.

### 4.1 SOAP Infrastructure

- [ ] Define `SOAPEnvelope`, `SOAPBody`, `SOAPFault` structs with `encoding/xml` tags in `api/soap/`
- [ ] XML marshal/unmarshal working correctly

### 4.2 SOAP Order Service

- [ ] `PlaceOrderRequest` / `PlaceOrderResponse` types
- [ ] SOAP handler: parse envelope → extract action → call `OrderService.PlaceOrder` → wrap response
- [ ] `SOAPFault` for error responses
- [ ] Register `/soap` endpoint in `main.go`
- [ ] Content-Type: `text/xml` or `application/soap+xml`

### 4.3 WSDL (Optional)

- [ ] Static WSDL file describing the service
- [ ] Serve at `/soap?wsdl`

---

## Phase 5: gRPC API

Purpose: high-performance analytics procedures.

### 5.1 Protocol Buffer Setup

- [ ] Install `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`
- [ ] Create `/proto/analytics.proto` — messages + service definition
- [ ] Generate Go code

### 5.2 Analytics Service

- [ ] Add analytics methods (new `AnalyticsService` or extend `OrderService`):
  - `GetTotalOrderCount(ctx) (int64, error)`
  - `GetAverageOrderValue(ctx) (float64, error)`
  - `GetProductStats(ctx, productID) (stats, error)`
- [ ] Add aggregate SQL queries to repos

### 5.3 gRPC Server

- [ ] Implement gRPC service in `api/grpc/grpc.go`
- [ ] Start on separate port (`:50051`)
- [ ] Graceful shutdown alongside HTTP server

### 5.4 gRPC Streaming (Optional)

- [ ] Server-side streaming RPC for real-time stats
- [ ] Proper stream close + `context.Done()` handling

---

## Phase 6: WebSocket API

Purpose: real-time streaming of new orders and analytics.

### 6.1 WebSocket Infrastructure

- [ ] Create `api/websocket/` package
- [ ] Choose library: `nhooyr.io/websocket` (context-aware) or `gorilla/websocket`
- [ ] Upgrader + handler, define JSON message types (`subscribe`, `unsubscribe`, `order_created`, `stats_update`)

### 6.2 Hub Pattern

- [ ] Connection hub with `clients map`, `broadcast chan`, `register/unregister chan`
- [ ] `Run()` goroutine managing channels
- [ ] Per-connection `readPump()` / `writePump()` goroutines

### 6.3 Event Integration

- [ ] Modify `OrderService.PlaceOrder` to emit events (channel or callback)
- [ ] Periodic analytics via `time.Ticker`

### 6.4 WebSocket Endpoint

- [ ] Register `/ws` in `main.go`
- [ ] Subscription topics: `"orders"`, `"analytics"`
- [ ] Clean connection teardown, no goroutine leaks

---

## Phase 7: Final Integration

### 7.1 main.go Refactoring

- [ ] Extract server setup if warranted
- [ ] All services share repos (single DB pool)
- [ ] Configuration management (ports, timeouts, pool sizes)
- [ ] Graceful shutdown for HTTP + gRPC + WebSocket

### 7.2 Testing

- [ ] Unit tests for services (mock repos via interfaces)
- [ ] Unit tests for handlers (mock services via interfaces)
- [ ] Table-driven test style
- [ ] `go test -cover ./...` — aim for meaningful coverage on service logic

### 7.3 Manual Testing Checklist

- [ ] REST: curl all product + order endpoints
- [ ] GraphQL: queries + mutations via Postman/Insomnia
- [ ] SOAP: send SOAP envelope for order placement
- [ ] gRPC: `grpcurl` analytics methods
- [ ] WebSocket: `wscat` real-time updates

### 7.4 Documentation

- [ ] Update `README.md` with all endpoints and examples
- [ ] Document GraphQL schema
- [ ] Document SOAP WSDL location
- [ ] Document gRPC proto reference
- [ ] Document WebSocket message formats

---

### Final Checkpoint

**Architecture:**
- [ ] All 5 APIs share the same service + repository layers
- [ ] No business logic in handlers/resolvers
- [ ] Clean dependency injection throughout
- [ ] Single database connection pool shared

**Go Conventions:**
- [ ] Consistent error handling (`errors.Is`, `fmt.Errorf` with `%w`)
- [ ] Standard library `errors` only (no `pkg/errors`)
- [ ] Context propagated everywhere, by value
- [ ] No circular dependencies

**Performance & Concurrency:**
- [ ] HTTP + gRPC servers run concurrently
- [ ] WebSocket hub manages goroutines cleanly
- [ ] No goroutine leaks (`runtime.NumGoroutine()`)
- [ ] DB pool configured

**Security:**
- [ ] Parameterized SQL only
- [ ] No request body logging
- [ ] Error messages never leak internal details
