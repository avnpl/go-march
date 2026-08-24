# Go March Backend — Development Roadmap

> **Concept**: A JSONPlaceholder-style learning API demonstrating 4 different API styles in Go. Users can test and learn REST, GraphQL, gRPC, and WebSocket by interacting with a simple inventory/order system.
>
> **Architecture**: Layered design (handlers → services → repos) where service and repo layers are API-agnostic. All API styles share the same business logic.

---

## Strategy

Each API style demonstrates its strengths. No duplication of CRUD across APIs.

```
Phase 1   REST Completion ────────────── complete end-to-end flow (products + orders + payments)
Phase 2   GraphQL Enhancement ─────────── orders + nested products
Phase 3   gRPC Analytics ──────────────── high-perf aggregations
Phase 4   WebSocket Real-time ──────────── notifications
Phase 5   Cleanup + Documentation ─────── reset mechanism + README
Phase 6   User Authentication ─────────── token-based auth with middleware
Phase 7   Perf polish ─────────────────── GraphQL nested product batch fetch (after core features)
```

**Current status**: Product CRUD complete. Order CRUD complete. Phase 2 GraphQL order queries + nested `product` complete. Next: Phase 3 gRPC.

## Progress Summary

| Phase | Status | Notes |
|-------|--------|-------|
| **Phase 1.1** | ✅ Complete | Product CRUD with routes (`/products`, `/products/{id}`) + pagination |
| **Phase 1.2** | ✅ Complete | Order CRUD: POST, GET list, GET by ID. No PATCH/DELETE/Payments. |
| **Phase 1.3-1.4** | N/A | Out of scope — no payments, no order update/delete |
| **Phase 2** | ✅ Complete | `getOrderByID`, `getAllOrders`, nested `product` (still 1+N; batch fetch is Phase 7) |
| **Phase 3-4** | ⬜ Not Started | gRPC, WebSocket stubs |
| **Phase 5** | ⬜ Not Started | TTL, README |
| **Phase 6** | ⬜ TODO | User authentication with middleware (after Phase 5) |
| **Phase 7** | ⬜ Later | Nested product batch-by-IDs (after Phases 3–6) |

**Legend**: ✅ Complete | 🔶 In Progress | ⬜ Not Started

---

## Data Models

### Product (current)
- `prod_id` (string) — format: `PR-XXXXXX` (primary key)
- `prod_name` (string)
- `price` (float64)
- `stock` (int)
- `created_at` (timestamp)
- `updated_at` (timestamp)
- `ttl_expires_at` (timestamp) — for auto-cleanup

### Order (implemented, Phase 1.2 complete — supersedes the original spec below)
- `order_id` (string) — format: `OR-XXXXXX` (primary key)
- `product_id` (string, FK) — references `prod_id`
- `quantity` (int)
- `amount` (float64) — validated against `product.price * quantity` (epsilon 0.005)
- `created_at` (timestamp)
- `status` (string) — currently always set to `"success"` on create; no state machine yet
- `shipping_address` (string)
- `card_number` (string) — 4-digit simulated card token (not a real PAN); `"6969"` simulates a failed transaction
- `notes` (string, optional)
- `ttl_expires_at` (timestamp)

> No `user_id` column exists — auth (Phase 6) hasn't landed. No separate `Payment` model/table is used by the app; Phase 1.3/1.4 payments were dropped from scope (see Progress Summary). The `payments` table still exists in `migrations/003_create_payments.up.sql` but nothing in Go code reads or writes it.

> **Note**: ID is generated in the service layer. Format: `PR-` for products, `OR-` for orders, `PA-` for payments. Use short random string (7 chars) after prefix.

---

## API Style Mapping

| API | Resources | Purpose | Strength Demonstrated |
|-----|-----------|---------|----------------------|
| REST | Products + Orders | Complete CRUD + full flow | Standard REST patterns |
| GraphQL | Orders (with nested products) | Filtering + nested queries | Flexible data fetching |
| gRPC | Analytics | Aggregations | High-performance streaming |
| WebSocket | Notifications | Real-time events | Push updates |

