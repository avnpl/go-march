# Production-Grade Code Audit Report

**Repository:** `go-march`
**Auditor:** Opus 4 (Senior Go Engineer Review)
**Date:** 2026-05-14
**Go Version:** 1.24.5
**Files Reviewed:** 25 Go files, 4 SQL migrations, 1 go.mod, 1 .env

---

## Executive Summary

### Overall Assessment

This codebase is an early-stage learning project with a clean layered architecture (handlers → services → repos), but it contains **multiple critical production-blocking bugs** including a guaranteed nil-pointer panic, a database schema mismatch that will crash order creation at runtime, credit card numbers stored in plaintext, and zero test coverage.

### Main Risks

1. **Nil-pointer panic** in the order creation flow will crash the server on any `BeginTransaction` failure
2. **Database schema mismatch** between Go models and SQL migrations means the order system is non-functional
3. **Credit card data stored in plaintext** — PCI-DSS violation
4. **Zero test files** across the entire codebase
5. **Request body logging** of sensitive data (card numbers, PII) in production
6. **Two `panic("unimplemented")` methods** reachable via public API interfaces
7. **Floating-point arithmetic** for monetary calculations — precision errors guaranteed
8. **No authentication, rate limiting, or CORS** on any endpoint
9. **Unbounded request body reads** — trivial denial-of-service vector
10. **Development-mode logger** with DPanic-to-panic behavior deployed

### Technical Debt Assessment

The codebase has moderate structural debt (utils grab-bag, inconsistent error handling patterns between product and order flows) and severe functional debt (missing features claimed by interfaces, schema mismatches). The architecture is directionally sound but needs 2-3 focused sprints of hardening before any production traffic.

### Production Readiness Score: 2/10

Critical safety and correctness issues must be resolved before any production deployment.

---

## Detailed Findings

---

### 1. [Critical] Nil-Pointer Panic in Order Creation — `defer txn.Rollback()` Before Error Check

#### Location

`services/order_service.go`, function `Create`, lines 49-50

#### Problem

```go
txn, err := os.productRepo.BeginTransaction()
defer txn.Rollback()   // <— if err != nil, txn is nil → PANIC
```

`BeginTransaction()` can return `(nil, error)` if the database connection pool is exhausted, the DB is down, or any connection error occurs. The very next line calls `defer txn.Rollback()` **without checking `err`**. When `txn` is `nil`, the deferred `Rollback()` call dereferences a nil pointer and **panics, crashing the entire server process**.

This is not a theoretical risk — it will happen under any database connectivity issue, connection pool exhaustion, or transient network failure.

#### Impact

- **Server crash** on any DB connection failure during order creation
- **All in-flight requests killed** because Go's HTTP server does not recover panics in goroutines
- **Cascading failure** — if the DB is briefly slow, all order requests pile up, exhaust the pool, and every subsequent request panics
- In production with no process supervisor, this is **permanent downtime** until manual restart

#### Recommendation

Check the error before using the transaction. Move `defer txn.Rollback()` after the nil check.

#### Improved Example

```go
func (os *orderService) Create(ctx context.Context, req models.CreateOrderReq) (models.Order, error) {
	txn, err := os.productRepo.BeginTransaction()
	if err != nil {
		return models.Order{}, fmt.Errorf("order_service.Create: begin txn: %w", err)
	}
	defer txn.Rollback()

	// ... rest of the function
}
```

#### Confidence: **High**

---

### 2. [Critical] Database Schema Mismatch — Order Model vs Migration

#### Location

- `models/models.go`, lines 18-29 (`Order` struct)
- `migrations/002_create_orders.up.sql` (orders table DDL)
- `repos/order_repo.go`, line 30 (INSERT query)

#### Problem

The Go `Order` struct expects these columns via `db:` tags:

| Go struct field | `db` tag | SQL migration column |
|---|---|---|
| `Amount` | `amount` | `total_price` |
| `CreatedAt` | `created_at` | `order_time` |
| `CardNumber` | `card_number` | *(does not exist)* |

The `order_repo.Create` INSERT query references `amount`, `created_at`, and `card_number` — **none of which exist** in the actual migration schema. Additionally, `select *` in `FetchByID` and `FetchAll` will fail because the SQL column names don't match the struct tags.

#### Impact

- **Every order creation fails** at runtime with a SQL error (column does not exist)
- **Every order fetch fails** because `sqlx` cannot scan `total_price` into `db:"amount"`
- The entire order subsystem is non-functional against the defined schema

#### Recommendation

Align the migration with the Go model, or vice versa. One source of truth must be canonical.

#### Improved Example

Either update the migration:
```sql
CREATE TABLE IF NOT EXISTS orders (
    order_id STRING PRIMARY KEY,
    product_id STRING NOT NULL,
    quantity INT8 NOT NULL,
    amount DECIMAL(10, 2) NOT NULL,         -- was total_price
    created_at TIMESTAMPTZ NOT NULL DEFAULT now():::TIMESTAMPTZ,  -- was order_time
    status STRING DEFAULT 'pending',
    shipping_address TEXT,
    card_number STRING,                      -- was missing
    notes TEXT,
    ttl_expires_at TIMESTAMPTZ,
    CONSTRAINT orders_product_id_fkey FOREIGN KEY (product_id) REFERENCES products(prod_id)
);
```

Or update the Go struct tags to match the existing schema. The former is recommended since the Go code is the application source of truth.

#### Confidence: **High**

---

### 3. [Critical] Credit Card Numbers Stored in Plaintext

#### Location

- `models/models.go`, line 27: `CardNumber string`
- `repos/order_repo.go`, line 30: inserted directly into DB
- `services/order_service.go`, line 44: `CardNumber: req.CardNumber`
- `api/rest/order_handler.go`, lines 62-65: logged as part of raw body

#### Problem

Full 16-digit credit card numbers are:
1. Accepted via API request body
2. Stored verbatim in the `Order` struct
3. Persisted to the database in plaintext
4. Logged in the request body at debug level
5. Returned in API responses (via `json.Encode(order)`)

This violates **PCI-DSS** requirements (specifically SAQ D requirements 3.4, 3.5, 3.6 for encryption of stored cardholder data).

#### Impact

- **Legal liability** — PCI-DSS non-compliance can result in fines of $5,000–$100,000/month
- **Data breach risk** — a SQL injection or DB backup leak exposes all card numbers
- **Log exposure** — card numbers appear in `logs/app.log` in plaintext
- **API response exposure** — card numbers returned to clients who may not be the card holder

#### Recommendation

Never store full card numbers. Store only the last 4 digits for display purposes. Delegate payment processing to a PCI-compliant payment gateway (Stripe, Braintree, etc.). If this is a learning project simulating payments, use a masked field.

#### Improved Example

```go
type Order struct {
	// ...
	CardLastFour string `db:"card_last_four" json:"card_last_four"`
	// Remove: CardNumber string — never store full card numbers
}

// In service layer:
order.CardLastFour = req.CardNumber[len(req.CardNumber)-4:]
```

