# Go March Backend — Development Roadmap

> **Concept**: A JSONPlaceholder-style learning API demonstrating 5 different API styles in Go. Users can test and learn REST, GraphQL, SOAP, gRPC, and WebSocket by interacting with a simple inventory/order system.
>
> **Architecture**: Layered design (handlers → services → repos) where service and repo layers are API-agnostic. All API styles share the same business logic.

---

## Strategy

Each API style demonstrates its strengths. No duplication of CRUD across APIs.

```
Phase 1   REST Completion ────────────── complete end-to-end flow (products + orders + payments)
Phase 2   GraphQL Enhancement ─────────── orders + nested products
Phase 3   SOAP Implementation ────────── payment transactions
Phase 4   gRPC Analytics ──────────────── high-perf aggregations
Phase 5   WebSocket Real-time ──────────── notifications
Phase 6   Cleanup + Documentation ─────── reset mechanism + README
Phase 7   User Authentication ─────────── token-based auth with middleware
```

**Current status**: Product CRUD functional. Orders/payments not started.

## Progress Summary

| Phase | Status | Notes |
|-------|--------|-------|
| **Phase 1.1** | ✅ Complete | Product CRUD with routes (`/products`, `/products/{id}`) + pagination |
| **Phase 1.2-1.4** | ⬜ Not Started | Orders, payments, payment simulation |
| **Phase 2** | 🔶 Minimal | GraphQL has products only |
| **Phase 3-5** | ⬜ Not Started | SOAP, gRPC, WebSocket stubs |
| **Phase 6** | ⬜ Not Started | TTL, README |
| **Phase 7** | ⬜ TODO | User authentication with middleware (after Phase 6) |

**Legend**: ✅ Complete | 🔶 In Progress | ⬜ Not Started

---

## Data Models

### After Phase 7 (with auth)

### Product (current)
- `prod_id` (string) — format: `PR-XXXXXX` (primary key)
- `prod_name` (string)
- `price` (float64)
- `stock` (int)
- `created_at` (timestamp)
- `updated_at` (timestamp)
- `ttl_expires_at` (timestamp) — for auto-cleanup

### Order (future, Phase 1.2)
- `order_id` (string) — format: `OR-XXXXXX` (primary key)
- `user_id` (string, FK) — references `user_id` (Phase 7)
- `product_id` (string, FK) — references `prod_id`
- `quantity` (int)
- `total_price` (float64)
- `order_time` (timestamp)
- `status` (string: "pending", "paid", "failed")
- `shipping_address` (string) — updatable
- `notes` (string) — updatable
- `ttl_expires_at` (timestamp)

### Payment (future, Phase 1.3)
- `payment_id` (string) — format: `PA-XXXXXX` (primary key)
- `order_id` (string, FK) — references `order_id`
- `amount` (float64)
- `status` (string: "pending", "success", "failed")
- `card_number` (string) — stored for simulation validation
- `card_last_four` (string) — last 4 digits
- `created_at` (timestamp)
- `ttl_expires_at` (timestamp)

> **Note**: ID is generated in the service layer. Format: `PR-` for products, `OR-` for orders, `PA-` for payments. Use short random string (7 chars) after prefix.

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
- [x] **L6** Fix error response inconsistency in `UpdateProduct` handler
  - Location: `product_handler.go:143` — was `http.Error(w, err.Error(), http.StatusConflict)`
  - Issue: exposed internal error messages to client
  - Fix: changed to `utils.SendJSONError(w, http.StatusConflict, "")`
- [x] **L7** Standardize debug field naming
  - Remove spaces from field names (`"request param"` → `"id"`)
  - Remove trailing punctuation from messages (`"received ID => "` → `"received request"`)
  - Example: `product_handler.go:74` — `zap.String("request param", idStr)` → `zap.String("id", idStr)`
- [ ] **L8** Add context fields to error logs
  - Include relevant IDs/identifiers when logging errors for traceability
  - Example: `h.log.Error("failed to fetch product", zap.Error(err), zap.String("id", idStr))`
  - Currently some error logs lack context (e.g., `product_handler.go:100` — no context on FetchAll failure)

**ID generation**:
- [ ] Change `prod_id` from INT8 to STRING in database schema
- [x] All code layers use string IDs (models, repo, service, handlers, GraphQL)

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

# Phase 7: User Authentication (TODO - implemented after Phase 6)

**Goal**: Users get a short-lived token. All requests must include token in header. Each user only sees their own products/orders.

**Implementation**: Authentication via middleware. Token validation middleware wraps protected routes.

**Data models to add**:
- `user_id` (string) — format: `US-XXXXXX` (primary key)
- `user.token` (string) — short-lived auth token
- `user.token_expires_at` (timestamp)
- `user.created_at` (timestamp)
- Add `user_id` column to products, orders tables

## 7.1 Token Generation

**Auth flow**:
- [ ] User registers (generates `user_id`)
- [ ] Server generates token (e.g., `US-XXXXXX:abcdef123456`, 7-char token)
- [ ] Token expires after configurable duration via `TOKEN_EXPIRY` env var (default: 1 hour, min: 5 minutes)
- [ ] Returns both `user_id` and `token`

## 7.2 Token Validation

**Per-request validation**:
- [ ] Extract token from `Authorization: Bearer <token>` header
- [ ] Validate token exists and not expired
- [ ] Reject requests with missing/invalid/expired token (401 Unauthorized)

## 7.3 Data Isolation

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
/products         POST, GET (list)
/products/{id}    GET, PATCH, DELETE
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

## Auth (`:8080/auth`)

```
POST /auth/register   Create user, returns user_id + token
POST /auth/token     Refresh token (extends expiry)
```

> All endpoints require: `Authorization: Bearer <token>` header

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