---

# Phase 1: REST Completion

## 1.1 Complete Product CRUD

**Implemented routes**:
- [x] `POST /products` — create product
- [x] `GET /products` — list all products
- [x] `GET /products/{id}` — get single product
- [x] `PATCH /products/{id}` — update product
- [x] `DELETE /products/{id}` — delete product *(still returns 200 with JSON; Phase 1 target is 204 No Content)*

**Target REST shape** (documentation / client examples):
- [x] `POST /products` — create product
- [x] `GET /products` — list all products
- [x] `GET /products/{id}` — get single product
- [x] `PATCH /products/{id}` — update product
- [x] `DELETE /products/{id}` — delete product
- [x] `GET /products` — pagination (e.g. `limit` / `offset` or cursor) so list is never unbounded

**Logging improvements** (deferred to Phase 6 or post-Phase 1 cleanup):
- [x] **L1** Make log level configurable via `LOG_LEVEL` env var (currently hardcoded to Debug in `utils.BuildLogger`)
- [x] **L2** Environment-based logger config (development vs production mode)
  - Development: console encoding, file output to `logs/app.log`, stack traces on error
  - Production: JSON encoding, stdout only, stack traces on panic
  - Use `ENV` environment variable to switch modes
- [x] **L3** Add request ID middleware for context propagation
  - Generate unique request ID per HTTP request (e.g., UUID)
  - Inject into `context.Context` via middleware
  - Include in all logs: `zap.String("request_id", requestID)`
- [x] **L4** Add logger to GraphQL resolvers
  - Pass `*zap.Logger` to `Resolver` struct (currently only has `productService`)
  - Log errors in resolver methods (currently silent failures)
  - Include query/mutation name in log context
- [x] **L5** Standardize service layer logging policy
  - Log mutations (create/update/delete) at Info level with entity ID
  - Don't log read operations (get/list) unless they fail
  - Document this policy in CLAUDE.md
- [x] **L6** Fix error response inconsistency in `UpdateProduct` handler
  - Location: `product_handler.go:143` — was `http.Error(w, err.Error(), http.StatusConflict)`
  - Issue: exposed internal error messages to client
  - Fix: changed to `utils.SendJSONError(w, http.StatusConflict, "")`
- [x] **L7** Standardize debug field naming
  - Remove spaces from field names (`"request param"` → `"id"`)
  - Remove trailing punctuation from messages (`"received ID => "` → `"received request"`)
  - Example: `product_handler.go:74` — `zap.String("request param", idStr)` → `zap.String("id", idStr)`
- [x] **L8** Add context fields to error logs
  - Include relevant IDs/identifiers when logging errors for traceability
  - Example: `h.log.Error("failed to fetch product", zap.Error(err), zap.String("id", idStr))`
  - Currently some error logs lack context (e.g., `product_handler.go:100` — no context on FetchAll failure)

**ID generation**:
- [x] Change `prod_id` from INT8 to STRING in database schema
- [x] All code layers use string IDs (models, repo, service, handlers, GraphQL)

## 1.2 Complete Orders CRUD

**Endpoints**:
- [x] `POST /orders` — create order (decrements stock)
- [x] `GET /orders` — list orders *(with pagination; same style as `GET /products`)*
- [x] `GET /orders/{id}` — get single order

**Business logic**:
- [x] Validate product exists and has sufficient stock
- [x] Decrement stock on order creation
- [x] Auto-set order status based on payment (handled later)
- [x] Generate `OR-XXXXXX` ID in service layer on create

**Create order — pending / incomplete** (handler or service may exist before this is finished; implement end-to-end in service + repo layers):
- [x] **Stock check**: before insert, ensure `products.stock >= quantity`; reject when insufficient (prevents overselling under concurrency when combined with transactional update below).
- [x] **Stock decrement**: update `products.stock` (subtract `quantity`) in the **same transaction** as inserting the order row so both succeed or both roll back.
- [x] **Error mapping** (do not expose internal DB messages): product missing → **404**; insufficient stock → **409 Conflict** (or **422 Unprocessable Entity**, project-wide pick one); invalid body / `quantity <= 0` → **400**; transaction / unexpected failures → **500** with generic client message.
- [ ] Optional later: retries, idempotency keys, or row-level locking strategy if contention shows up in tests.