#### Confidence: **High**

---

### 4. [Critical] Zero Test Coverage

#### Location

Entire repository — no `*_test.go` files exist anywhere.

#### Problem

There are zero unit tests, zero integration tests, zero handler tests, and zero repository tests in the entire codebase. The `CLAUDE.md` mentions testing conventions but none are implemented.

#### Impact

- **No regression safety net** — any change can silently break existing functionality
- **No contract verification** — service interfaces have no validated behavior
- **Bugs like #1 (nil panic) and #2 (schema mismatch) would be caught by even basic tests**
- **No CI/CD pipeline value** — `go test ./...` is a no-op
- **Impossible to refactor safely** — every planned improvement (ID migration, schema alignment) is high risk

#### Recommendation

Prioritize tests in this order:
1. **Service layer unit tests** with mocked repos (catches business logic bugs)
2. **Handler tests** using `httptest.NewRecorder` (catches HTTP contract bugs)
3. **Repository integration tests** against a test database (catches schema mismatches)

#### Improved Example

```go
// services/product_service_test.go
func TestProductService_CreateProduct(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockProductRepo(ctrl)
	svc := services.NewProductService(mockRepo, zap.NewNop())

	req := &models.CreateProductReq{Name: "Test", Price: 9.99, Stock: 10}
	mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(models.Product{
		ProductID: "PR-TEST123",
		Name:      "Test",
	}, nil)

	got, err := svc.CreateProduct(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Test" {
		t.Errorf("got name %q, want %q", got.Name, "Test")
	}
}
```

#### Confidence: **High**

---

### 5. [Critical] `panic("unimplemented")` Reachable via Public Interface

#### Location

- `repos/order_repo.go`, line 72-74: `func (or orderRepo) Delete()`
- `services/order_service.go`, line 113-115: `func (os *orderService) Delete()`

#### Problem

Both the `OrderRepo` and `OrderService` interfaces expose a `Delete()` method that immediately panics with `"unimplemented"`. These methods are part of public interfaces and could be called by any new handler or integration code. A single call **crashes the server**.

#### Impact

- **Server crash** if any code path calls `Delete()` on an order
- **Interface contract violation** — callers have no way to know the method panics
- **Maintenance trap** — future developers may wire up a DELETE /orders/{id} endpoint and trigger the panic

#### Recommendation

Either remove the method from the interface entirely, or return an error instead of panicking.

#### Improved Example

```go
// Option A: Remove from interface until implemented
type OrderRepo interface {
	Create(txn *sqlx.Tx, ctx context.Context, order models.Order) (models.Order, error)
	FetchByID(ctx context.Context, id string) (models.Order, error)
	FetchAll(ctx context.Context, limit int, offset int) ([]models.Order, error)
	// Delete removed until implemented
}

// Option B: Return error
func (or orderRepo) Delete(ctx context.Context, id string) error {
	return fmt.Errorf("order_repo.Delete: %w", errors.New("not implemented"))
}
```

#### Confidence: **High**

---

### 6. [High] Request Body Logging Exposes Sensitive Data

#### Location

- `api/rest/product_handler.go`, lines 67-71 (createProduct)
- `api/rest/product_handler.go`, lines 184-188 (updateProduct)
- `api/rest/order_handler.go`, lines 61-65 (createOrder — **logs credit card numbers**)
- `api/graphql/handler.go`, line 41 (handleGraphQL)

#### Problem

Request bodies are logged at Debug level, but the content includes raw JSON which can contain:
- Credit card numbers (CreateOrderReq has `card_num`)
- Shipping addresses (PII)
- Any user-submitted data

Even at Debug level, if `LOG_LEVEL=debug` is set (which it is in the `.env`), **all this data goes to both stdout and `logs/app.log`**.

#### Impact

- **PCI-DSS violation** — card numbers in log files
- **GDPR/privacy violation** — PII in persistent logs
- **Security exposure** — log aggregation systems (ELK, Datadog) now contain sensitive data
- **Audit failure** — any security audit would flag this immediately

#### Recommendation

Remove request body logging entirely, or create a sanitization function that redacts sensitive fields. Never log raw request bodies in any environment.

#### Improved Example

```go
// Remove these blocks entirely, or if debugging is essential:
log.Debug(ctx, h.logger, "received order request",
	zap.String("prod_id", req.ProductID),
	zap.Int("quantity", req.Quantity),
	// Never log: card_num, shipping_address
)
```

#### Confidence: **High**

---

### 7. [High] Floating-Point Arithmetic for Monetary Values

#### Location

- `models/models.go`: `Price float64`, `Amount float64`
- `services/order_service.go`, line 61: `order.Amount != product.Price*float64(order.Quantity)`

#### Problem

The equality check `order.Amount != product.Price*float64(order.Quantity)` compares two `float64` values. Floating-point arithmetic is inherently imprecise for decimal values. Example:

```
Price = 19.99, Quantity = 3
Expected: 59.97
float64 result: 59.97000000000001
```

The equality check will fail for legitimate orders due to floating-point representation errors.

#### Impact

- **Legitimate orders rejected** — users submit the correct total but the float comparison fails
- **Intermittent failures** — some price×quantity combinations work, others don't, creating confusing user experiences
- **Financial reporting inaccuracies** — accumulated float errors in order amounts

#### Recommendation

Use integer cents for all monetary calculations, or use a decimal library. At minimum, use an epsilon-based comparison.

#### Improved Example

```go
// Option A: Integer cents (preferred)
type Product struct {
	PriceCents int64 `db:"price_cents" json:"price_cents"`
}

// Option B: Epsilon comparison (quick fix)
const epsilon = 0.005 // half a cent
expected := product.Price * float64(order.Quantity)
if math.Abs(order.Amount-expected) > epsilon {
	return models.Order{}, customErrors.IncorrectAmount
}
```

#### Confidence: **High**

---

### 8. [High] Zero-Value Update Bug — Cannot Set Stock or Price to Zero

#### Location

`repos/product_repo.go`, `UpdateByID`, lines 100-108

#### Problem

```go
if p.Stock != 0 {
	fieldsToUpdate = append(fieldsToUpdate, "stock = :stock")
}
if p.Price != 0.0 {
	fieldsToUpdate = append(fieldsToUpdate, "price = :price")
}
```

Zero is a valid value for `Stock` (sold out) and arguably for `Price` (free item). But these checks treat zero as "not provided," making it **impossible to set stock to 0** via the update endpoint. If a product sells out and you want to mark it as out-of-stock, the update silently ignores the field.

#### Impact

- **Cannot mark products as out-of-stock** via the API
- **Cannot set price to zero** for promotional/free items
- **Silent data loss** — the API returns 200 but the field wasn't actually updated

#### Recommendation

Use pointer types for optional fields in the update request, so `nil` means "not provided" and `*0` means "set to zero."

