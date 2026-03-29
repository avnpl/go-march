# Go March Backend — Development Roadmap

> **Concept**: A JSONPlaceholder-style learning API demonstrating 5 different API styles in Go. Users can test and learn REST, GraphQL, SOAP, gRPC, and WebSocket by interacting with a simple inventory/order system.
>
> **Architecture**: Layered design (handlers → services → repos) where service and repo layers are API-agnostic. All API styles share the same business logic.

---

## Strategy

Each API style demonstrates its strengths. No duplication of CRUD across APIs.

```
Phase 0   Critical Fixes ──────────────── resolve review findings before new features
Phase 1   REST Completion ────────────── complete end-to-end flow (products + orders + payments)
Phase 2   GraphQL Enhancement ─────────── orders + nested products
Phase 3   SOAP Implementation ────────── payment transactions
Phase 4   gRPC Analytics ──────────────── high-perf aggregations
Phase 5   WebSocket Real-time ──────────── notifications
Phase 6   Cleanup + Documentation ─────── reset mechanism + README
```

**Current status**: Phase 0 in progress — product REST endpoints work but have code quality/security issues per `agent-reviews/2026-03-12.md`. Phase 1 orders/payments not started.

## Progress Summary

| Phase | Status | Completion |
|-------|--------|------------|
| **Phase 0** | 🔶 In Progress | 6/16 review issues fixed; 10 open (4 high priority, 6 medium) |
| **Phase 1.1** | 🔶 Partial | Product CRUD functional; paths differ from target; validation/IDs/DELETE semantics open |
| **Phase 1.2-1.4** | ⬜ Not Started | Orders, payments, payment simulation not implemented |
| **Phase 2** | 🔶 Minimal | GraphQL has product queries/mutations only; no orders (target: Phase 2) |
| **Phase 3-5** | ⬜ Not Started | SOAP, gRPC, WebSocket are stub packages |
| **Phase 6** | ⬜ Not Started | TTL, reset mechanism, comprehensive README |

**Legend**: ✅ Complete | 🔶 In Progress | ⬜ Not Started

---

## Data Models

### Product
- `prod_id` (string) — format: `PR-XXXXXX` (primary key)
- `prod_name` (string)
- `price` (float64)
- `stock` (int)
- `created_at` (timestamp)
- `updated_at` (timestamp)
- `ttl_expires_at` (timestamp) — for auto-cleanup

### Order
- `order_id` (string) — format: `OR-XXXXXX` (primary key)
- `product_id` (string, FK) — references `prod_id`
- `quantity` (int)
- `total_price` (float64)
- `order_time` (timestamp)
- `status` (string: "pending", "paid", "failed")
- `shipping_address` (string) — updatable
- `notes` (string) — updatable
- `ttl_expires_at` (timestamp)

### Payment
- `payment_id` (string) — format: `PA-XXXXXX` (primary key)
- `order_id` (string, FK) — references `order_id`
- `amount` (float64)
- `status` (string: "pending", "success", "failed")
- `card_number` (string) — stored for simulation validation
- `card_last_four` (string) — last 4 digits
- `created_at` (timestamp)
- `ttl_expires_at` (timestamp)

> **Note**: ID is generated in the service layer. Format: `PR-` for products, `OR-` for orders, `PA-` for payments. Use short random string (6 chars) after prefix.

---

## API Style Mapping

| API | Resources | Purpose | Strength Demonstrated |
|-----|-----------|---------|----------------------|
| REST | Products + Orders + Payments | Complete CRUD + full flow | Standard REST patterns |
| GraphQL | Orders (with nested products) | Filtering + nested queries | Flexible data fetching |
| SOAP | Payments | Transaction operations | XML contracts |
| gRPC | Analytics | Aggregations | High-performance streaming |
| WebSocket | Notifications | Real-time events | Push updates |

---

# Phase 0: Critical Fixes (Before New Features)

These issues should be resolved before implementing orders/payments to avoid propagating anti-patterns.

## 0.1 REST Handler Cleanup