---

# Phase 2: GraphQL Enhancement

Product queries and mutations were already in the schema before this phase. Phase 2 adds orders. REST still owns create/update/delete for orders.

## 2.1 GraphQL Schema

**Query (current)**:
```graphql
type Query {
  getProductByID(id: String!): Product
  getAllProducts(limit: Int = 10, offset: Int = 0): [Product]
  getOrderByID(id: String!): Order
  getAllOrders(limit: Int = 10, offset: Int = 0): [Order]
}
```

The original spec used `order(id)` / `orders(...)`. The live names follow the product queries (`getProductByID`, `getAllProducts`). Keep that pattern.

**Mutation**: No order mutations. Product `updateProduct` and `deleteProduct` already exist. REST handles order writes.

**Order Type** (live, with nested product):
```graphql
type Order {
  order_id: String
  product_id: String
  quantity: Int
  amount: Float
  status: String
  shipping_address: String
  notes: String
  created_at: String
  product: Product
}

type Product {
  prod_id: String
  prod_name: String
  price: Float
  stock: Int
  created_at: String
  updated_at: String
}
```

Field names match the Go `Order` struct, so GraphQL can resolve scalars without custom field resolvers. `amount` is the order total. The original spec called this `total_price`. Nested `product` uses a field resolver. `GetOrderByID` / `GetAllOrders` return `models.Order`. Then `ResolveOrderProduct` loads the product via `ProductService.GetProductByID` using `order.ProductID`. GraphQL runs that resolver only when the client asks for `product`. That path is **1+N** today (one product query per order). Batch-by-IDs is Phase 7, after the core feature phases.

## 2.2 Resolver Implementation

- [x] `getAllOrders` query with pagination (`limit` / `offset`, same style as REST list)
- [x] `getOrderByID` query
- [x] Nested `product` resolver on Order (`ResolveOrderProduct`)
- [x] Use existing `OrderService` from the shared layer (`FetchByID` / `FetchAll`)

## 2.3 Integration

- [x] GraphQL endpoint at `/graphql` (already registered)
- [x] Reuse `OrderService` (same instance as REST, wired in `main.go`)
- [x] Context propagation: HTTP `r.Context()` → `gql.Params.Context` → resolver → service

---

# Phase 3: gRPC Analytics

## 3.1 Protocol Buffer Definition

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

## 3.2 Implementation

- [ ] Generate Go code from proto
- [ ] Create `AnalyticsService` in `services/`
- [ ] Add aggregate SQL queries to repo
- [ ] Implement gRPC server in `api/grpc/`
- [ ] Run on separate port (`:50051`)

## 3.3 Streaming (Optional)

- [ ] Server-side streaming for top products / low stock
- [ ] Demonstrates gRPC streaming capability

---

# Phase 4: WebSocket Real-time

## 4.1 WebSocket Architecture

**Library**: `nhooyr.io/websocket` (context-aware, modern)

**Connection Management**: Hub pattern
- `clients` — map of connections
- `broadcast` — channel for messages
- `register/unregister` — channels for connection lifecycle

## 4.2 Events

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

## 4.3 Integration

- [ ] Create `hub` struct with run loop
- [ ] HTTP upgrade handler at `/ws`
- [ ] Per-connection read/write pumps
- [ ] Emit events from service layer (channel or callback)
- [ ] Graceful disconnect handling

---

# Phase 5: Cleanup + Documentation

## 5.1 Reset Mechanism

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

## 5.2 README

**Content**:
- [ ] Overview of each API style
- [ ] REST endpoints with curl examples
- [ ] GraphQL queries examples
- [ ] gRPC `grpcurl` examples
- [ ] WebSocket client example

## 5.3 Testing Checklist