#### Improved Example

```go
type UpdateProductReq struct {
	ProductID string   `json:"prod_id" validate:"required"`
	Name      string   `json:"name,omitempty"`
	Price     *float64 `json:"price,omitempty" validate:"omitempty,gte=0"`
	Stock     *int     `json:"stock,omitempty" validate:"omitempty,min=0"`
}

// In repo:
if p.Price != nil {
	fieldsToUpdate = append(fieldsToUpdate, "price = :price")
	args["price"] = *p.Price
}
if p.Stock != nil {
	fieldsToUpdate = append(fieldsToUpdate, "stock = :stock")
	args["stock"] = *p.Stock
}
```

#### Confidence: **High**

---

### 9. [High] Unbounded Request Body Read — Denial of Service

#### Location

- `api/rest/product_handler.go`, line 60: `io.ReadAll(r.Body)`
- `api/rest/product_handler.go`, line 177: `io.ReadAll(r.Body)`
- `api/rest/order_handler.go`, line 54: `io.ReadAll(r.Body)`
- `api/graphql/handler.go`, line 36: `io.ReadAll(r.Body)`

#### Problem

`io.ReadAll(r.Body)` reads the **entire** request body into memory with no size limit. An attacker can send a multi-gigabyte request body, causing the server to allocate unbounded memory and eventually OOM-kill.

While `server.ReadTimeout = 5 * time.Second` provides some protection, a fast network can push hundreds of megabytes in 5 seconds.

#### Impact

- **Denial of service** — trivial to exhaust server memory with a single curl command
- **OOM kill** — OS kills the Go process, taking all connections down
- **Amplification** — each concurrent malicious request multiplies memory usage

#### Recommendation

Use `http.MaxBytesReader` to cap request body size.

#### Improved Example

```go
const maxBodySize = 1 << 20 // 1 MB

func (h ProductHandler) createProduct(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req models.CreateProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// MaxBytesReader returns a specific error type that can be checked
		utils.SendJSONError(w, http.StatusBadRequest, "Invalid or too-large request body")
		return
	}
	// ...
}
```

#### Confidence: **High**

---

### 10. [High] `RecordNotFound` Maps to HTTP 400 Instead of 404

#### Location

`utils/customErrors/errors.go`, line 32

#### Problem

```go
{RecordNotFound, http.StatusBadRequest},
```

`RecordNotFound` is mapped to `400 Bad Request` instead of `404 Not Found`. This is semantically wrong — a missing resource is a 404, not a 400. A 400 means the client's request syntax is malformed.

#### Impact

- **API contract violation** — clients cannot distinguish between "bad request format" and "resource not found"
- **Client confusion** — retry logic that retries on 400 will pointlessly retry "not found" responses
- **REST anti-pattern** — violates HTTP semantics

#### Recommendation

```go
{RecordNotFound, http.StatusNotFound}, // 404, not 400
```

#### Confidence: **High**

---

### 11. [High] Inconsistent Error Handling — `deleteProduct` Checks Wrong Error

#### Location

`api/rest/product_handler.go`, `deleteProduct`, lines 236-239

#### Problem

```go
if errors.Is(err, sql.ErrNoRows) {
	utils.SendJSONError(w, http.StatusNotFound, "Record with given ID not found")
}
```

The handler checks for `sql.ErrNoRows` directly, but the service layer (`product_service.DeleteProduct`) wraps errors with `fmt.Errorf("prod_service.Delete: %w", err)`. The `errors.Is` check works because `%w` preserves the chain. **However**, the service layer for `GetProductByID` converts `sql.ErrNoRows` into `customErrors.RecordNotFound` — but `DeleteProduct` does **not** do this conversion. This means:

- `fetchProduct` handler → checks `customErrors.RecordNotFound` via `SendErrorResponse` ✓
- `deleteProduct` handler → checks `sql.ErrNoRows` directly ✓ (works but inconsistent)
- `updateProduct` handler → checks neither `RecordNotFound` nor `sql.ErrNoRows` → returns **500** for missing products ✗

#### Impact

- **500 errors on update of non-existent products** — should be 404
- **Inconsistent client experience** — same "not found" condition returns different status codes across endpoints
- **Maintenance confusion** — developers must remember which error each handler expects

#### Recommendation

Standardize: the service layer should always convert `sql.ErrNoRows` to `customErrors.RecordNotFound`. Handlers should use `SendErrorResponse` uniformly.

#### Improved Example

```go
// services/product_service.go — DeleteProduct
func (s *productService) DeleteProduct(ctx context.Context, id string) (models.Product, error) {
	res, err := s.repo.DeleteByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Product{}, customErrors.RecordNotFound
		}
		return models.Product{}, fmt.Errorf("product_service.Delete: %w", err)
	}
	return res, nil
}

// All handlers use the same pattern:
if err != nil {
	SendErrorResponse(ctx, w, err)
	return
}
```

#### Confidence: **High**

---

### 12. [High] `FormatValidationErrors` Panics on Non-ValidationErrors

#### Location

`utils/validations.go`, line 12

#### Problem

```go
for _, e := range err.(validator.ValidationErrors) {
```

