# Go March Backend — Development Roadmap

> **Concept**: A JSONPlaceholder-style learning API demonstrating 5 different API styles in Go. Users can test and learn REST, GraphQL, SOAP, gRPC, and WebSocket by interacting with a simple inventory/order system.
>
> **Architecture**: Layered design (handlers → services → repos) where service and repo layers are API-agnostic. All API styles share the same business logic.

---

## Strategy

Each API style demonstrates its strengths. No duplication of CRUD across APIs.

```
Phase 1   REST Completion ────────────── complete end-to-end flow
Phase 2   GraphQL Enhancement ─────────── orders + nested products
Phase 3   SOAP Implementation ────────── payment transactions
Phase 4   gRPC Analytics ──────────────── high-perf aggregations
Phase 5   WebSocket Real-time ──────────── notifications
Phase 6   Cleanup + Documentation ─────── reset mechanism + README
```

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

# Phase 1: REST Completion

## 1.1 Complete Product CRUD

**Existing** (needs cleanup):
- [ ] `POST /products` — create product
- [ ] `GET /products` — list all products
- [ ] `GET /products/{id}` — get single product
- [ ] `PATCH /products/{id}` — update product
- [ ] `DELETE /products/{id}` — delete product

**Fixes needed** (from prior review):
- [ ] Fix HTTP status codes (200/201/204)
- [ ] Remove request body logging
- [ ] Add proper error handling
- [ ] Validate inputs (name required, price > 0, stock >= 0)

**ID generation**:
- [ ] Change `prod_id` from INT8 to STRING in models and repos
- [ ] Generate `PR-XXXXXX` ID in service layer on create (6-char random/alphanumeric)
- [ ] Update all product handlers/repos to use string IDs

## 1.2 Complete Orders CRUD

**Endpoints**:
- [ ] `POST /orders` — create order (decrements stock)
- [ ] `GET /orders` — list all orders
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

- [ ] `orders` query with optional status filter
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

**Mechanism**: Database TTL (CockroachDB native)

**How TTL works**:
1. Each table has `ttl_expires_at` column (timestamp)
2. CockroachDB auto-deletes rows when current time > `ttl_expires_at`
3. Sample data: `ttl_expires_at = NULL` (never expires)
4. User-inserted data: `ttl_expires_at = NOW() + 3 hours`

**Implementation**:
- [ ] Add `ttl_expires_at` column to products, orders, payments tables
- [ ] Enable TTL on tables using `ttl_expiration_expression`:
  ```sql
  ALTER TABLE products SET (ttl_expiration_expression = 'ttl_expires_at');
  ALTER TABLE orders SET (ttl_expiration_expression = 'ttl_expires_at');
  ALTER TABLE payments SET (ttl_expiration_expression = 'ttl_expires_at');
  ```
- [ ] Service layer: set `ttl_expires_at = NOW() + 3 hours` on new inserts
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

```
/products         GET, POST
/products/{id}    GET, PATCH, DELETE
/orders           GET, POST
/orders/{id}      GET, PATCH
/payments         POST
/payments/{id}    GET
/graphql          POST    # GraphQL
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