- [ ] REST: curl all endpoints
- [ ] GraphQL: queries via Postman/Insomnia
- [ ] gRPC: `grpcurl` commands
- [ ] WebSocket: client connection test

---

# Phase 6: User Authentication (TODO - implemented after Phase 5)

**Goal**: Users get a short-lived token. All requests must include token in header. Each user only sees their own products/orders.

**Implementation**: Authentication via middleware. Token validation middleware wraps protected routes.

**Data models to add**:
- `user_id` (string) — format: `US-XXXXXX` (primary key)
- `user.token` (string) — short-lived auth token
- `user.token_expires_at` (timestamp)
- `user.created_at` (timestamp)
- Add `user_id` column to products, orders tables

## 6.1 Token Generation

**Auth flow**:
- [ ] User registers (generates `user_id`)
- [ ] Server generates token (e.g., `US-XXXXXX:abcdef123456`, 7-char token)
- [ ] Token expires after configurable duration via `TOKEN_EXPIRY` env var (default: 1 hour, min: 5 minutes)
- [ ] Returns both `user_id` and `token`

## 6.2 Token Validation

**Per-request validation**:
- [ ] Extract token from `Authorization: Bearer <token>` header
- [ ] Validate token exists and not expired
- [ ] Reject requests with missing/invalid/expired token (401 Unauthorized)

## 6.3 Data Isolation

**Queries filter by user**:
- [ ] `GET /products` → returns only products where `product.user_id == token.user_id`
- [ ] `GET /orders` → returns only orders where `order.user_id == token.user_id`
- [ ] `POST /products` → creates product with `user_id` from token
- [ ] `POST /orders` → creates order with `user_id` from token

**Implementation notes**:
- [ ] Add `user_id` column to products, orders tables
- [ ] Modify service layer to accept `user_id` context
- [ ] Middleware to validate token and extract `user_id`
- [ ] Pass `user_id` through context to service/repo layer

---

# Phase 7: Perf polish (after Phases 3–6)

Do this only after REST, GraphQL, gRPC, WebSocket, cleanup/docs, and auth are in place. Not part of Phase 2.

## 7.1 GraphQL nested product: batch by IDs

**Current behavior**: When a client selects `product` under a list of orders, GraphQL runs `ResolveOrderProduct` once per order. That is **1 query for orders + N queries for products** (1+N).

**Fix (no DataLoader required)**: batch fetch by product IDs.

- [ ] Add `FetchByIDs` / `GetProductsByIDs` (`WHERE prod_id IN (...)`)
- [ ] In `GetAllOrders` / `GetOrderByID`, collect unique `product_id`s and load products in one trip
- [ ] Stash `map[productID]Product` (request context or richer source)
- [ ] Change `ResolveOrderProduct` to map lookup only (no per-order DB call)

Target shape: **1 query for orders + 1 query for products**.

---

# Architecture Principles

## Service/Repo Layer API-Agnostic

All API styles (REST, GraphQL, gRPC, WebSocket) use the **same service layer**:

```
┌────────────────────────────────────────────────────────────────┐
│  API Handlers (REST / GraphQL / gRPC / WebSocket)              │
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
- Use sentinel errors from `utils/customErrors/errors.go`
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
- Request bodies may be logged freely — toy project, no real user data (no PII/secrets policy needed)
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

## REST (`:8013` by default, configurable via `PORT`)

**Current implementation (Phase 1 complete)**
```
/products         POST, GET (list)
/products/{id}    GET, PATCH, DELETE
/orders           POST, GET (list)
/orders/{id}      GET
/graphql          POST
```

## Auth (`/auth`, same port as REST)

```
POST /auth/register   Create user, returns user_id + token
POST /auth/token     Refresh token (extends expiry)
```

> All endpoints require: `Authorization: Bearer <token>` header

## gRPC (`:50051`)

```
AnalyticsService: GetTotalSales, GetAverageOrderValue, GetTopProducts, GetLowStockProducts
```

## WebSocket (`/ws`, same port as REST)

```
WS /ws            Subscribe to: orders, payments, alerts
```