This is an **unchecked type assertion**. If `err` is not of type `validator.ValidationErrors` (e.g., it's a wrapped error or a different error type), this panics with a nil-pointer dereference.

#### Impact

- **Server crash** if validation produces an unexpected error type
- **Fragile coupling** — any change in the validator library's error types can crash the server

#### Recommendation

Use a type-switch or comma-ok assertion.

#### Improved Example

```go
func FormatValidationErrors(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return "Invalid request"
	}

	var messages []string
	for _, e := range ve {
		switch e.Tag() {
		case "required":
			messages = append(messages, fmt.Sprintf("%s is required", e.StructField()))
		case "gt":
			messages = append(messages, fmt.Sprintf("%s must be greater than %s", e.StructField(), e.Param()))
		default:
			messages = append(messages, fmt.Sprintf("%s is invalid", e.StructField()))
		}
	}
	return strings.Join(messages, "; ")
}
```

#### Confidence: **High**

---

### 13. [High] `math/rand` Used for ID Generation — Not Cryptographically Suitable

#### Location

`utils/utils.go`, `GenerateID`, lines 129-140

#### Problem

`GenerateID` uses `math/rand.Intn(36)` with a 7-character suffix from a 36-character alphabet. While Go 1.22+ seeds `math/rand` from `crypto/rand` automatically, there are two issues:

1. **Collision risk**: 36^7 ≈ 78 billion combinations sounds large, but there's no collision detection. A birthday-paradox collision becomes likely around ~280K IDs for the same prefix. The DB insert will fail with a unique constraint violation, but this error is not retried — it surfaces as a 500 error to the user.

2. **Predictability**: `math/rand`'s output is deterministic given its state. While the initial seed is cryptographic, the PRNG output can be predicted if an attacker observes enough IDs in sequence.

#### Impact

- **Silent failures** at moderate scale due to ID collisions
- **Information leakage** if IDs are used as bearer references (an attacker who sees a few IDs can predict future ones)

#### Recommendation

Use `rs/xid` (already a dependency) for ID generation — it's monotonic, globally unique, and URL-safe. Or use `crypto/rand.Text()` (Go 1.24+).

#### Improved Example

```go
import "github.com/rs/xid"

func GenerateID(prefix string) string {
	return prefix + "-" + xid.New().String()
}
```

#### Confidence: **High**

---

### 14. [High] Missing Error Wrapping in `order_repo.FetchByID`

#### Location

`repos/order_repo.go`, line 47

#### Problem

```go
return models.Order{}, err  // unwrapped!
```

Every other repo method wraps errors with `fmt.Errorf("repo_name.Method: %w", err)`, but `FetchByID` returns the raw error. This breaks the consistent error-context chain and makes debugging harder — stack traces in logs won't show which repo method failed.

More critically, the log message on line 45 uses `%w` format verb in a zap string message field:
```go
log.Error(ctx, or.logger, "failed to fetch order: %w", zap.Error(err))
```
The `%w` here is a literal string in the log message — it does nothing. It should just be a descriptive string.

#### Impact

- **Loss of error context** in production logs
- **Misleading log messages** with literal `%w` in text
- **Inconsistency** with the rest of the codebase's error handling pattern

#### Recommendation

```go
func (or orderRepo) FetchByID(ctx context.Context, id string) (models.Order, error) {
	const query = "select * from orders where order_id = $1"
	var res models.Order
	if err := or.db.GetContext(ctx, &res, query, id); err != nil {
		log.Error(ctx, or.logger, "failed to fetch order", zap.String("id", id), zap.Error(err))
		return models.Order{}, fmt.Errorf("order_repo.FetchByID: %w", err)
	}
	return res, nil
}
```

#### Confidence: **High**

---

### 15. [High] `FetchByID` Interface Violates Go Context Convention

#### Location

- `repos/product_repo.go`, line 18: `FetchByID(txn *sqlx.Tx, ctx context.Context, id string)`
- `repos/order_repo.go`, line 14: `Create(txn *sqlx.Tx, ctx context.Context, order models.Order)`
- `repos/product_repo.go`, line 22: `UpdateProductStock(txn *sqlx.Tx, ctx context.Context, id string, stock int)`

#### Problem

Go convention (enforced by `go vet`, linters, and the standard library) requires `context.Context` to be the **first parameter** of any function, named `ctx`. Placing `*sqlx.Tx` before `ctx` violates this convention.

#### Impact

- **Linter warnings** — `golangci-lint` with `contextcheck` will flag these
- **Readability** — experienced Go developers will be confused
- **Tooling friction** — context-aware tools and middleware expect ctx-first signatures

#### Recommendation

```go
type ProductRepo interface {
	FetchByID(ctx context.Context, txn *sqlx.Tx, id string) (models.Product, error)
	UpdateProductStock(ctx context.Context, txn *sqlx.Tx, id string, stock int) error
}
```

#### Confidence: **High**

---

### 16. [High] `SendErrorResponse` Leaks Internal Error Details to Clients

#### Location

`api/rest/helper.go`, line 13

#### Problem

```go
utils.SendJSONError(w, status, err.Error())
```

`err.Error()` includes the full error chain: `"order_service.Create: product_repo.FetchByID: sql: no rows in result set"`. This exposes:
- Internal package structure
- Database layer implementation details
- SQL error messages (which could reveal schema information)

#### Impact

- **Information disclosure** — attackers learn internal architecture
- **SQL injection reconnaissance** — error messages from malformed queries reveal DB structure
- **Professional impression** — internal error text in API responses is unprofessional

#### Recommendation

Map sentinel errors to user-facing messages. Never expose `err.Error()` raw.

#### Improved Example

```go
func SendErrorResponse(ctx context.Context, w http.ResponseWriter, err error) {
	if status, ok := customErrors.HTTPFor(err); ok {
		utils.SendJSONError(w, status, customErrors.UserMessage(err))
	} else {
		utils.SendInternalError(w)
	}
}

// In customErrors:
var userMessages = map[error]string{
	RecordNotFound:    "The requested resource was not found",
	OutOfStock:        "This product is currently out of stock",
	IncorrectAmount:   "The order amount does not match the expected total",
	FailedTransaction: "Payment processing failed",
}

func UserMessage(err error) string {
	for sentinel, msg := range userMessages {
		if errors.Is(err, sentinel) {
			return msg
		}
	}
	return "An unexpected error occurred"
}
```

#### Confidence: **High**

---

### 17. [High] Development Logger in Production — DPanic Becomes Panic

#### Location

`utils/utils.go`, `BuildLogger`, line 67

#### Problem

```go
loggerConfig.Development = true
```

When `Development` is `true`, zap's `DPanic` level **panics** instead of just logging. If any code path (including third-party libraries) calls `logger.DPanic(...)`, the server crashes. Additionally, development mode enables more verbose output that's not appropriate for production.

#### Impact

- **Unexpected panics** in production from DPanic calls
- **Excessive log verbosity** increasing storage costs and noise
- **Stack traces on every error** — performance overhead and log pollution

#### Recommendation

Read from environment configuration:

```go
loggerConfig.Development = (getEnvVar("ENV") == "dev")
```

#### Confidence: **High**

---

### 18. [Medium] `DB_MAX_CONN_LIFETIME_SEC=10` Is Dangerously Short

#### Location

- `backend/.env`, line 7: `DB_MAX_CONN_LIFETIME_SEC=10`
- `utils/utils.go`, line 102: `db.SetConnMaxLifetime(time.Duration(lifetime) * time.Second)`

#### Problem

A 10-second connection lifetime means every connection is destroyed and recreated every 10 seconds. For a connection pool of 25, this means ~2.5 new connections per second, each requiring a TCP handshake, TLS negotiation, and authentication against CockroachDB Cloud.

#### Impact

- **Increased latency** — connection establishment overhead on every request
- **Connection storms** — under load, all 25 connections expire near-simultaneously
- **Unnecessary load on CockroachDB** — connection creation is expensive for the DB server
- **TLS overhead** — CockroachDB Cloud uses `sslmode=verify-full`, so every new connection requires full TLS handshake

#### Recommendation

Set `DB_MAX_CONN_LIFETIME_SEC` to 300-1800 (5-30 minutes). Add `ConnMaxIdleTime` for cleanup of idle connections.

```go
db.SetConnMaxLifetime(5 * time.Minute)
db.SetConnMaxIdleTime(3 * time.Minute)
```

#### Confidence: **High**

---

### 19. [Medium] Environment Variable Read on Every Request

#### Location

`repos/product_repo.go`, `FetchAll`, line 69

#### Problem

```go
limit = utils.GetEnvVarInteger("FETCH_ALL_PRODS_DEFAULT_LIMIT", 10, r.logger)
```

`os.Getenv` is called on every `FetchAll` request to read a configuration value that never changes at runtime. While `os.Getenv` is thread-safe and fast, this is architecturally wrong — configuration should be read once at startup and injected.

The same pattern exists in `services/order_service.go`, line 102.

#### Impact

- **Configuration drift risk** — if someone sets the env var mid-process, behavior changes without restart
- **Architecture violation** — repo layer reaches into environment, bypassing dependency injection
- **Testing difficulty** — tests must manipulate env vars instead of passing config

#### Recommendation

Read configuration at startup and pass it as part of the constructor.

#### Improved Example

```go
type productRepo struct {
	db           *sqlx.DB
	logger       *zap.Logger
	defaultLimit int
}

func NewProductRepo(db *sqlx.DB, logger *zap.Logger, defaultLimit int) ProductRepo {
	return productRepo{db: db, logger: logger, defaultLimit: defaultLimit}
}
```

#### Confidence: **High**

---

### 20. [Medium] Negative Limit/Offset Accepted Without Validation

#### Location

- `api/rest/product_handler.go`, `fetchAllProducts`, lines 136-152
- `api/rest/order_handler.go`, `fetchAllOrders`, lines 126-142

#### Problem

`limit` and `offset` are parsed from query parameters using `strconv.Atoi`, which accepts negative values. A request like `GET /products?limit=-1&offset=-5` passes validation and reaches the database. CockroachDB/PostgreSQL may return an error or unexpected results for negative LIMIT/OFFSET.

#### Impact

- **Unexpected SQL behavior** — negative LIMIT may return no rows or error
- **Potential SQL injection surface** — while parameterized queries prevent injection, unexpected values can trigger edge-case DB behavior
- **API abuse** — negative offset could expose data in unintended ways

#### Recommendation

```go
if limit < 0 {
	utils.SendJSONError(w, http.StatusBadRequest, "limit must be non-negative")
	return
}
if offset < 0 {
	utils.SendJSONError(w, http.StatusBadRequest, "offset must be non-negative")
	return
}
```

#### Confidence: **High**

---

### 21. [Medium] Receiver Name `os` Shadows Standard Library Package

#### Location

`services/order_service.go`, lines 35, 49, 50, 52, 58, 61, 65, 70, 75, 80, 84, 88, 100, 102, 105, 107, 113

#### Problem

```go
func (os *orderService) Create(ctx context.Context, req models.CreateOrderReq) (models.Order, error) {
```

The receiver variable is named `os`, which shadows the `os` standard library package. While `os` is not imported in this file, this creates a maintenance trap — if someone adds `os.Getenv()` or similar in the future, it will silently resolve to the receiver instead of the package, causing confusing bugs.

#### Impact

- **Maintenance trap** — future imports of `os` package will silently break
- **Code review confusion** — `os.productRepo` reads like an OS-related call
- **Go convention violation** — receiver names should be 1-2 letter abbreviations of the type name

#### Recommendation

Use `s` or `svc` as the receiver name, consistent with `productService`.

#### Confidence: **High**

---

### 22. [Medium] No Authentication or Authorization

#### Location

`main.go` — no auth middleware registered; all routes are public.

#### Problem

Every endpoint is publicly accessible without any authentication:
- `POST /products` — anyone can create products
- `DELETE /products/{id}` — anyone can delete products
- `POST /orders` — anyone can place orders with arbitrary card numbers
- `/graphql` — full read/write access to product data

#### Impact

- **Data manipulation** — anyone can create/update/delete products
- **Financial fraud** — order creation with fake card data
- **Data exfiltration** — all product and order data readable by anyone

#### Recommendation

Implement JWT or API-key middleware before any production deployment. At minimum, separate read (public) and write (authenticated) endpoints.

#### Confidence: **High**

---

### 23. [Medium] No Rate Limiting

#### Location

`main.go` — no rate limiting middleware.

#### Problem

No rate limiting on any endpoint. Combined with the unbounded body read (Finding #9), this makes the server trivially vulnerable to DoS attacks.

#### Impact

- **Denial of service** — flood of requests exhausts connection pool, memory, or CPU
- **Database overload** — every request hits the database
- **Cost amplification** — CockroachDB Cloud charges by resource usage

#### Recommendation

Add a middleware using `golang.org/x/time/rate` or a token-bucket implementation.

#### Confidence: **High**

---

### 24. [Medium] No Health Check Endpoint

#### Location

`main.go` — no `/health` or `/readiness` route.

#### Problem

There's no health check endpoint for load balancers, container orchestrators (Kubernetes), or monitoring systems to verify the server is alive and its dependencies (database) are reachable.

#### Impact

- **Silent failures** — if the DB connection dies, the server still accepts requests and returns 500s
- **Load balancer misconfiguration** — LB can't distinguish healthy from unhealthy instances
- **Kubernetes readiness** — without a readiness probe, traffic is routed to unready pods

#### Recommendation

```go
mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
	if err := db.PingContext(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "db": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
})
```

#### Confidence: **High**

---

### 25. [Medium] Log File Grows Unbounded — No Rotation

#### Location

`utils/utils.go`, line 76: `"logs/app.log"`

#### Problem

`logs/app.log` is appended to forever with no rotation, size limit, or archival. In a long-running production process, this file will grow until it fills the disk.

#### Impact

- **Disk exhaustion** — server crashes when disk is full
- **Write failures** — once disk is full, all log writes fail, potentially causing the logger to drop messages
- **Performance degradation** — large log files slow down file I/O

#### Recommendation

Use `lumberjack` for log rotation, or rely solely on stdout and let the deployment infrastructure handle log aggregation.

```go
// Simplest fix: log only to stdout, use container log drivers
loggerConfig.OutputPaths = []string{"stdout"}
loggerConfig.ErrorOutputPaths = []string{"stderr"}
```

#### Confidence: **High**

---

### 26. [Medium] Logger Allocation Per Log Call

#### Location

`utils/log/logging.go`, `WithRequestID`, lines 11-16

#### Problem

```go
func WithRequestID(ctx context.Context, logger *zap.Logger) *zap.Logger {
	if rid := utils.GetRequestID(ctx); rid != "" {
		return logger.With(zap.String("requestID", rid))
	}
	return logger
}
```

Every `log.Info/Error/Debug/Warn` call creates a new `*zap.Logger` via `.With()`. While zap is efficient, this still allocates memory per log statement. In a high-throughput service, this creates GC pressure.

#### Impact

- **GC pressure** — thousands of logger allocations per second under load
- **Unnecessary work** — the same requestID is attached to dozens of log calls per request

#### Recommendation

Create the request-scoped logger once in middleware and store it in the context.

#### Improved Example

```go
// In middleware:
func RequestIDMiddleware(next http.Handler, baseLogger *zap.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = xid.New().String()
		}
		logger := baseLogger.With(zap.String("requestID", rid))
		ctx := context.WithValue(r.Context(), loggerKey, logger)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Usage:
func LoggerFromContext(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(loggerKey).(*zap.Logger); ok {
		return l
	}
	return zap.NewNop()
}
```

#### Confidence: **Medium**

---

### 27. [Medium] `select *` Queries Are Fragile

#### Location

- `repos/product_repo.go`, lines 36, 47, 66, 112, 138
- `repos/order_repo.go`, lines 30, 41, 52

#### Problem

All queries use `select *` or `RETURNING *`. If a column is added to the database table but not to the Go struct (or vice versa), `sqlx` scanning will fail at runtime with a column-mismatch error.

#### Impact

- **Brittle schema evolution** — any DB migration that adds a column breaks all queries
- **Performance** — fetches columns that aren't needed (e.g., `ttl_expires_at`)
- **Security** — may expose columns that shouldn't be returned (e.g., internal audit fields)

#### Recommendation

Explicitly list columns in all queries:

```sql
SELECT prod_id, prod_name, price, stock, created_at, updated_at FROM products WHERE prod_id = $1
```

#### Confidence: **High**

---

### 28. [Medium] `Conflict` Error Not Used Consistently

#### Location

- `utils/customErrors/errors.go`, line 24: `Conflict` is defined
- `api/rest/product_handler.go`, lines 91, 209: handlers check for `Conflict`
- `services/product_service.go`: no code path returns `Conflict`

#### Problem

The `Conflict` error is defined and checked in handlers, but **no service or repo method ever returns it**. The `if errors.Is(err, customErrors.Conflict)` checks in `createProduct` and `updateProduct` are dead code — they will never match.

CockroachDB returns a specific error code (`23505`) for unique constraint violations, but the repo layer doesn't translate this into `customErrors.Conflict`.

#### Impact

- **Dead code** — the Conflict check gives false confidence
- **Wrong error code** — duplicate product creation returns 500 instead of 409

#### Recommendation

Detect PostgreSQL error code 23505 in the repo layer and convert to `customErrors.Conflict`.

```go
import "github.com/jackc/pgx/v5/pgconn"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
```

#### Confidence: **High**

---

### 29. [Medium] GraphQL Handler Missing `variables` and `operationName` Support

#### Location

`api/graphql/handler.go`, lines 43-45

#### Problem

```go
var params struct {
	Query string `json:"query"`
}
```

Standard GraphQL over HTTP requires support for three fields:
- `query` (required)
- `variables` (optional, JSON object)
- `operationName` (optional, string)

The handler only parses `query`, ignoring variables and operation names. Any client sending parameterized queries (which is best practice) will silently have their variables ignored.

#### Impact

- **Client incompatibility** — standard GraphQL clients (Apollo, Relay) send variables by default
- **Security regression** — without variables, clients are forced to interpolate values into query strings, defeating GraphQL's built-in parameterization

#### Recommendation

```go
var params struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables"`
	OperationName string                 `json:"operationName"`
}

result := gql.Do(gql.Params{
	Schema:         h.schema,
	RequestString:  params.Query,
	VariableValues: params.Variables,
	OperationName:  params.OperationName,
	Context:        r.Context(),
})
```

#### Confidence: **High**

---

### 30. [Medium] Error Naming Convention Violation

#### Location

`utils/customErrors/errors.go`, lines 18-26

#### Problem

Go convention for sentinel errors is `ErrFoo`, not `Foo`:

```go
// Current
var RecordNotFound = errors.New("Record Not Found")
var OutOfStock = errors.New("Out of Stock")

// Go convention
var ErrRecordNotFound = errors.New("record not found")
var ErrOutOfStock = errors.New("out of stock")
```

Additionally, error messages should be lowercase and not capitalized (per Go conventions, since they may be composed with `fmt.Errorf("doing X: %w", err)`).

#### Impact

- **Convention violation** — confuses Go developers, fails linter checks
- **Composability** — capitalized error messages produce awkward composed errors like `"order_service.Create: Out of Stock"`

#### Recommendation

Rename all sentinel errors to `ErrXxx` pattern with lowercase messages.

#### Confidence: **High**

---

### 31. [Medium] `customErrors` Package Name Violates Go Conventions

#### Location

`utils/customErrors/` directory

#### Problem

Go package names must be lowercase without underscores or mixed case. `customErrors` uses camelCase which violates `go vet` and linter rules. Additionally, nesting it under `utils` creates a deep import path for something used everywhere.

#### Impact

- **Linter failures** — `golangci-lint` will flag this
- **Import verbosity** — `"github.com/avnpl/go-march/utils/customErrors"` is unnecessarily long
- **Convention violation** — experienced Go developers will question the package structure

#### Recommendation

Rename to `apperrors` or `errs` at the top level:

```
backend/
├── apperrors/
│   └── errors.go
```

#### Confidence: **High**

---

### 32. [Medium] `utils` Package Is a Grab-Bag — Violates Single Responsibility

#### Location

`utils/` package contains:
- Database initialization (`GetDBPoolObject`)
- HTTP response helpers (`SendJSONError`, `SendInternalError`)
- ID generation (`GenerateID`)
- Logger construction (`BuildLogger`)
- Middleware (`RequestIDMiddleware`)
- Context helpers (`GetRequestID`, `SetRequestID`)
- Configuration helpers (`GetEnvVarString`, `GetEnvVarInteger`)
- Validation formatting (`FormatValidationErrors`)
- Constants (empty)

#### Problem

This violates the single-responsibility principle and Go's convention of small, focused packages. The `utils` package has become a dumping ground for unrelated functionality.

#### Impact

- **Circular dependency risk** — `utils/log` imports `utils`, which imports `utils/customErrors`
- **Import bloat** — importing `utils` for `SendJSONError` also brings in `sqlx`, `zap`, etc.
- **Testability** — testing `GenerateID` requires pulling in database and HTTP dependencies
- **Discoverability** — developers must read the whole package to find what they need

#### Recommendation

Split into focused packages:

```
backend/
├── config/         # GetEnvVarString, GetEnvVarInteger, BuildLogger
├── database/       # GetDBPoolObject
├── httputil/       # SendJSONError, SendInternalError, RequestIDMiddleware
├── idgen/          # GenerateID
├── apperrors/      # Sentinel errors, APIError
├── ctxutil/        # GetRequestID, SetRequestID
```

#### Confidence: **High**

---

### 33. [Medium] `server.Shutdown` Error Ignored

#### Location

`main.go`, line 83

#### Problem

```go
server.Shutdown(ctx)
```

The error return from `server.Shutdown(ctx)` is discarded. If shutdown fails (e.g., context deadline exceeded with in-flight requests), the process exits without logging the failure.

#### Impact

- **Silent shutdown failure** — in-flight requests may be dropped without any log evidence
- **Debugging difficulty** — shutdown issues are invisible

#### Recommendation

```go
if err := server.Shutdown(ctx); err != nil {
	logger.Error("shutdown error", zap.Error(err))
}
```

#### Confidence: **High**

---

### 34. [Medium] No CORS Headers

#### Location

`main.go` — no CORS middleware.

#### Problem

No `Access-Control-Allow-Origin` or related headers are set. If a browser-based frontend ever calls this API, all requests will be blocked by the browser's same-origin policy.

#### Impact

- **Frontend incompatibility** — browser apps cannot call this API
- **Development friction** — local development with a separate frontend requires workarounds

#### Recommendation

Add CORS middleware for development and configure allowed origins for production.

#### Confidence: **High**

---

### 35. [Low] `FormatValidationErrors` Returns Only Last Error

#### Location

`utils/validations.go`, lines 9-20

#### Problem

The loop iterates over all validation errors but overwrites `message` on each iteration, so only the **last** error is returned. If a request has multiple validation failures (e.g., missing `name` AND invalid `price`), the user only sees one.

#### Impact

- **Poor UX** — users must fix and resubmit one error at a time
- **Increased API calls** — multiple round-trips to discover all validation errors

#### Recommendation

Collect all errors and join them (see Finding #12's improved example).

#### Confidence: **High**

---

### 36. [Low] Env Var Warning Logs Don't Include Key Name

#### Location

`utils/utils.go`, lines 27, 36

#### Problem

```go
logger.Warn("Variable not set in env")     // line 27 — which variable?
logger.Warn("Key not present in env variables") // line 36 — which key?
```

The warning messages don't include the variable name, making them useless in production logs where multiple env vars are read.

#### Impact

- **Debugging difficulty** — impossible to know which env var is missing from logs alone

#### Recommendation

```go
logger.Warn("env variable not set, using default", zap.String("key", key), zap.String("default", defaultValue))
```

#### Confidence: **High**

---

### 37. [Low] `BeginTransaction` Missing Context Parameter

#### Location

`repos/product_repo.go`, line 23, 162-164

#### Problem

```go
BeginTransaction() (*sqlx.Tx, error)
```

`BeginTransaction` doesn't accept a `context.Context`, which means the transaction cannot be cancelled by the caller's context. If the request is cancelled (client disconnect), the transaction hangs until the database timeout.

#### Recommendation

```go
BeginTransaction(ctx context.Context) (*sqlx.Tx, error)

func (r productRepo) BeginTransaction(ctx context.Context) (*sqlx.Tx, error) {
	return r.db.BeginTxx(ctx, nil)
}
```

#### Confidence: **High**

---

### 38. [Low] Double Read of Request Body (Inefficient)

#### Location

- `api/rest/product_handler.go`, lines 60-72 (createProduct)
- `api/rest/product_handler.go`, lines 177-189 (updateProduct)
- `api/rest/order_handler.go`, lines 54-66 (createOrder)

#### Problem

Each handler reads the entire body with `io.ReadAll`, converts it to string for logging, then rewraps it and reads it again with `json.NewDecoder`. This doubles the memory allocation and processing for every request.

#### Impact

- **Double memory allocation** per request
- **Unnecessary CPU** for string conversion and re-reading
- **Complexity** — the body is only logged for debugging

#### Recommendation

Since body logging should be removed anyway (Finding #6), decode directly:

```go
var req models.CreateProductReq
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
	utils.SendJSONError(w, http.StatusBadRequest, "Invalid JSON")
	return
}
```

#### Confidence: **High**

---

### 39. [Low] `json.NewEncoder(w).Encode(prod)` Errors Silently Ignored

#### Location

- `api/rest/product_handler.go`, lines 100, 162, 219, 246
- `api/rest/order_handler.go`, lines 91, 113, 152

#### Problem

Most `json.Encode` calls ignore the returned error (no `_ =` prefix on some). While JSON encoding of simple structs rarely fails, network errors (broken pipe, client disconnect) can cause write failures that go unnoticed.

#### Impact

- **Silent response failures** — client disconnects aren't logged
- **Inconsistency** — some calls use `_ = json.NewEncoder(w).Encode(...)` and some don't

#### Recommendation

At minimum, log encoding errors:

```go
if err := json.NewEncoder(w).Encode(prod); err != nil {
	log.Error(ctx, h.logger, "failed to write response", zap.Error(err))
}
```

#### Confidence: **Medium**

---

### 40. [Low] `DeleteProductInput.prod_id` Is Not NonNull

#### Location

`api/graphql/types.go`, line 44

#### Problem

```go
"prod_id": &graphql.InputObjectFieldConfig{
	Type: graphql.String,  // Should be graphql.NewNonNull(graphql.String)
}
```

The `prod_id` field in `DeleteProductInput` is optional (nullable), but deleting a product without an ID is meaningless. The resolver silently returns `nil, nil` when the ID is missing, which produces a `null` response with no error — confusing for the client.

#### Impact

- **Silent no-op** — `deleteProduct` with no ID returns success with null data
- **Schema inconsistency** — `UpdateProductInput` correctly marks `prod_id` as NonNull

#### Recommendation

```go
"prod_id": &graphql.InputObjectFieldConfig{
	Type: graphql.NewNonNull(graphql.String),
}
```

#### Confidence: **High**

---

### 41. [Low] GraphQL Resolvers Return `nil, nil` on Invalid Input

#### Location

`api/graphql/resolvers.go`, lines 29, 74, 79, 120, 126

#### Problem

```go
if !ok {
	return nil, nil
}
```

When type assertions fail (invalid/missing arguments), resolvers return `(nil, nil)` — no data and no error. The client receives a `null` field with no indication of what went wrong.

#### Impact

- **Silent failures** — clients can't distinguish "no data" from "bad request"
- **Debugging difficulty** — no error is logged or returned

#### Recommendation

Return a descriptive error:

```go
if !ok {
	return nil, fmt.Errorf("missing or invalid 'id' argument")
}
```

#### Confidence: **High**

---

### 42. [Low] Missing `Content-Type` Validation on POST/PATCH Requests

#### Location

All handlers accept POST/PATCH without verifying `Content-Type: application/json`.

#### Problem

If a client sends `Content-Type: text/plain` or `multipart/form-data`, the JSON decoder will fail with a generic error. A clear `415 Unsupported Media Type` response would be more helpful.

#### Recommendation

Add a middleware or per-handler check:

```go
if r.Header.Get("Content-Type") != "application/json" {
	utils.SendJSONError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
	return
}
```

#### Confidence: **Medium**

---

---

# Architecture Assessment

**Score: 5/10**

**Strengths:**
- Clean layered architecture: handlers → services → repos
- Interface-based service layer enables testability
- Proper use of dependency injection (no global state for core services)
- Good separation of REST and GraphQL handlers sharing the same service layer
- Sensible use of Go 1.22+ `http.ServeMux` with method+path patterns

**Weaknesses:**
- `utils` is a grab-bag package violating SRP
- `customErrors` package naming violates Go conventions
- Config is read from environment at call sites instead of being injected
- `BeginTransaction` on `ProductRepo` is architecturally misplaced — transaction orchestration is a service concern leaked into the repo interface
- No configuration struct — env vars are read ad-hoc throughout the codebase
- No shared `App` or `Server` struct to hold dependencies

---

# Concurrency Assessment

**Score: 7/10**

**Strengths:**
- No explicit goroutine usage outside the main server goroutine (no goroutine leaks possible)
- Database access through `sqlx` connection pool (thread-safe)
- Proper use of `context.Context` propagation in most places
- No shared mutable state between handlers

**Weaknesses:**
- `BeginTransaction` doesn't accept context — transactions can't be cancelled
- No context timeout on database queries (relies solely on server-level timeouts)
- `math/rand.Intn` is thread-safe in Go 1.22+ but should be documented

---

# Security Assessment

**Score: 1/10**

**Critical Issues:**
1. Credit card numbers stored in plaintext (PCI-DSS violation)
2. No authentication on any endpoint
3. No rate limiting
4. Unbounded request body reads (DoS vector)
5. Internal error messages leaked to clients
6. Request bodies with sensitive data logged to disk
7. `.env` with production DB credentials exists in repo (untracked, but risky)
8. No input sanitization beyond basic JSON parsing
9. No CORS configuration
10. GraphQL introspection enabled by default (exposes schema)

---

# Performance Assessment

**Score: 5/10**

**Strengths:**
- Connection pool properly configured (max open/idle conns)
- Proper use of prepared statements via parameterized queries
- No N+1 query patterns in current code
- Reasonable HTTP timeouts configured

**Weaknesses:**
- 10-second connection lifetime causes excessive reconnections
- `select *` fetches unnecessary columns
- Logger allocation per log call (GC pressure)
- Double request body read per handler
- Env var read per request in FetchAll
- No caching layer for frequently-read data (product catalog)

---

# Maintainability Assessment

**Score: 4/10**

**Strengths:**
- Consistent file naming (`snake_case.go`)
- Good code organization within individual files
- CLAUDE.md and ROADMAP.md provide project context
- Clean import grouping in most files

**Weaknesses:**
- Zero tests — any change is a regression risk
- Inconsistent error handling across product vs order flows
- Dead code (Conflict error checks, empty constants file)
- Two panic-unimplemented methods in public interfaces
- Inconsistent error wrapping (some wrap, some don't)
- Mixed SQL keyword casing (some uppercase, some lowercase)
- `utils` package creates tight coupling across layers

---

# Testing Assessment

**Score: 0/10**

- Zero test files
- Zero test functions
- Zero test coverage
- No test infrastructure (no mocks, no test helpers, no test database config)
- No CI/CD pipeline to enforce test requirements

---

# Refactoring Priorities

1. **Fix nil-pointer panic** in `orderService.Create` (30 minutes)
2. **Align DB schema** with Go models or vice versa (1 hour)
3. **Remove credit card storage**, store only last 4 digits (2 hours)
4. **Add basic test suite** for service layer (1-2 days)
5. **Split `utils` package** into focused packages (half day)
6. **Standardize error handling** — all services convert `sql.ErrNoRows` to `ErrRecordNotFound` (2 hours)
7. **Replace zero-value checks** with pointer types in `UpdateProductReq` (1 hour)
8. **Add request body size limits** (30 minutes)
9. **Remove request body logging** (15 minutes)
10. **Add health check endpoint** (30 minutes)

---

# Quick Wins

1. Fix `defer txn.Rollback()` nil panic — 1 line change
2. Change `RecordNotFound` mapping from 400 to 404 — 1 line change
3. Remove/gate request body debug logging — delete ~15 lines
4. Add `http.MaxBytesReader` to all handlers — ~5 lines each
5. Fix `server.Shutdown(ctx)` error handling — 3 lines
6. Make `DeleteProductInput.prod_id` NonNull — 1 line change
7. Add GraphQL `variables` and `operationName` support — ~5 lines
8. Fix `FormatValidationErrors` type assertion panic — ~3 lines
9. Increase `DB_MAX_CONN_LIFETIME_SEC` to 300 — 1 line in `.env`
10. Set `Development = false` for non-dev environments — 1 line change

---

# Long-Term Improvements

1. **Authentication & authorization middleware** (JWT/OAuth2)
2. **Payment gateway integration** (Stripe) replacing direct card handling
3. **Structured configuration** package with validation at startup
4. **Observability stack** (OpenTelemetry traces, Prometheus metrics)
5. **Database migration tooling** (golang-migrate with up/down scripts)
6. **CI/CD pipeline** with linting, testing, and security scanning
7. **API versioning** (`/v1/products`)
8. **Pagination response** format with total count and next/prev links
9. **Idempotency keys** for order creation
10. **Decimal type** for monetary values throughout

---

# Top 5 Highest Priority Fixes

| # | Finding | Severity | Effort |
|---|---------|----------|--------|
| 1 | Nil-pointer panic in `orderService.Create` (defer before error check) | Critical | 5 min |
| 2 | Database schema mismatch (Order model vs migration) | Critical | 1 hour |
| 3 | Credit card numbers stored in plaintext | Critical | 2 hours |
| 4 | Add basic test suite (at least service layer) | Critical | 1-2 days |
| 5 | Fix `FormatValidationErrors` panic on type assertion | High | 10 min |

# Top 5 Easiest Wins

| # | Finding | Impact | Effort |
|---|---------|--------|--------|
| 1 | Fix `defer txn.Rollback()` nil panic | Prevents server crash | 1 line |
| 2 | Change `RecordNotFound` → 404 | Correct HTTP semantics | 1 line |
| 3 | Add `MaxBytesReader` to handlers | Prevents DoS | 4 lines/handler |
| 4 | Log `server.Shutdown` error | Visibility | 3 lines |
| 5 | Remove body logging | Security | Delete lines |

# Top 5 Scalability Concerns

| # | Concern | Impact |
|---|---------|--------|
| 1 | 10-second `ConnMaxLifetime` causes connection storms | Latency spikes under load |
| 2 | No caching layer — every read hits the database | DB becomes bottleneck |
| 3 | `select *` queries fetch unnecessary data | Wasted I/O and bandwidth |
| 4 | ID collision risk in `GenerateID` grows with scale | Silent failures at ~280K IDs |
| 5 | No pagination metadata — clients can't efficiently paginate large datasets | Poor UX, full table scans |

# Top 5 Reliability Concerns

| # | Concern | Impact |
|---|---------|--------|
| 1 | Nil-pointer panic crashes server on DB failure | Total downtime |
| 2 | `panic("unimplemented")` reachable via public API | Server crash |
| 3 | No health check — silent database disconnections | Requests fail with 500 |
| 4 | Development-mode logger — DPanic causes panic | Unexpected crashes |
| 5 | Unbounded log file — disk exhaustion | Process killed by OS |