**Priority: High** — affects API contract and security

- [ ] **DELETE semantics** — `DeleteProduct` should return 204 No Content with empty body (review #4)
  - Remove JSON encoding after delete
  - Change status from 200 to 204
  - Update tests/docs to expect no response body
- [ ] **Request body logging** — remove or strictly gate (review #10, #11)
  - Security: request bodies may contain PII, passwords, tokens, credit cards
  - Remove `product_handler.go:38-41, 112-115` debug body logs
  - Remove `main.go:84` GraphQL body log (or gate with `if os.Getenv("LOG_REQUEST_BODIES") == "true"`)
  - Remove unused `strings` import after cleanup (review #11)
- [ ] **Error logging pattern** — fix `fmt.Errorf(...).Error()` anti-pattern (review #5)
  - Locations: `product_handler.go:32, 46, 68, 94, 106, 120`
  - Pattern: `h.log.Error(fmt.Errorf("msg: %w", err).Error(), zap.Error(err))` → `h.log.Error("msg", zap.Error(err))`
  - Critical: line 94 loses error entirely — `fmt.Errorf("FetchAllProducts failed")` without `%w`
- [ ] **Error response consistency** — `UpdateProduct` exposes internal errors (logging improvement L6)
  - Location: `product_handler.go:134`
  - Change: `http.Error(w, err.Error(), http.StatusConflict)` → `utilErrs.SendJSONError(w, http.StatusConflict, "")`

## 0.2 Input Validation

**Priority: High** — prevents invalid data in database

- [ ] **Implement validation** for `CreateProductReq` (review #6)
  - Option A: Manual checks in handler before calling service
    ```go
    if req.Name == "" {
        utilErrs.SendJSONError(w, http.StatusBadRequest, "name is required")
        return
    }
    if req.Price <= 0 {
        utilErrs.SendJSONError(w, http.StatusBadRequest, "price must be greater than 0")
        return
    }
    if req.Stock < 0 {
        utilErrs.SendJSONError(w, http.StatusBadRequest, "stock cannot be negative")
        return
    }
    ```
  - Option B: Add `github.com/go-playground/validator/v10` and use struct tags
  - Recommendation: Option A (fewer dependencies, explicit)
- [ ] **Implement validation** for `UpdateProductReq`
  - If `Name` provided: must not be empty string
  - If `Price` provided: must be > 0
  - If `Stock` provided: must be >= 0
- [ ] **Remove or document** inert `validate` tags on structs
  - Either use them (Option B above) or remove them to avoid confusion

## 0.3 Infrastructure Hardening

**Priority: Medium** — improves reliability and performance

- [ ] **Environment loading** — load `.env` once at startup (review #8)
  - Move `godotenv.Load(".env")` from `getEnvVar` to `main()` before logger init
  - Or use `init()` in `utils` package (runs once on import)
  - Keep `getEnvVar` as simple `os.Getenv(key)` wrapper
- [ ] **Database connection pool** — configure limits (review #9)
  - Add after `sqlx.Connect` in `utils.GetDBPoolObject`:
    ```go
    db.SetMaxOpenConns(25)              // Max concurrent connections
    db.SetMaxIdleConns(10)              // Keep this many idle
    db.SetConnMaxLifetime(5 * time.Minute)  // Recycle connections
    if err := db.Ping(); err != nil {
        logger.Fatal("db ping failed", zap.Error(err))
    }
    ```
  - Consider making these configurable via env vars
- [ ] **SQL consistency** — lowercase keywords and identifiers (review #12)
  - Normalize `product_repo.go` queries to lowercase
  - Define query constants at package level for reusability
  - Example:
    ```go
    const (
        fetchByIDQuery  = "select * from products where prod_id = $1"
        fetchAllQuery   = "select * from products"
        deleteByIDQuery = "delete from products where prod_id = $1 returning *"
    )
    ```

## 0.4 Data Model Improvements

**Priority: Medium** — can be done alongside 0.1-0.3 or deferred to Phase 1 cleanup

- [ ] **Timestamp types** — use `time.Time` instead of `string` (review #7)
  - Change `models.Product`: `CreatedAt time.Time`, `UpdatedAt time.Time`
  - Change `models.Orders`: `CreatedAt time.Time` (field is `order_time` in DB)
  - Update repo queries if needed (pgx/sqlx handle `time.Time` ↔ TIMESTAMPTZ automatically)
  - JSON serialization: Go's `json.Marshal` converts `time.Time` to RFC3339 format automatically
- [ ] **Orders.Amount type** — change from `string` to `float64`
  - Currently: `Amount string` with DB tag `total_price`
  - Should be: `TotalPrice float64` (align field name with DB column or use explicit tag)

---

# Phase 1: REST Completion

## 1.1 Complete Product CRUD

**Implemented routes** (differs from target naming below — consolidate on `/products` + `/products/{id}` when convenient):
- [x] `POST /product` — create product
- [x] `GET /products` — list all products
- [x] `GET /product/{id}` — get single product
- [x] `PATCH /product` — update product (`prod_id` in JSON body)
- [x] `DELETE /product/{id}` — delete product *(still returns 200 with JSON; Phase 1 target is 204 No Content)*

**Target REST shape** (documentation / client examples):
- [ ] `POST /products` — create product
- [x] `GET /products` — list all products *(path matches; POST not on `/products` yet)*
- [ ] `GET /products/{id}` — get single product
- [ ] `PATCH /products/{id}` — update product
- [ ] `DELETE /products/{id}` — delete product
- [ ] `GET /products` — pagination (e.g. `limit` / `offset` or cursor) so list is never unbounded

**Code review follow-ups** (see `agent-reviews/2026-03-12.md` for detailed explanations):

*Fixed (commits c600527–3d8c6e0):*
- [x] **#1** Context by value for `UpdateByID` (not `*context.Context`) — `repos/product_repo.go:61`
- [x] **#2** GraphQL: handle invalid JSON and empty `query` — `main.go:81`
- [x] **#3** ParseInt / invalid path ID returns 400 — `product_handler.go:75-76, 150-151`
- [x] **#13** Use `errors.Is` for sentinel and `sql.ErrNoRows` in handlers — `product_handler.go:53, 79, 153`
- [x] **#14** Close `*sqlx.Rows` in `UpdateByID` (`defer result.Close()`) — prevents connection leak
- [x] **#15** Sentinel errors use standard library `errors` (`utils/errors.go`) — no `pkg/errors` dependency

*Open (blocking Phase 1 completion):*
- [ ] **#4** HTTP status codes — GET and PATCH use 200; **DELETE should be 204** without a body
  - Currently: `DeleteProduct` returns 200 + JSON body
  - Target: 204 No Content with empty body
  - Impact: REST semantics compliance
- [ ] **#5** Error logging — replace remaining `fmt.Errorf(...).Error()` as log message with message + `zap.Error(err)`
  - Locations: `product_handler.go:32, 46, 68, 94, 106, 120`
  - Line 94 is worst: `fmt.Errorf("FetchAllProducts failed")` without `err` — loses error entirely
  - Pattern: `h.log.Error(fmt.Errorf("...%w", err).Error(), zap.Error(err))` → `h.log.Error("message", zap.Error(err))`
- [ ] **#6** Input validation — name required, price > 0, stock >= 0
  - Current: `validate:"required"` tags on `CreateProductReq` are inert — no validator reads them
  - Options: manual checks in handler/service, or add `github.com/go-playground/validator/v10`
  - Affects: `CreateProduct` and `UpdateProduct` handlers
- [ ] **#10** Remove request body logging (security issue — may contain PII/secrets)
  - REST: `product_handler.go:38-41` (CreateProduct), `112-115` (UpdateProduct)
  - GraphQL: `main.go:84` (debug body log)
  - Pattern: `logger.Debug("raw body", zap.String("body", ...))` — remove or gate with strict env check
- [ ] **#11** Import `strings` only used for body logging — remove when #10 is fixed
  - `product_handler.go:12` — `strings.ReplaceAll` appears only in debug logs
- [ ] **#12** Lowercase SQL consistently in `product_repo.go`
  - Line 40: `"SELECT * FROM PRODUCTS WHERE PROD_ID = $1"` (UPPERCASE)
  - Line 51: `"SELECT * FROM PRODUCTS"` (UPPERCASE)
  - Line 106: `"DELETE FROM products where PROD_ID = $1 RETURNING *"` (mixed case!)
  - Target: all lowercase keywords and identifiers

*Open (infrastructure — can defer to Phase 1 cleanup):*
- [ ] **#7** Use `time.Time` (or DB TIMESTAMPTZ) instead of `string` for timestamps in models
  - Affects: `models.Product.CreatedAt`, `UpdatedAt`; `models.Orders.CreatedAt`
  - Impact: type safety, JSON serialization, time operations
  - Requires: model changes + repo query updates
- [ ] **#8** Load `.env` once at startup, not on every `getEnvVar` call
  - Current: `utils/utils.go:16` — `godotenv.Load(".env")` inside `getEnvVar` (file I/O on every call)
  - Fix: load once in `main()` or `init()`
- [ ] **#9** Configure DB connection pool (max open, idle, conn lifetime)
  - Current: `utils/utils.go:54-64` — no pool config after `sqlx.Connect`
  - Add: `db.SetMaxOpenConns(25)`, `SetMaxIdleConns(10)`, `SetConnMaxLifetime(5*time.Minute)`
  - Add: `db.Ping()` to verify connection

*Future (not blocking Phase 1):*
- [ ] **#16** No interfaces for HTTP handlers — consider for testing (minor priority)

**Logging improvements** (deferred to Phase 6 or post-Phase 1 cleanup):
- [ ] **L1** Make log level configurable via `LOG_LEVEL` env var (currently hardcoded to Debug in `utils.BuildLogger`)
- [ ] **L2** Environment-based logger config (development vs production mode)
  - Development: console encoding, file output to `logs/app.log`, stack traces on error
  - Production: JSON encoding, stdout only, stack traces on panic
  - Use `ENV` environment variable to switch modes
- [ ] **L3** Add request ID middleware for context propagation
  - Generate unique request ID per HTTP request (e.g., UUID)
  - Inject into `context.Context` via middleware
  - Include in all logs: `zap.String("request_id", requestID)`
  - Add `X-Request-ID` response header for client correlation
- [ ] **L4** Add logger to GraphQL resolvers
  - Pass `*zap.Logger` to `Resolver` struct (currently only has `productService`)
  - Log errors in resolver methods (currently silent failures)
  - Include query/mutation name in log context
- [ ] **L5** Standardize service layer logging policy
  - Log mutations (create/update/delete) at Info level with entity ID
  - Don't log read operations (get/list) unless they fail
  - Document this policy in CLAUDE.md
- [ ] **L6** Fix error response inconsistency in `UpdateProduct` handler
  - Location: `product_handler.go:134` — `http.Error(w, err.Error(), http.StatusConflict)`
  - Issue: exposes internal error messages to client
  - Fix: use `utilErrs.SendJSONError(w, http.StatusConflict, "")` like other handlers
- [ ] **L7** Standardize debug field naming
  - Remove spaces from field names (`"request param"` → `"id"`)
  - Remove trailing punctuation from messages (`"received ID => "` → `"received request"`)
  - Example: `product_handler.go:74` — `zap.String("request param", idStr)` → `zap.String("id", idStr)`
- [ ] **L8** Add context fields to error logs
  - Include relevant IDs/identifiers when logging errors for traceability
  - Example: `h.log.Error("failed to fetch product", zap.Error(err), zap.String("id", idStr))`
  - Currently some error logs lack context (e.g., `product_handler.go:100` — no context on FetchAll failure)

**ID generation** (still numeric `int64` IDs end-to-end in handlers/services/repos):
- [ ] Change `prod_id` from INT8 to STRING in database schema
- [ ] Update `models.Product` — `ProductID` already has `string` JSON tag but handlers parse as `int64`
- [ ] Update `ProductRepo` interface and `pgProductRepo` — methods take/return `string` IDs
- [ ] Update `ProductService` interface and implementation — methods take/return `string` IDs
- [ ] Update REST handlers — remove `strconv.ParseInt`, use path value directly as string
- [ ] Update GraphQL resolvers — type-assert `id` as `string` instead of converting to `int64`
- [ ] Generate `PR-XXXXXX` ID in service layer on create (6-char random alphanumeric after prefix)
  - Use `crypto/rand` or `math/rand` with seed for ID generation
  - Example: `PR-A1B2C3`, `PR-9XYZ42`
  - Ensure uniqueness (retry on conflict or use timestamp component)

## 1.2 Complete Orders CRUD

**Endpoints**:
- [ ] `POST /orders` — create order (decrements stock)
- [ ] `GET /orders` — list orders *(with pagination; same style as `GET /products`)*
- [ ] `GET /orders/{id}` — get single order
- [ ] `PATCH /orders/{id}` — update (address, notes ONLY)

**Business logic**:
- [ ] Validate product exists and has sufficient stock
- [ ] Decrement stock on order creation
- [ ] Auto-set order status based on payment (handled later)
- [ ] Generate `OR-XXXXXX` ID in service layer on create

## 1.3 Complete Payments API

**Endpoints** (no refunds):
- [ ] `POST /payments` — create payment (simulate authorize)
- [ ] `GET /payments/{id}` — get payment status
- [ ] Link payment to order via `order_id`
- [ ] Generate `PA-XXXXXX` ID in service layer on create

## Payment simulation

- [ ] Payment fails if card number ends in "6969"
- [ ] All other card numbers succeed (deterministic for testing)
- [ ] Atomic operation: create payment + set order status in single transaction
- [ ] Payment status = "success" → Order status = "paid"
- [ ] Payment status = "failed" → Order status = "failed"

## 1.4 Order Update Scope

**Only these fields updatable**:
- `shipping_address`
- `notes`

**Not updatable** (automatic or admin only):
- `status` — set by payment flow
- `delivery_date` — out of scope

---

# Phase 2: GraphQL Enhancement

## 2.1 GraphQL Schema

**Query**:
```graphql
type Query {
  orders(status: String): [Order!]!
  order(id: ID!): Order
}
```

**Mutation**: None (REST handles all mutations)

**Order Type** (with nested product):
```graphql
type Order {
  order_id: ID!
  product: Product!
  quantity: Int!
  total_price: Float!
  status: String!
  shipping_address: String
  notes: String
  created_at: String!
}

type Product {
  prod_id: ID!
  prod_name: String!
  price: Float!
  stock: Int!
}
```

## 2.2 Resolver Implementation

- [ ] `orders` query with optional status filter and pagination (same semantics as REST list)
- [ ] `order` query by ID
- [ ] Nested `product` resolver in Order type
- [ ] Use existing `OrderService` from shared layer

## 2.3 Integration

- [ ] Register GraphQL endpoint at `/graphql`
- [ ] Reuse `OrderService` (API-agnostic — same as REST)
- [ ] Context propagation: HTTP context → resolver → service

---

# Phase 3: SOAP Implementation

## 3.1 SOAP Endpoints

**Purpose**: Payment transactions with strict XML contracts. Demonstrates enterprise/XML patterns.

**Operations**:
- [ ] `PlaceOrder` — create order + process payment atomically
- [ ] `GetPaymentStatus` — retrieve payment details

## 3.2 XML Schema

**SOAP Envelope Structure**:
```xml
<soap:Envelope>
  <soap:Header>
    <!-- optional auth -->
  </soap:Header>
  <soap:Body>
    <PlaceOrderRequest>
      <product_id>123</product_id>
      <quantity>2</quantity>
      <shipping_address>123 Main St</shipping_address>
      <payment>
        <card_number>4111111111111111</card_number>
        <expiry>12/25</expiry>
      </payment>
    </PlaceOrderRequest>
  </soap:Body>
</soap:Envelope>
```

## 3.3 Implementation

- [ ] XML structs with `encoding/xml` tags
- [ ] `SOAPAction` header handling
- [ ] SOAP Fault for errors
- [ ] Reuse `OrderService` + `PaymentService` (shared layer)
- [ ] Endpoint: `POST /soap`

## 3.4 Payment Simulation

- [ ] Simulate authorization (success/failure)
- [ ] Return appropriate SOAP response
- [ ] Link order + payment in database

---

# Phase 4: gRPC Analytics

## 4.1 Protocol Buffer Definition

**File**: `proto/analytics.proto`

```protobuf
service AnalyticsService {
  rpc GetTotalSales(GetTotalSalesRequest) returns (GetTotalSalesResponse);
  rpc GetAverageOrderValue(GetAverageOrderValueRequest) returns (GetAverageOrderValueResponse);
  rpc GetTopProducts(GetTopProductsRequest) returns (stream ProductStat);
  rpc GetLowStockProducts(GetLowStockProductsRequest) returns (stream ProductStat);
}
```

**Messages**:
```protobuf
message GetTotalSalesRequest {
  // optional date range
}

message GetTotalSalesResponse {
  int64 total_orders = 1;
  double total_revenue = 2;
}

message ProductStat {
  int64 product_id = 1;
  string product_name = 2;
  int64 units_sold = 3;
  double revenue = 4;
}
```

## 4.2 Implementation

- [ ] Generate Go code from proto
- [ ] Create `AnalyticsService` in `services/`
- [ ] Add aggregate SQL queries to repo
- [ ] Implement gRPC server in `api/grpc/`
- [ ] Run on separate port (`:50051`)

## 4.3 Streaming (Optional)

- [ ] Server-side streaming for top products / low stock
- [ ] Demonstrates gRPC streaming capability

---

# Phase 5: WebSocket Real-time

## 5.1 WebSocket Architecture

**Library**: `nhooyr.io/websocket` (context-aware, modern)

**Connection Management**: Hub pattern
- `clients` — map of connections
- `broadcast` — channel for messages
- `register/unregister` — channels for connection lifecycle

## 5.2 Events

**Subscription topics**:
- [ ] `orders` — new order created
- [ ] `payments` — payment status changed
- [ ] `alerts` — low stock warnings

**Message Format**:
```json
{
  "type": "order_created",
  "data": {
    "order_id": 123,
    "total_price": 99.99,
    "status": "paid"
  }
}
```

## 5.3 Integration

- [ ] Create `hub` struct with run loop
- [ ] HTTP upgrade handler at `/ws`
- [ ] Per-connection read/write pumps
- [ ] Emit events from service layer (channel or callback)
- [ ] Graceful disconnect handling

---

# Phase 6: Cleanup + Documentation

## 6.1 Reset Mechanism

**Mechanism**: Database TTL (CockroachDB native) with configurable duration

**How TTL works**:
1. Each table has `ttl_expires_at` column (TIMESTAMPTZ)
2. CockroachDB auto-deletes rows when current time > `ttl_expires_at`
3. Sample data: `ttl_expires_at = NULL` (never expires)
4. User-inserted data: `ttl_expires_at = NOW() + TTL_DURATION`

**TTL Duration Configuration**:
- [ ] Add `TTL_DURATION` environment variable (default: 3 hours, min: 1 minute)
- [ ] Service layer reads `TTL_DURATION` on startup
- [ ] On insert: set `ttl_expires_at = NOW() + TTL_DURATION`
- [ ] On update: reset `ttl_expires_at = NOW() + TTL_DURATION` (if updating row)

**Implementation**:
- [ ] Add `ttl_expires_at` column to products, orders, payments tables
- [ ] Enable TTL on tables using `ttl_expiration_expression`:
  ```sql
  ALTER TABLE products SET (ttl_expiration_expression = 'ttl_expires_at');
  ALTER TABLE orders SET (ttl_expiration_expression = 'ttl_expires_at');
  ALTER TABLE payments SET (ttl_expiration_expression = 'ttl_expires_at');
  ```
- [ ] Service layer: set `ttl_expires_at = NOW() + TTL_DURATION` on new inserts
- [ ] On row read: optionally update `ttl_expires_at` to reset timer

**Sample data**: Always `ttl_expires_at = NULL` (permanent)

**Note**: CockroachDB handles auto-deletion. No API endpoint needed.

## 6.2 README

**Content**:
- [ ] Overview of each API style
- [ ] REST endpoints with curl examples
- [ ] GraphQL queries examples
- [ ] SOAP request/response samples
- [ ] gRPC `grpcurl` examples
- [ ] WebSocket client example

## 6.3 Testing Checklist

- [ ] REST: curl all endpoints
- [ ] GraphQL: queries via Postman/Insomnia
- [ ] SOAP: XML envelope examples
- [ ] gRPC: `grpcurl` commands
- [ ] WebSocket: client connection test

---

# Architecture Principles

## Service/Repo Layer API-Agnostic

All API styles (REST, GraphQL, SOAP, gRPC, WebSocket) use the **same service layer**:

```
┌────────────────────────────────────────────────────────────────┐
│  API Handlers (REST / GraphQL / SOAP / gRPC / WebSocket)       │
├────────────────────────────────────────────────────────────────┤
│  Services (ProductService, OrderService, PaymentService...)    │
│  - Business logic only                                         │
│  - No HTTP/gRPC/WS knowledge                                   │
├────────────────────────────────────────────────────────────────┤
│  Repos (ProductRepo, OrderRepo, PaymentRepo...)                │
│  - Database access only                                        │
│  - No business logic                                           │
├────────────────────────────────────────────────────────────────┤
│  Database (CockroachDB)                                         │
└────────────────────────────────────────────────────────────────┘
```

**Why**:
- Avoid duplication of business logic
- Consistency across API styles
- Easier testing (mock services)
- Clear separation of concerns

---

# Technical Standards

## Error Handling
- Use sentinel errors from `utils/errors.go`
- Wrap with context: `fmt.Errorf("service.Method: %w", err)`
- Return structured errors (not internal details)

## Database
- Parameterized queries only (no string concat)
- Lowercase SQL keywords
- Configure connection pool

## Context
- Pass `context.Context` as first parameter, by value
- Propagate `r.Context()` from HTTP handlers

## Naming
- Files: `snake_case.go`
- Interfaces: `PascalCase`
- Functions: `camelCase`

## Logging
- Use `zap` logger
- Structure: `logger.Info("message", zap.String("key", value))`
- Never log request bodies (may contain PII/secrets)
- Static messages with structured fields (not `fmt.Errorf().Error()`)
- Log errors at handler layer, business events at service layer

---

# Dependencies

| Package | Purpose |
|---------|---------|
| `jackc/pgx/v5` | PostgreSQL/CockroachDB driver |
| `jmoiron/sqlx` | Database access |
| `go.uber.org/zap` | Structured logging |
| `graphql-go/graphql` | GraphQL implementation |
| `nhooyr.io/websocket` | WebSocket implementation |

---

# Endpoints Summary

## REST (`:8080`)

**Current implementation**
```
/product          POST (create), PATCH (update)
/products         GET (list)
/product/{id}     GET, DELETE
/graphql          POST
```

**Planned (Phase 1 complete)**
```
/products         GET, POST
/products/{id}    GET, PATCH, DELETE
/orders           GET, POST
/orders/{id}      GET, PATCH
/payments         POST
/payments/{id}    GET
/graphql          POST
```

## SOAP (`:8080/soap`)

```
POST /soap        PlaceOrder, GetPaymentStatus
```

## gRPC (`:50051`)

```
AnalyticsService: GetTotalSales, GetAverageOrderValue, GetTopProducts, GetLowStockProducts
```

## WebSocket (`:8080/ws`)

```
WS /ws            Subscribe to: orders, payments, alerts
```