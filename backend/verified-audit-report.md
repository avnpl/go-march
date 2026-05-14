# Final Code Audit Report — go-march Backend

**Auditor:** Synthesized verdict over four independent reviews (`ring-2-6`, `deepseek`, `opus-cursor`, `minimax`), with every finding re-checked against the current code on `p1/rest-completion` @ `661c925`.
**Date:** 2026-05-14
**Scope:** All Go source, SQL migrations, and `go.mod` under `backend/`.

This document is the single source of truth. Each finding lists exact file paths and line numbers verified in-tree, the problem, its impact, the recommended fix, an improved example, and a confidence level. Findings are ordered by severity — fix them top-down.

---

# Executive Summary

## Overall Assessment

`go-march` is an early-stage learning backend with a sound layered architecture (handlers → services → repositories) and a clear intent to demonstrate five API styles sharing one service layer. The product CRUD path is mostly functional; the order path is **broken on contact with the database** because the Go `Order` model and the orders migration disagree on column names. There is a guaranteed nil-pointer panic in the order creation flow, two reachable `panic("unimplemented")` methods, plaintext storage of full card numbers, request bodies (including PANs) logged at debug, monetary equality compared in `float64`, a read-modify-write stock decrement vulnerable to overselling, and zero automated tests.

The codebase reads cleanly and follows many Go idioms (context propagation, wrapped errors, structured logging, dependency injection), but it is not deployable in its current state.

## Main Risks

1. Orders are non-functional — schema/code mismatch on three columns.
2. Server panics on any DB error during order creation (nil `txn.Rollback`).
3. PCI-DSS violation — full PAN stored, logged, and serialized in API responses.
4. Concurrent orders can oversell stock (no `FOR UPDATE`, no atomic decrement).
5. Float-equality check on `order.Amount` rejects legitimate orders.
6. Two `panic("unimplemented")` methods reachable through public interfaces.
7. Zero test coverage — none of the above would be caught by CI.
8. No request body size cap; trivial OOM DoS via `io.ReadAll`.
9. Request bodies logged at debug → PII + card numbers in `logs/app.log`.
10. Logger hardcoded to `Development = true` — DPanic panics, console encoding, file output not container-friendly.

## Production Readiness Score: **2/10**

The application would fail the very first realistic order request, would crash on any DB hiccup, and would not pass a basic security review.

## Top 10 Critical Findings (capsule)

| # | Finding | Severity |
|---|---------|----------|
| 1 | Order model ↔ migration column mismatch (`amount`/`created_at`/`card_number` vs `total_price`/`order_time` and no `card_number` column at all) | Critical |
| 2 | `defer txn.Rollback()` before the error check on `BeginTransaction` → nil-pointer panic | Critical |
| 3 | Plaintext credit card numbers stored, logged, and returned in JSON | Critical |
| 4 | `panic("unimplemented")` in `orderRepo.Delete` and `orderService.Delete` | Critical |
| 5 | Float-equality monetary comparison rejects valid orders | Critical |
| 6 | Stock decrement race condition under concurrency (overselling) | Critical |
| 7 | Zero tests in the repository | Critical |
| 8 | `io.ReadAll(r.Body)` everywhere — unbounded body read, OOM DoS | High |
| 9 | Debug-level raw request body logging (PAN/PII exposure) | High |
| 10 | Logger hardcoded to `Development = true`; `DPanic` becomes `panic` | High |

## Technical Debt Assessment

Heavy. The architecture is sound, but the implementation has correctness, security, and reliability defects that span every layer (model/migration drift, transaction handling, validation, error mapping, observability, dependency hygiene). Plan for two focused sprints: one for correctness/security blockers (findings #1–#15), one for hardening (testing, observability, rate limiting, secrets handling).

---

# Detailed Findings

---

## 1. [CRITICAL] Order model ↔ migration column drift breaks every order DB op

### Location
- `models/models.go:21-23,27` — `Amount float64 \`db:"amount"\``, `CreatedAt time.Time \`db:"created_at"\``, `CardNumber string \`db:"card_number"\``
- `migrations/002_create_orders.up.sql:5-12` — columns are `total_price`, `order_time`, **no `card_number` column**
- `repos/order_repo.go:30` — INSERT references `amount`, `created_at`, `card_number`
- `repos/order_repo.go:40,51` — `select *` for fetch

### Problem
The struct, the INSERT, and the schema disagree on three fields:

| Go struct → `db:` tag | Migration column | Status |
|---|---|---|
| `Amount` → `amount` | `total_price` | renamed |
| `CreatedAt` → `created_at` | `order_time` | renamed |
| `CardNumber` → `card_number` | *(no such column)* | missing in orders; only exists in `payments` |

The INSERT in `order_repo.Create` will fail with `column "amount" does not exist` (and similarly for the others). `FetchByID` / `FetchAll` use `select *`; sqlx will not be able to scan `total_price` into `Amount` (tag says `amount`) or `order_time` into `CreatedAt` (tag says `created_at`), so reads silently zero those fields out.

### Impact
- Every `POST /orders` returns 500.
- Every `GET /orders/:id` and `GET /orders` returns rows with `Amount=0` and zero `CreatedAt`.
- The card number cannot be persisted on `orders` even if you wanted to — it should live in `payments` per the existing schema.

### Recommendation
Decide a canonical name and align all three. Recommended direction: keep the existing migration (it already encodes payments correctly), update the Go model, and remove `CardNumber` from `Order` entirely — payments belong in their own table and the model already has a separate `payments` schema for card data.

### Improved Example
```go
// models/models.go
type Order struct {
    OrderID         string       `db:"order_id"         json:"order_id"`
    ProductID       string       `db:"product_id"       json:"product_id"`
    Quantity        int          `db:"quantity"         json:"quantity"`
    TotalPrice      float64      `db:"total_price"      json:"total_price"`
    OrderTime       time.Time    `db:"order_time"       json:"order_time"`
    Status          string       `db:"status"           json:"status"`
    ShippingAddress string       `db:"shipping_address" json:"shipping_address"`
    Notes           string       `db:"notes"            json:"notes"`
    ExpiresAt       sql.NullTime `db:"ttl_expires_at"   json:"-"`
    // CardNumber removed — write to payments table separately
}

// repos/order_repo.go
const insertOrderQuery = `insert into orders
    (order_id, product_id, quantity, total_price, order_time, status, shipping_address, notes)
    values ($1, $2, $3, $4, $5, $6, $7, $8) returning *`
```

Update `services/order_service.go:Create` to call into a `paymentRepo.Create` within the same transaction, persisting `card_last_four` only (see finding #3).

### Confidence: **High**

---

## 2. [CRITICAL] Nil-pointer panic — `defer txn.Rollback()` runs before error check

### Location
`services/order_service.go:36-37`

```go
txn, err := os.productRepo.BeginTransaction()
defer txn.Rollback()
```

### Problem
`BeginTransaction()` (`repos/product_repo.go:162-164`) calls `r.db.Beginx()`, which returns `(nil, err)` whenever the pool is exhausted, the DB is unreachable, or the OS context aborts the connection. Because `defer txn.Rollback()` is registered before `err` is checked, the deferred call dereferences a nil `*sqlx.Tx` and panics. Go's `net/http` does not recover panics on a per-request basis (it logs and continues), but every in-flight request on that goroutine is lost and the process is left in an undefined state.

Note: this is *not* the same as "defer Rollback after Commit is a bug." Calling `Rollback()` after a successful `Commit()` on a non-nil `*sqlx.Tx` returns `sql.ErrTxDone` and is harmless and idiomatic. The bug is strictly the nil-pointer case.

### Impact
Any transient database failure during order creation (pool exhaustion under load, brief network blip, CockroachDB maintenance) panics the goroutine. Combined with the lack of request body limits (finding #8) and zero rate limiting, a flood of requests during a DB blip will cascade.

### Recommendation
Check the error first, then defer.

### Improved Example
```go
txn, err := os.productRepo.BeginTransaction()
if err != nil {
    return models.Order{}, fmt.Errorf("order_service.Create: begin txn: %w", err)
}
defer func() { _ = txn.Rollback() }() // post-Commit returns ErrTxDone, harmless
```

Combine with finding #23 to also accept a `context.Context` in `BeginTransaction` so the transaction inherits the request's cancellation.

### Confidence: **High**

---

## 3. [CRITICAL] Full credit card numbers stored, logged, and returned

### Location
- `models/models.go:27` — `CardNumber string \`db:"card_number" json:"card_number"\``
- `models/models.go:50` — `CreateOrderReq.CardNumber string \`json:"card_num" validate:"required,numeric,len=16"\``
- `services/order_service.go:43` — `CardNumber: req.CardNumber` (stored on the order)
- `repos/order_repo.go:30,32` — passed to the INSERT
- `api/rest/order_handler.go:60-66` — raw body logged at debug (includes PAN)
- `api/rest/product_handler.go:67-72, 183-188` — same pattern for product handlers
- `api/graphql/handler.go:41` — full GraphQL body logged
- `migrations/003_create_payments.up.sql:6` — payments table also has a plaintext `card_number` column

### Problem
The full 16-digit Primary Account Number is accepted from clients, attached to the `Order` struct, persisted (or attempted to — see finding #1), serialized into every order JSON response (`json:"card_number"`), and written to `logs/app.log` at debug level. This violates PCI-DSS 3.4 (render PAN unreadable wherever stored) and 3.3 (mask in displays). The seed data in `003_create_payments.up.sql` even contains real-looking 16-digit PANs.

### Impact
- PCI-DSS non-compliance. In a regulated environment: $5k–$100k/month fines, loss of card-processing privileges, unlimited liability on breach.
- A single SQL injection, log leak, or backup mishandling exposes every customer's card.
- The CLAUDE.md explicitly says "Never log request bodies (may contain PII/secrets)" and this is violated everywhere.

### Recommendation
1. Remove `CardNumber` from the `Order` model and from any `orders` table column. Card data is a `payments` concern.
2. Store only `card_last_four` (already a column in `payments`).
3. Integrate a payment processor (Stripe, Braintree) and store only the tokenized reference.
4. Delete all `zap.String("body", …)` debug logs unconditionally.
5. Strip `card_number` from `payments` migration (or at minimum mask before insert) and rotate the seed data — `4111111111111111` etc. are real Visa test PANs and should be replaced with `XXXXXXXXXXXX1111`.

### Improved Example
```go
// services/order_service.go (excerpt)
lastFour := req.CardNumber[len(req.CardNumber)-4:]

// orderRepo.Create(...): no card data
// paymentRepo.Create(txn, ctx, models.Payment{
//     OrderID:      order.OrderID,
//     Amount:       order.TotalPrice,
//     CardLastFour: lastFour,
//     Status:       "pending",
// })

// Wipe the raw PAN as soon as it's been used for the gateway call:
req.CardNumber = ""
```

### Confidence: **High**

---

## 4. [CRITICAL] `panic("unimplemented")` in reachable interface methods

### Location
- `repos/order_repo.go:72-74` — `func (or orderRepo) Delete() { panic("unimplemented") }`
- `services/order_service.go:113-115` — `func (os *orderService) Delete() { panic("unimplemented") }`

### Problem
Both `OrderRepo` and `OrderService` declare `Delete()` on the interface (with no parameters and no return — itself a sign the API hasn't been thought through) and the implementations panic. The interface contract is "this method can be called and will work." Any new handler, GraphQL resolver, or test that touches `Delete()` crashes the server. The panic kills the entire goroutine.

### Impact
A future `DELETE /orders/{id}` route — exactly the kind of obvious next step in an order CRUD flow — drops the server. Code review won't catch it because the panic is one indirection away.

### Recommendation
Either drop `Delete` from both interfaces until you implement it, or stub it as an error-returning method with a real signature. Do not panic in production code paths.

### Improved Example
```go
// Option A — drop from the interface entirely (preferred while unimplemented)
type OrderRepo interface {
    Create(ctx context.Context, txn *sqlx.Tx, order models.Order) (models.Order, error)
    FetchByID(ctx context.Context, id string) (models.Order, error)
    FetchAll(ctx context.Context, limit, offset int) ([]models.Order, error)
}

// Option B — keep the signature, return an error
func (or orderRepo) Delete(ctx context.Context, id string) error {
    return fmt.Errorf("order_repo.Delete: not implemented")
}
```

### Confidence: **High**

---

## 5. [CRITICAL] Float equality for monetary amount

### Location
`services/order_service.go:60`

```go
if order.Amount != product.Price*float64(order.Quantity) {
    return models.Order{}, customErrors.IncorrectAmount
}
```

### Problem
Both `Price` and `Amount` are `float64`. IEEE-754 representation of decimals is approximate. `19.99 * 3` evaluates to `59.97000000000001`, not `59.97`. A client that correctly multiplies and sends `59.97` will be rejected with `IncorrectAmount`.

### Impact
Legitimate orders intermittently fail. Customers see a "wrong amount" error when their math was right. The failure pattern depends on the price values, so it manifests unpredictably.

### Recommendation
Store and compare cents as `int64` end-to-end (preferred), or compare with an epsilon at the boundary. Long-term, switch to a decimal library (`shopspring/decimal`) and a `DECIMAL` column type.

### Improved Example
```go
// Quick fix — epsilon comparison
const epsilon = 0.005 // half a cent
expected := product.Price * float64(order.Quantity)
if math.Abs(order.Amount-expected) > epsilon {
    return models.Order{}, customErrors.IncorrectAmount
}

// Long-term — integer cents
type Product struct {
    PriceCents int64 `db:"price_cents" json:"price_cents"`
}
```

### Confidence: **High**

---

## 6. [CRITICAL] Stock decrement race — overselling under concurrent orders

### Location
- `services/order_service.go:39-72` — read stock, check stock, write stock-quantity
- `repos/product_repo.go:151-159` — `UpdateProductStock` does `update products set stock = $1`

### Problem
The order flow is:
1. `FetchByID` reads the current stock (no `FOR UPDATE`, no advisory lock).
2. Check `product.Stock < order.Quantity`.
3. `UpdateProductStock(ctx, id, product.Stock-order.Quantity)`.

All three happen inside one transaction, but the lock acquired by a plain `SELECT` in CockroachDB does not block another concurrent transaction's `SELECT`. Two requests for the last unit both read `stock = 1`, both pass the check, both write `stock = 0`. One order goes through that should not have. With higher quantities, stock can go negative.

### Impact
E-commerce overselling. You promise inventory you don't have. Customers get charged for items that won't ship.

### Recommendation
Use a single atomic conditional UPDATE that decrements only if there is enough stock, and rely on the returned row count.

### Improved Example
```go
// repos/product_repo.go
func (r productRepo) DecrementStock(ctx context.Context, txn *sqlx.Tx, id string, qty int) (int, error) {
    const query = `update products set stock = stock - $1
                   where prod_id = $2 and stock >= $1
                   returning stock`
    var newStock int
    err := txn.GetContext(ctx, &newStock, query, qty, id)
    if errors.Is(err, sql.ErrNoRows) {
        return 0, customErrors.OutOfStock
    }
    if err != nil {
        return 0, fmt.Errorf("product_repo.DecrementStock: %w", err)
    }
    return newStock, nil
}
```

Then in `orderService.Create` drop the read-check-write pattern entirely — let the SQL be the single source of truth.

### Confidence: **High**

---

## 7. [CRITICAL] Zero tests in the entire repository

### Location
Project-wide. `find . -name '*_test.go'` returns nothing.

### Problem
No unit tests, no integration tests, no handler tests, no table-driven tests, no test infrastructure. Findings #1, #2, #5 and #6 are all defects that a single passing test would have caught.

### Impact
- Every change is a regression risk.
- No CI signal — `go test ./...` is a no-op.
- Refactors (e.g., the ID-type migration listed in `CLAUDE.md`) are blind.
- Code review cannot lean on test results.

### Recommendation
In order of priority:
1. Service-layer unit tests with mocked repos (catches business-logic bugs).
2. Handler tests via `httptest.NewRecorder` (catches HTTP contract bugs and status-code mapping).
3. Repo integration tests against a disposable CockroachDB container (catches the kind of schema/code drift in finding #1).

Use `testing/quick` or table-driven tests. Generate mocks with `gomock` or hand-roll them given the small interface surface.

### Improved Example
```go
// services/order_service_test.go
func TestOrderService_Create_OutOfStock(t *testing.T) {
    ctrl := gomock.NewController(t)
    pr := mocks.NewMockProductRepo(ctrl)
    or := mocks.NewMockOrderRepo(ctrl)
    txn := &sqlx.Tx{} // or a thin fake

    pr.EXPECT().BeginTransaction().Return(txn, nil)
    pr.EXPECT().FetchByID(txn, gomock.Any(), "PR-1").
        Return(models.Product{Stock: 0, Price: 10}, nil)

    svc := NewOrderService(or, pr, zap.NewNop())
    _, err := svc.Create(context.Background(), models.CreateOrderReq{
        ProductID: "PR-1", Quantity: 1, Amount: 10,
        CardNumber: "4111111111111111", ShippingAddress: "x",
    })
    if !errors.Is(err, customErrors.OutOfStock) {
        t.Fatalf("got %v, want OutOfStock", err)
    }
}
```

### Confidence: **High**

---

## 8. [HIGH] No request body size limit — trivial OOM DoS

### Location
- `api/rest/product_handler.go:60, 177` — `io.ReadAll(r.Body)`
- `api/rest/order_handler.go:54` — same
- `api/graphql/handler.go:36` — same

### Problem
`io.ReadAll` consumes the entire body into memory with no upper bound. The HTTP server's `ReadTimeout = 5*time.Second` is not a useful defence — a fast client can stream hundreds of MB in five seconds. A single unauthenticated request can OOM-kill the process.

### Impact
One curl with a multi-GB body crashes the server. Repeat for amplification.

### Recommendation
Wrap `r.Body` in `http.MaxBytesReader` before reading. Apply at the handler level (best for per-route limits) or as middleware (simpler).

### Improved Example
```go
const maxBodySize = 1 << 20 // 1 MB

func (h ProductHandler) createProduct(w http.ResponseWriter, r *http.Request) {
    r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
    var req models.CreateProductReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.SendJSONError(w, http.StatusBadRequest, "Invalid or oversized JSON")
        return
    }
    // ...
}
```

While there, also drop the `io.ReadAll → string → bytes.NewReader → json.Decode` round-trip — decode directly from `r.Body` once body logging is removed (finding #9).

### Confidence: **High**

---

## 9. [HIGH] Debug logging of raw request bodies (PAN/PII exposure)

### Location
- `api/rest/product_handler.go:67-72, 183-188`
- `api/rest/order_handler.go:60-66`
- `api/graphql/handler.go:41` — `h.logger.Debug("graphql body", zap.String("body", string(bodyBytes)))`

### Problem
Raw bodies, including the `CreateOrderReq.CardNumber` field and any GraphQL mutation arguments, are logged at debug level. Debug is the *default* level the codebase ships with (the `.env` sets `LOG_LEVEL=debug`), so this is hot in development. With the Development logger (finding #10) the data also lands in `logs/app.log` on disk.

### Impact
- PCI exposure (PANs in `logs/app.log`).
- PII exposure (shipping addresses, notes).
- Logs become discovery liability in a breach.
- Log aggregation systems (ELK, Datadog) index sensitive fields.

### Recommendation
Delete the body-logging blocks. If you need debug visibility, log only the *length* and selected non-sensitive fields after parsing.

### Improved Example
```go
log.Debug(ctx, h.logger, "received order",
    zap.String("prod_id", req.ProductID),
    zap.Int("quantity", req.Quantity),
    // do NOT include CardNumber, ShippingAddress, Notes
)
```

The `api/graphql/handler.go:41` call also accesses `h.logger` directly instead of `log.Debug(ctx, ...)` — it loses the request-ID context. Fix while you're there.

### Confidence: **High**

---

## 10. [HIGH] Logger hardcoded to Development mode

### Location
`utils/utils.go:48-89`, in particular `utils/utils.go:62`:

```go
loggerConfig.Development = true
```

### Problem
Development mode in zap:
- Console (non-JSON) encoding — not parseable by log aggregators.
- `DPanic` → real panic on every call. Any third-party library that calls `Logger.DPanic` crashes the process.
- Stack traces on `Error` (already controlled by `AddStacktrace`, but Development implies similar behavior).
- File output to `logs/app.log` (`utils/utils.go:77`) is ephemeral in containers and grows unbounded (no rotation).

`ENV` is read into `.env` but never consulted by `BuildLogger`.

### Impact
- Production logs cannot be ingested by Splunk/ELK/Datadog.
- `DPanic` becomes a hidden landmine.
- Disk fills up over time on long-running deployments.

### Recommendation
Gate by `ENV`. In production: JSON encoding, stdout only, `Development = false`, stacktraces only on error.

### Improved Example
```go
func BuildLogger() *zap.Logger {
    isProd := os.Getenv("ENV") == "production"

    var cfg zap.Config
    if isProd {
        cfg = zap.NewProductionConfig() // JSON, stdout, no stacktrace
    } else {
        cfg = zap.NewDevelopmentConfig()
        if err := os.MkdirAll("logs", 0o755); err == nil {
            cfg.OutputPaths = append(cfg.OutputPaths, "logs/app.log")
        }
    }
    cfg.Level = zap.NewAtomicLevelAt(parseLevel(os.Getenv("LOG_LEVEL")))
    logger, err := cfg.Build(zap.AddStacktrace(zap.ErrorLevel))
    if err != nil {
        log.Fatalf("cannot build logger: %v", err)
    }
    return logger
}
```

Drop the `zap.NewAtomicLevel()` → `level.Level()` → `zap.NewAtomicLevelAt` redundancy at `utils/utils.go:64-67` while you're there.

### Confidence: **High**

---

## 11. [HIGH] `RecordNotFound` mapped to HTTP 400 instead of 404

### Location
`utils/customErrors/errors.go:32`

```go
{RecordNotFound, http.StatusBadRequest},
```

### Problem
A missing resource is semantically a 404. 400 means the request was malformed. Clients that branch on status code (retry on 4xx, treat 404 as "doesn't exist yet") will misbehave.

### Impact
- REST contract violation.
- Retry loops will pointlessly retry 404s.
- Monitoring buckets 404s with bad-request noise.

### Recommendation
```go
{RecordNotFound, http.StatusNotFound},
```

### Confidence: **High**

---

## 12. [HIGH] `FormatValidationErrors` is buggy in three ways

### Location
`utils/validations.go:9-20`

```go
func FormatValidationErrors(err error) string {
    var message string
    for _, e := range err.(validator.ValidationErrors) {
        switch e.Tag() {
        case "required":
            message = fmt.Sprintf("%s is required", e.StructField())
        default:
            message = "Invalid Request"
        }
    }
    return message
}
```

### Problem
Three defects in twelve lines:
1. **Unchecked type assertion** — `err.(validator.ValidationErrors)` panics if the validator ever returns `*validator.InvalidValidationError` (e.g., from passing a non-struct to `validate.Struct`).
2. **Only the last error survives** — `message =` overwrites each iteration. Multi-field failures collapse to one message.
3. **Only `required` is recognised** — `gt`, `min`, `len`, `numeric`, `omitempty` (all used in `CreateOrderReq` and `CreateProductReq`) fall through to `"Invalid Request"`. Clients can't tell "quantity must be > 0" from "card number must be 16 chars."

### Impact
- Server can panic on a misconfigured validator.
- UX is terrible — users iterate one error at a time.
- The validator tags in `models.go` (`gt=0`, `min=0`, `numeric`, `len=16`) are effectively wasted.

### Recommendation
Use `errors.As`, accumulate all errors, and handle the tags actually in use.

### Improved Example
```go
func FormatValidationErrors(err error) string {
    var ve validator.ValidationErrors
    if !errors.As(err, &ve) {
        return "Invalid Request"
    }
    msgs := make([]string, 0, len(ve))
    for _, e := range ve {
        switch e.Tag() {
        case "required":
            msgs = append(msgs, fmt.Sprintf("%s is required", e.StructField()))
        case "gt":
            msgs = append(msgs, fmt.Sprintf("%s must be greater than %s", e.StructField(), e.Param()))
        case "min":
            msgs = append(msgs, fmt.Sprintf("%s must be at least %s", e.StructField(), e.Param()))
        case "len":
            msgs = append(msgs, fmt.Sprintf("%s must be exactly %s characters", e.StructField(), e.Param()))
        case "numeric":
            msgs = append(msgs, fmt.Sprintf("%s must be numeric", e.StructField()))
        default:
            msgs = append(msgs, fmt.Sprintf("%s is invalid", e.StructField()))
        }
    }
    return strings.Join(msgs, "; ")
}
```

### Confidence: **High**

---

## 13. [HIGH] `UpdateByID` cannot set `stock` or `price` to zero

### Location
`repos/product_repo.go:100-108`

```go
if p.Stock != 0 {
    fieldsToUpdate = append(fieldsToUpdate, "stock = :stock")
    args["stock"] = p.Stock
}
if p.Price != 0.0 {
    fieldsToUpdate = append(fieldsToUpdate, "price = :price")
    args["price"] = p.Price
}
```

### Problem
Treats the zero value as "not provided." But zero is a valid state for `Stock` (sold out) and arguably for `Price` (free promotion). With the current code the API accepts a `PATCH` with `stock: 0`, validation passes (`min=0,omitempty` allows zero), and the repo silently drops the field — endpoint returns 200, nothing changed.

### Impact
- Cannot mark products as out-of-stock via the API.
- Silent data loss; no error is returned.
- The handler returns the row, but the row reflects the prior `stock` value, so even client-side reconciliation looks weird.

### Recommendation
Use pointer fields in `UpdateProductReq` so the JSON decoder can distinguish "absent" from "present-and-zero." Update the repo accordingly.

### Improved Example
```go
// models/models.go
type UpdateProductReq struct {
    ProductID string   `json:"prod_id" validate:"required"`
    Name      *string  `json:"name,omitempty"`
    Price     *float64 `json:"price,omitempty" validate:"omitempty,gte=0"`
    Stock     *int     `json:"stock,omitempty" validate:"omitempty,min=0"`
}

// repos/product_repo.go
if p.Stock != nil {
    fieldsToUpdate = append(fieldsToUpdate, "stock = :stock")
    args["stock"] = *p.Stock
}
if p.Price != nil {
    fieldsToUpdate = append(fieldsToUpdate, "price = :price")
    args["price"] = *p.Price
}
```

### Confidence: **High**

---

## 14. [HIGH] `SendErrorResponse` echoes wrapped internal error chain to clients

### Location
`api/rest/helper.go:11-15`

```go
func SendErrorResponse(ctx context.Context, w http.ResponseWriter, err error) {
    if status, ok := customErrors.HTTPFor(err); ok {
        utils.SendJSONError(w, status, err.Error())
    } else {
        utils.SendInternalError(w)
    }
}
```

### Problem
`err.Error()` is the full wrap chain, e.g.
`"order_service.Create: product_repo.FetchByID: sql: no rows in result set"`.
That message ends up in the JSON `message` field of the API response. It leaks the package layout, the function being called, and SQL driver internals.

### Impact
- Information disclosure (architecture, ORM/driver, schema).
- Helps an attacker map the system and craft injection payloads.
- Looks unprofessional.

### Recommendation
Map sentinels to user-safe messages. Never expose raw `err.Error()`.

### Improved Example
```go
var userMessages = map[error]string{
    customErrors.RecordNotFound:    "The requested resource was not found",
    customErrors.OutOfStock:        "This product is currently out of stock",
    customErrors.IncorrectAmount:   "The order amount does not match the expected total",
    customErrors.FailedTransaction: "Payment processing failed",
    customErrors.InvalidRequest:    "The request was invalid",
    customErrors.InvalidHTTPMethod: "Method not allowed",
}

func SendErrorResponse(ctx context.Context, w http.ResponseWriter, err error) {
    for sentinel, msg := range userMessages {
        if errors.Is(err, sentinel) {
            status, _ := customErrors.HTTPFor(err)
            utils.SendJSONError(w, status, msg)
            return
        }
    }
    utils.SendInternalError(w)
}
```

### Confidence: **High**

---

## 15. [HIGH] Inconsistent not-found handling across product handlers

### Location
- `api/rest/product_handler.go:115-117` — `fetchProduct` calls `SendErrorResponse(ctx, w, err)` ✓ (uses sentinel registry)
- `api/rest/product_handler.go:236-239` — `deleteProduct` checks `errors.Is(err, sql.ErrNoRows)` directly
- `api/rest/product_handler.go:215-225` — `updateProduct` checks neither `RecordNotFound` nor `sql.ErrNoRows`; returns 500 for missing product
- `services/product_service.go:42-50` — `GetProductByID` converts `sql.ErrNoRows → RecordNotFound`
- `services/product_service.go:81-86` — `UpdateProduct` does **not** convert; passes wrapped `sql.ErrNoRows` through (note `repos/product_repo.go:124` does wrap it explicitly with `%w`)
- `services/product_service.go:88-93` — `DeleteProduct` does **not** convert either

### Problem
Three handlers handle "not found" three different ways. `updateProduct` returns 500 when the product doesn't exist. `deleteProduct` checks the wrong sentinel (`sql.ErrNoRows` instead of `customErrors.RecordNotFound`) — it works only because the wrap chain preserves `sql.ErrNoRows`, but it bypasses the central error registry.

### Impact
- Inconsistent client experience: same condition, three different status codes.
- Future maintainers will copy the wrong pattern.
- Centralised error mapping is undermined.

### Recommendation
1. In every service method, convert `sql.ErrNoRows` to `customErrors.RecordNotFound`.
2. In every handler, use `SendErrorResponse` and let the registry decide the status.

### Improved Example
```go
// services/product_service.go — UpdateProduct
res, err := s.repo.UpdateByID(ctx, req)
if err != nil {
    if errors.Is(err, sql.ErrNoRows) {
        return models.Product{}, customErrors.RecordNotFound
    }
    return models.Product{}, fmt.Errorf("product_service.Update: %w", err)
}

// api/rest/product_handler.go — updateProduct / deleteProduct
prod, err := h.svc.UpdateProduct(ctx, &req)
if err != nil {
    SendErrorResponse(ctx, w, err)
    return
}
```

### Confidence: **High**

---

## 16. [HIGH] `order_repo.FetchByID` returns unwrapped error and logs `%w` literal

### Location
`repos/order_repo.go:42-48`

```go
if err := or.db.GetContext(ctx, &res, query, id); err != nil {
    log.Error(ctx, or.logger, "failed to fetch order: %w", zap.Error(err))
    return models.Order{}, err
}
```

### Problem
Two issues:
1. The error is returned without wrapping, breaking the codebase's `"pkg.Method: %w"` convention. Callers can still match `sql.ErrNoRows` via `errors.Is`, but the chain has no context.
2. `"failed to fetch order: %w"` is a `fmt.Errorf` format string in a zap message slot. Zap treats the message as a literal — the log line will contain a literal `%w`.

### Impact
- Log message is garbled.
- Repo-level error context is lost in production logs.

### Recommendation
```go
if err := or.db.GetContext(ctx, &res, query, id); err != nil {
    log.Error(ctx, or.logger, "failed to fetch order", zap.String("id", id), zap.Error(err))
    return models.Order{}, fmt.Errorf("order_repo.FetchByID: %w", err)
}
```

While there, fix the trailing space in `repos/product_repo.go:157` — `"product_repo.UpdateProductStock : %w"` has a stray space before the colon.

### Confidence: **High**

---

## 17. [HIGH] No authentication, authorization, rate limiting, or CORS

### Location
`main.go` — middleware chain is just `RequestIDMiddleware`.

### Problem
Every endpoint is anonymous:
- `POST /products`, `DELETE /products/{id}` — anyone can mutate the catalog.
- `POST /orders` — anyone can place orders with arbitrary card data.
- `/graphql` — full read/write on products.

No rate limiting, so combined with finding #8 a single attacker can OOM the server or brute-force ID enumeration. No CORS, so browser frontends can't call the API at all.

### Impact
- Catalog tampering, data exfiltration, fraudulent orders.
- DoS via flood.
- Web frontend integrations blocked.

### Recommendation
- Add JWT or session middleware before any production deployment (acknowledged as Phase 7 in ROADMAP, but flag it).
- Add a token-bucket rate limiter (`golang.org/x/time/rate`) per IP or per token.
- Add a CORS middleware with explicit allowed origins (not `*` in production).

### Improved Example
```go
// utils/middleware.go
func RateLimitMiddleware(next http.Handler) http.Handler {
    limiter := rate.NewLimiter(rate.Every(time.Second/10), 20) // 10 rps, burst 20
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !limiter.Allow() {
            utils.SendJSONError(w, http.StatusTooManyRequests, "rate limited")
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### Confidence: **High**

---

## 18. [HIGH] GraphQL resolvers swallow input errors as `(nil, nil)`

### Location
- `api/graphql/resolvers.go:27-30` — `GetProductByID`
- `api/graphql/resolvers.go:74-77, 79-82` — `UpdateProduct` (input and prod_id)
- `api/graphql/resolvers.go:122-125, 127-130` — `DeleteProduct` (input and prod_id)

### Problem
Every type-assertion on `p.Args` returns `(nil, nil)` on failure. The GraphQL library interprets that as "the field resolved to null with no error." Clients see `null` and no indication of what went wrong.

### Impact
- Silent failures on malformed queries.
- Painful debugging for API consumers.
- Masks the schema-level bug that `DeleteProductInput.prod_id` is nullable (finding #19) — without the `nil, nil` swallowing, the type-assertion error would surface it.

### Recommendation
```go
idStr, ok := p.Args["id"].(string)
if !ok || idStr == "" {
    return nil, fmt.Errorf("getProductByID: 'id' must be a non-empty string")
}
```

Apply to all four resolvers.

### Confidence: **High**

---

## 19. [HIGH] `DeleteProductInput.prod_id` is not `NewNonNull`

### Location
`api/graphql/types.go:44-49`

```go
"prod_id": &graphql.InputObjectFieldConfig{
    Type:        graphql.String,
    Description: "The ID of the product to be deleted",
},
```

### Problem
`UpdateProductInput.prod_id` is `graphql.NewNonNull(graphql.String)`. `DeleteProductInput.prod_id` is plain `graphql.String`. The schema allows `deleteProduct(input: {})` which then silently no-ops (see finding #18).

### Impact
- Schema inconsistency.
- A delete mutation can be issued with no ID and the server reports success with no error.

### Recommendation
```go
"prod_id": &graphql.InputObjectFieldConfig{
    Type:        graphql.NewNonNull(graphql.String),
    Description: "The ID of the product to be deleted",
},
```

### Confidence: **High**

---

## 20. [HIGH] GraphQL handler ignores `variables` and `operationName`

### Location
`api/graphql/handler.go:43-46`

```go
var params struct {
    Query string `json:"query"`
}
```

### Problem
The GraphQL-over-HTTP spec requires the handler to read `query`, `variables`, and `operationName`. Apollo Client, Relay, and the GraphiQL Playground all send `variables` by default. This handler silently drops them. Clients are forced to interpolate values into the query string — which defeats GraphQL's parameterisation and is the closest GraphQL has to an injection vector.

### Impact
- Standard GraphQL clients incompatible.
- Pushes consumers toward unsafe query construction.

### Recommendation
```go
var params struct {
    Query         string                 `json:"query"`
    Variables     map[string]interface{} `json:"variables"`
    OperationName string                 `json:"operationName"`
}
// ...
result := gql.Do(gql.Params{
    Schema:         h.schema,
    RequestString:  params.Query,
    VariableValues: params.Variables,
    OperationName:  params.OperationName,
    Context:        r.Context(),
})
```

Also replace the three `http.Error` plain-text responses (`api/graphql/handler.go:33, 48, 54`) with `utils.SendJSONError` so the GraphQL surface returns JSON like the REST surface.

### Confidence: **High**

---

## 21. [HIGH] No bounds on `limit`; negative `limit`/`offset` accepted

### Location
- `api/rest/product_handler.go:128-152` — pagination parsing
- `api/rest/order_handler.go:118-142` — pagination parsing
- `repos/product_repo.go:65-87`, `repos/order_repo.go:50-69` — direct interpolation into SQL

### Problem
`strconv.Atoi` accepts negative values. `?limit=-1` or `?offset=-1` reaches the database and CockroachDB will error or behave unexpectedly. No upper bound on `limit` means `?limit=1000000000` loads the entire table into memory.

### Impact
- 500s on negative inputs.
- OOM on absurdly large `limit`.
- Offset-based pagination is also O(n) on large offsets — fine for now, future scale concern.

### Recommendation
```go
const maxLimit = 100
if limit < 0 || limit > maxLimit {
    utils.SendJSONError(w, http.StatusBadRequest, "limit must be between 0 and 100")
    return
}
if offset < 0 {
    utils.SendJSONError(w, http.StatusBadRequest, "offset must be non-negative")
    return
}
if limit == 0 {
    limit = 10 // default
}
```

Consider extracting a shared `parsePagination(r *http.Request) (limit, offset int, err error)` helper; both handlers repeat the logic.

### Confidence: **High**

---

## 22. [HIGH] `BeginTransaction` takes no `context.Context`

### Location
- `repos/product_repo.go:23` (interface)
- `repos/product_repo.go:162-164` (implementation: `return r.db.Beginx()`)

### Problem
The transaction cannot be cancelled by the request context. If the client disconnects mid-flight, the transaction stays open until the database-side timeout. Combined with the nil-panic above and lack of context timeouts at the repo layer, you get hung connections holding pool slots.

### Recommendation
```go
type ProductRepo interface {
    // ...
    BeginTransaction(ctx context.Context) (*sqlx.Tx, error)
}

func (r productRepo) BeginTransaction(ctx context.Context) (*sqlx.Tx, error) {
    return r.db.BeginTxx(ctx, nil)
}
```

Apply ctx-first to all transaction-aware methods:
- `repos/product_repo.go:18` — `FetchByID(txn *sqlx.Tx, ctx context.Context, ...)` should be `FetchByID(ctx, txn, ...)`.
- `repos/product_repo.go:22` — `UpdateProductStock(txn, ctx, ...)` should be `(ctx, txn, ...)`.
- `repos/order_repo.go:14` — same pattern for `Create`.

`go vet` and most linters require `ctx` first.

### Confidence: **High**

---

## 23. [HIGH] `DB_MAX_CONN_LIFETIME_SEC` defaults to 10 seconds

### Location
`utils/utils.go:102` — `lifetime := GetEnvVarInteger("DB_MAX_CONN_LIFETIME_SEC", 10, logger)`

### Problem
A 10-second max lifetime means every connection is recycled every 10 seconds. With pool max 25, that's 2.5 new TCP+TLS handshakes per second against CockroachDB Cloud (which mandates `sslmode=verify-full`). The driver also doesn't pre-warm — connection creation happens on demand.

### Impact
- Latency spikes after each cohort of connection expirations.
- Unnecessary load on the DB authentication path.
- TLS handshake CPU overhead.

### Recommendation
Default to 5–30 minutes. Add `SetConnMaxIdleTime` so idle connections still get reaped without churning hot ones.

### Improved Example
```go
db.SetConnMaxLifetime(30 * time.Minute)
db.SetConnMaxIdleTime(5 * time.Minute)
```

### Confidence: **High**

---

## 24. [HIGH] `GetEnvVarInteger` silently falls back on parse error

### Location
`utils/utils.go:33-46`

```go
res, err := strconv.ParseInt(value, 10, 64)
if err != nil {
    logger.Error("Error converting env variable to int")
    res = int64(defaultValue)
}
return int(res)
```

### Problem
- The error log doesn't include the key, the value, or the parse error.
- The function returns the default on parse failure, so a typo in `DB_MAX_OPEN_CONNS=abc` silently runs the service with default 25.

### Impact
Configuration errors mask themselves in production. The operator believes the env var is in effect; the service uses the default.

### Recommendation
Fail fast at startup for required config; accept defaults only when the variable is unset, not when it's malformed.

### Improved Example
```go
func GetEnvVarInteger(key string, defaultValue int, logger *zap.Logger) int {
    raw := os.Getenv(key)
    if raw == "" {
        logger.Warn("env var not set, using default",
            zap.String("key", key), zap.Int("default", defaultValue))
        return defaultValue
    }
    n, err := strconv.Atoi(raw)
    if err != nil {
        logger.Fatal("invalid integer in env var",
            zap.String("key", key), zap.String("value", raw), zap.Error(err))
    }
    return n
}
```

Same for `GetEnvVarString` (`utils/utils.go:22-30`) — both warn messages omit the key name.

### Confidence: **High**

---

## 25. [HIGH] `math/rand` for ID generation; no collision detection

### Location
`utils/utils.go:130-140`

```go
for range 7 {
    result.WriteByte(charSet[rand.Intn(36)])
}
```

### Problem
Two distinct concerns:
1. **Predictability.** Since Go 1.20, `math/rand`'s global source is auto-seeded from `crypto/rand`, so the IDs are not deterministic across restarts (contrary to one of the input reports). But the PRNG state is still observable; an attacker who sees enough IDs can theoretically predict the next, which matters if IDs are ever used as bearer references.
2. **Collisions without retry.** 36⁷ ≈ 78 billion combinations, but the birthday bound is ~280k IDs before a collision becomes likely. The DB has a primary-key constraint, so a collision surfaces as a 500 to the user; nothing retries.

### Impact
- Information leakage at low confidence.
- Silent ordering failures at moderate scale.

### Recommendation
Use the already-imported `github.com/rs/xid` (it's used for request IDs) for entity IDs too. It's monotonic, globally unique, lock-free, and URL-safe. Alternatively `crypto/rand` with retry-on-collision.

### Improved Example
```go
import "github.com/rs/xid"

func GenerateID(prefix string) string {
    return prefix + "-" + xid.New().String()
}
```

### Confidence: **High**

---

## 26. [MEDIUM] `panic` in unreachable receiver also breaks `defer` chains in tests

### Location
Same as #4, but consider: the `Delete()` methods have **no parameters** and **no return**. That signature is meaningless for either a repo or a service — it cannot accept an ID, cannot return an error. Even if you implement them, the contract is broken.

### Recommendation
When you remove `panic`, also fix the signature: `Delete(ctx context.Context, id string) error`.

### Confidence: **High**

---

## 27. [MEDIUM] Hardcoded `"6969"` card-rejection magic string

### Location
`services/order_service.go:65-68`

```go
cardNumEnd := req.CardNumber[len(req.CardNumber)-4:]
if cardNumEnd == "6969" {
    return models.Order{}, customErrors.FailedTransaction
}
```

### Problem
This is demo / test code left in the business path. It rejects any real card whose last four digits are `6969`. It also gives a false sense of "payment validation" — real payment validation runs through a gateway.

### Impact
- Real customer cards ending `6969` rejected.
- Confusing to future readers.

### Recommendation
Remove. Move the simulation logic behind an env flag (`PAYMENT_MODE=simulate`) if you want to keep a deterministic failure mode for learning.

### Confidence: **High**

---

## 28. [MEDIUM] `*sqlx.Tx` leaks into the `ProductRepo` public interface

### Location
- `repos/product_repo.go:18` — `FetchByID(txn *sqlx.Tx, ctx context.Context, id string)`
- `repos/product_repo.go:22` — `UpdateProductStock(txn *sqlx.Tx, ctx context.Context, id string, stock int)`
- `repos/product_repo.go:46-58` — `FetchByID` does `if txn != nil { txn.GetContext } else { r.db.GetContext }`

### Problem
The service layer has to know whether it's in a transaction and pass `txn`. Non-transactional callers pass `nil`. The branching is duplicated and the abstraction leaks. The repo interface no longer hides persistence concerns; it just decorates them.

### Impact
- Service layer is coupled to `*sqlx.Tx`.
- Every test must construct a real `*sqlx.Tx` or a fake.
- Easy to forget the branch and accidentally bypass the transaction.

### Recommendation
Use a context-based transaction binding (a `Begin/Commit/Rollback` on a `TxManager` that stores the active tx in the context, and `db()` resolves to `tx` if present else `r.db`). Or split into `ProductReader` (no tx) and `ProductWriter` (tx-aware).

### Improved Example
```go
// txn package
type ctxKey struct{}

func With(ctx context.Context, tx *sqlx.Tx) context.Context {
    return context.WithValue(ctx, ctxKey{}, tx)
}
func From(ctx context.Context, db *sqlx.DB) sqlx.ExtContext {
    if tx, ok := ctx.Value(ctxKey{}).(*sqlx.Tx); ok && tx != nil {
        return tx
    }
    return db
}

// repos/product_repo.go
func (r productRepo) FetchByID(ctx context.Context, id string) (models.Product, error) {
    var p models.Product
    if err := sqlx.GetContext(ctx, txn.From(ctx, r.db), &p, query, id); err != nil { ... }
    return p, nil
}
```

### Confidence: **High**

---

## 29. [MEDIUM] `godotenv.Load` is fatal on missing `.env`

### Location
`main.go:27-30`

```go
if err := godotenv.Load(".env"); err != nil {
    log.Fatalln("Error loading .env file...")
}
```

### Problem
Many production deployments inject env vars at the OS level (Kubernetes secrets, Docker `--env`). No `.env` file is the norm there. The current code refuses to start.

### Recommendation
Treat missing `.env` as a warning, not a fatal.

### Improved Example
```go
if err := godotenv.Load(".env"); err != nil {
    log.Printf(".env not found, relying on process environment: %v", err)
}
```

### Confidence: **High**

---

## 30. [MEDIUM] `server.Shutdown` error ignored

### Location
`main.go:83` — `server.Shutdown(ctx)`

### Problem
If shutdown times out (in-flight requests longer than 10s) the error tells you that connections were force-closed. Right now this is silently dropped — the operator can't tell whether shutdown was graceful.

### Recommendation
```go
if err := server.Shutdown(ctx); err != nil {
    logger.Error("server shutdown failed", zap.Error(err))
}
```

### Confidence: **High**

---

## 31. [MEDIUM] No health/readiness endpoint

### Location
`main.go` — no `/health` or `/ready` route registered.

### Problem
Container orchestrators (K8s liveness/readiness probes) and load balancers can't distinguish a healthy instance from a hung one or one whose DB connection has died. Without it the only signal is "request returns 500."

### Recommendation
Add a lightweight `/health` that pings the DB.

### Improved Example
```go
mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    if err := db.PingContext(r.Context()); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        _ = json.NewEncoder(w).Encode(map[string]string{"status": "db_unreachable"})
        return
    }
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
})
```

### Confidence: **High**

---

## 32. [MEDIUM] No DB migration runner; no `.down.sql` files

### Location
`migrations/` — four `.up.sql` files, no runner integration.

### Problem
Migrations exist but the application doesn't run them. A new dev must apply SQL manually. The migrations have no `.down.sql` companions, so rollback requires hand-written SQL. Drift between environments is the default.

### Recommendation
Integrate `github.com/golang-migrate/migrate/v4` and run migrations at startup (or via a `make migrate` target). Add `.down.sql` for every migration.

### Confidence: **High**

---

## 33. [MEDIUM] No FK indexes on `orders.product_id` or `payments.order_id`

### Location
- `migrations/002_create_orders.up.sql:12` — FK declared, no index.
- `migrations/003_create_payments.up.sql:11` — same.

### Problem
CockroachDB does **not** automatically create an index for the referencing side of an FK in all configurations. Joins and back-references will full-scan. Cascading deletes on the parent side are also slower.

### Recommendation
```sql
CREATE INDEX IF NOT EXISTS idx_orders_product_id   ON orders(product_id);
CREATE INDEX IF NOT EXISTS idx_payments_order_id   ON payments(order_id);
```

### Confidence: **High**

---

## 34. [MEDIUM] Order `Status` hardcoded to `"success"` at construction

### Location
`services/order_service.go:45` — `Status: "success"`

### Problem
`Status` is set to `"success"` before any validation. Currently safe because failures return early and the transaction rolls back, but the structure is misleading and brittle. Future maintainers might extract the struct construction or persist before commit.

### Recommendation
Start as `pending`, transition to `confirmed` after stock decrement and before commit. Use a package-level constant set.

### Improved Example
```go
const (
    OrderStatusPending   = "pending"
    OrderStatusConfirmed = "confirmed"
    OrderStatusFailed    = "failed"
)

order := models.Order{ /* ... */, Status: OrderStatusPending }
// after stock decrement
order.Status = OrderStatusConfirmed
res, err := os.orderRepo.Create(ctx, txn, order)
```

### Confidence: **Medium**

---

## 35. [MEDIUM] `Conflict` sentinel is never produced

### Location
- `utils/customErrors/errors.go:24` — `Conflict = errors.New("Conflict Error")`
- `api/rest/product_handler.go:92, 213` — checked in `createProduct` and `updateProduct`
- `services/product_service.go` / `repos/product_repo.go` — no code path returns `Conflict`

### Problem
Dead code. The handler check has zero chance of firing because the repo never translates pg `23505` (unique violation) into `Conflict`. Future readers assume conflict handling exists; it doesn't.

### Recommendation
Detect unique violations at the repo layer.

### Improved Example
```go
import "github.com/jackc/pgx/v5/pgconn"

func isUniqueViolation(err error) bool {
    var pgErr *pgconn.PgError
    return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// in productRepo.Create
if err != nil {
    if isUniqueViolation(err) {
        return models.Product{}, customErrors.Conflict
    }
    return models.Product{}, fmt.Errorf("product_repo.Create: %w", err)
}
```

Also add `{Conflict, http.StatusConflict}` to `errRegistry`.

### Confidence: **High**

---

## 36. [MEDIUM] No timeout on DB calls or transactions

### Location
- All repo methods rely on the request context only.
- HTTP server has `WriteTimeout: 10*time.Second` (`main.go:62`), but that's the outer bound.
- Order creation transaction (`services/order_service.go:Create`) has no per-step timeout.

### Problem
A slow query inside an active transaction holds a pool connection and a row lock. Under load this cascades — when the pool is exhausted other requests queue, the HTTP write timeout fires, but the goroutine remains until the DB returns. Combined with `DB_MAX_CONN_LIFETIME_SEC=10` (#23), the pool churns and starves.

### Recommendation
Wrap each repo call (or the transaction body) in `context.WithTimeout`.

### Improved Example
```go
func (os *orderService) Create(ctx context.Context, req models.CreateOrderReq) (models.Order, error) {
    ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()
    // ...
}
```

### Confidence: **High**

---

## 37. [MEDIUM] `select *` everywhere — fragile to schema evolution

### Location
- `repos/product_repo.go:36, 47, 66, 138`
- `repos/order_repo.go:30, 40, 51`

### Problem
`select *` couples the Go struct shape to the table shape. Adding a column (which the FK indexes from #33 will eventually need; or a `version`, `deleted_at`, audit field) breaks every read. `RETURNING *` has the same issue. It also fetches `ttl_expires_at` you don't use in responses.

### Recommendation
List columns explicitly. Builders like `squirrel` help, or just constants.

### Improved Example
```go
const productColumns = "prod_id, prod_name, price, stock, created_at, updated_at"
const fetchProduct  = "select " + productColumns + " from products where prod_id = $1"
```

### Confidence: **High**

---

## 38. [MEDIUM] `customErrors` package name violates Go conventions

### Location
`utils/customErrors/` directory and import path.

### Problem
Go package names should be lowercase, no underscores, no camelCase. `customErrors` violates this. It's also nested under `utils` for no reason — sentinel errors are imported throughout and the deep path adds noise.

### Recommendation
Rename to `apperrors` (or `errs`) and move to top level.

```
backend/
├── apperrors/
│   └── errors.go
```

Also rename the sentinels themselves to the `ErrXxx` convention with lowercase messages: `var ErrRecordNotFound = errors.New("record not found")`. Capitalised messages produce awkward wrapped errors like `"order_service.Create: Out of Stock"`.

### Confidence: **High**

---

## 39. [MEDIUM] `utils` is a grab-bag package

### Location
`utils/` contains: DB pool, HTTP helpers, ID gen, logger build, middleware, context helpers, env helpers, validation formatting, an empty `constants.go`, and `customErrors`.

### Problem
- Importing `utils` for `SendJSONError` drags in `sqlx`, `zap`, `pgx`, and more.
- `utils/log` imports `utils` for `GetRequestID`, which couples logging to the catch-all package.
- Hard to test individual concerns in isolation.

### Recommendation
Split into focused packages: `httputil`, `idgen`, `dbpool` (or `database`), `config`, `apperrors`, `ctxutil`, `validation`, `loggerbuild`. Tedious but mechanical; do it before adding tests so each test only depends on what it needs.

### Confidence: **High**

---

## 40. [MEDIUM] Receiver `os` in `orderService` shadows the `os` standard library

### Location
`services/order_service.go` — every method uses `func (os *orderService) ...`.

### Problem
`os` is not imported in this file today, but the moment someone adds `os.Getenv`, it silently resolves to the receiver. Go convention is a 1–2 letter abbreviation of the type — `s` or `svc`.

### Recommendation
`func (s *orderService) Create(...)` etc. (find/replace).

### Confidence: **High**

---

## 41. [MEDIUM] Outdated `go.uber.org/zap` and stale indirect deps

### Location
`go.mod:14` — `go.uber.org/zap v1.13.0` (released 2019; current is v1.27.x)

### Problem
Six years of bug fixes and performance improvements missed. The indirect `golang.org/x/crypto v0.46.0` is fine but several others are old. Several stale transitive deps may trip security scanners (Snyk/Dependabot).

### Recommendation
```bash
go get -u go.uber.org/zap
go get -u golang.org/x/crypto golang.org/x/net golang.org/x/text golang.org/x/sys
go mod tidy
```

### Confidence: **High**

---

## 42. [LOW] Mixed SQL keyword casing

### Location
- `repos/product_repo.go:36` — `insert into products ...`
- `repos/product_repo.go:108` — `WHERE prod_id = :prod_id RETURNING *` (uppercase mixed in)
- `repos/order_repo.go:30` — lowercase

### Problem
Inconsistency only; the CLAUDE.md prescribes lowercase. Already flagged as tech debt there.

### Recommendation
Convert all remaining uppercase keywords to lowercase. `UpdateByID` builds its query by concatenation, so be careful with the `WHERE` clause and `RETURNING *`.

### Confidence: **High**

---

## 43. [LOW] Double read of request body

### Location
- `api/rest/product_handler.go:62-74, 178-190`
- `api/rest/order_handler.go:54-68`

### Problem
Each handler reads the body fully with `io.ReadAll`, stringifies it for logging, reconstructs an `io.NopCloser(bytes.NewReader(...))`, and re-decodes. Pure overhead once body logging is removed (#9).

### Recommendation
After removing the debug log, decode straight from `r.Body` after wrapping in `MaxBytesReader`.

### Confidence: **High**

---

## 44. [LOW] `json.NewEncoder(w).Encode(...)` errors ignored

### Location
- `api/rest/product_handler.go:100, 124, 162, 219, 246`
- `api/rest/order_handler.go:91, 114, 152`

### Problem
Encode errors (client disconnect mid-write, broken pipe) are silently dropped. The function `fetchProduct` uses `_ = json.NewEncoder(w).Encode(prod)` — but the rest don't. Inconsistent.

### Recommendation
Pick one pattern. Either always `_ = ...` (silence is intentional) or log at debug.

### Improved Example
```go
if err := json.NewEncoder(w).Encode(prod); err != nil {
    log.Debug(ctx, h.logger, "failed to write response", zap.Error(err))
}
```

### Confidence: **Medium**

---

## 45. [LOW] DELETE returns 200 with body instead of 204

### Location
`api/rest/product_handler.go:244-247`

### Problem
REST convention for DELETE is `204 No Content`. The roadmap acknowledges this and notes the current behaviour as a user preference. Flag it for visibility.

### Recommendation
If you want to keep returning the deleted entity, use `200 OK` and document it as an intentional deviation. Otherwise:

```go
w.WriteHeader(http.StatusNoContent)
```

### Confidence: **High**

---

## 46. [LOW] GraphQL handler uses `http.Error` instead of JSON

### Location
`api/graphql/handler.go:33, 48, 54` — `http.Error(w, "Invalid HTTP Method", ...)`

### Problem
Returns plain-text bodies on the GraphQL endpoint while the REST endpoint returns JSON. Inconsistent for a client that expects a uniform error shape.

### Recommendation
Use `utils.SendJSONError`.

### Confidence: **High**

---

## 47. [LOW] `WithRequestID` allocates per log call

### Location
`utils/log/logging.go:11-16`

### Problem
Every `log.Info/Error/Debug/Warn` calls `logger.With(...)` which allocates a new `*zap.Logger`. Under load, GC pressure adds up. Better: create the request-scoped logger once in middleware and stash it in the context.

### Recommendation
```go
// middleware adds the logger to context
ctx = context.WithValue(ctx, loggerKey, baseLogger.With(zap.String("requestID", rid)))

// callers
log.FromContext(ctx).Info("...")
```

### Confidence: **Medium**

---

## 48. [LOW] `logs/app.log` grows unbounded

### Location
`utils/utils.go:77` — `OutputPaths = []string{"stdout", "logs/app.log"}`

### Problem
No rotation. The file grows until the disk fills up, at which point logger writes start failing silently.

### Recommendation
Either drop the file output entirely in production (let container log drivers handle aggregation) or use `gopkg.in/natefinch/lumberjack.v2`.

### Confidence: **High**

---

## 49. [LOW] Empty stub files

### Location
- `utils/constants.go` (3 lines, `const ()`)
- `api/grpc/grpc.go`, `api/soap/soap.go`, `api/admin/admin.go` (each: 1 line `package <name>`)

### Problem
Dead files that signal "incomplete." Each is one line of comment explaining the stub or it should be deleted entirely.

### Recommendation
Delete `utils/constants.go`. For the API stub packages, either delete or add a package comment:

```go
// Package grpc will host the gRPC analytics service. Stubbed; see ROADMAP Phase 4.
package grpc
```

### Confidence: **High**

---

## 50. [LOW] `.env` may contain real credentials in repo

### Location
`.env` (project root)

### Problem
The `.env` is checked-in shape; ensure it's `.gitignore`'d and that no real CockroachDB Cloud credentials have ever been committed. Several reports flagged seeing real-looking creds during review.

### Recommendation
```bash
grep -n '^\.env$' .gitignore || echo '.env' >> .gitignore
git rm --cached .env 2>/dev/null || true
cp .env .env.example  # then redact and commit .env.example
```

If real credentials ever landed in any commit, **rotate them immediately** — Git history doesn't forget.

### Confidence: **Medium** (depends on actual git history)

---

## 51. [LOW] README references `/product` (singular)

### Location
`README.md` (per other reviewers; not re-verified)

### Problem
Routes are `/products` and `/products/{id}`. Anyone copy-pasting README curl examples gets 404. Quick to fix when you next touch the doc.

### Confidence: **Medium**

---

## 52. [LOW] No CORS

### Location
`main.go` — no CORS middleware.

### Problem
Browser-based clients (local `localhost:3000` frontend during dev) get blocked. Already covered in #17 as part of the "no auth/rate/cors" bundle; listed separately for the dev-experience aspect.

### Recommendation
Add a development-only CORS middleware reading allowed origins from env.

### Confidence: **High**

---

## 53. [LOW] No `SIGQUIT` handling on shutdown

### Location
`main.go:75` — `signal.Notify(stop, os.Interrupt, syscall.SIGTERM)`

### Problem
`SIGQUIT` triggers Go's default stack-dump-and-exit behaviour. Operators sometimes send `SIGQUIT` to a frozen process to get a goroutine dump; consider whether you'd rather log+exit gracefully or take the dump.

### Recommendation
Decide intent. Either add `syscall.SIGQUIT` to the slice (graceful), or leave it (you want the goroutine dump). Just document the choice.

### Confidence: **Low**

---

# Architecture Assessment

**Score: 5/10**

**Strengths**
- Clean, conventional layered split (handler → service → repo).
- Service interfaces enable mocking once tests exist.
- Single shared `productService` between REST and GraphQL — this is exactly what the project set out to demonstrate.
- Request-ID middleware threads through context to the logger.
- `http.ServeMux` method+path patterns (Go 1.22+) used appropriately.

**Weaknesses**
- Transaction concerns leak out of the repo (#28).
- `BeginTransaction` is on `ProductRepo` — orchestrating across `OrderRepo` and `ProductRepo` via the *product* repo's transaction is awkward and surprising.
- `OrderService` depends on both `OrderRepo` and `ProductRepo` directly; no `PaymentRepo` yet despite a payments table.
- `utils` is a grab-bag (#39).
- No `Config` or `App` struct — env vars are read ad-hoc (`utils.GetEnvVarInteger` at the call site in repos and services).
- No domain layer; all behaviour is in services; that's fine for now but watch for service bloat.
- Empty stub packages (`grpc`, `soap`, `admin`) carry no value.
- No observability — no metrics, no tracing, no health probe (#31).

---

# Concurrency Assessment

**Score: 3/10**

**Strengths**
- HTTP server has `ReadTimeout`, `WriteTimeout`, `IdleTimeout`.
- DB pool is configured (max open/idle, lifetime).
- No bespoke goroutines; nothing to leak.
- Context plumbed through every layer.

**Critical issues**
- Nil-pointer panic on `defer txn.Rollback()` (#2). This is the dominant concurrency risk: any pool exhaustion under load cascades into panics.
- Stock decrement read-modify-write race (#6). Two concurrent orders for the last unit both succeed.

**Other issues**
- `BeginTransaction` doesn't accept a context (#22).
- No per-step timeouts in services or repos (#36).
- `DB_MAX_CONN_LIFETIME_SEC=10` causes connection churn under load (#23).
- No goroutine or thread cap; `debug.SetMaxThreads` not used.

---

# Security Assessment

**Score: 1/10**

**Critical**
1. Full PAN stored, logged, and serialized in JSON (#3).
2. Plaintext PAN in `payments.card_number` migration and seed data (#3).
3. No authentication or authorization (#17).
4. Unbounded request body read (#8).
5. Request bodies logged at debug (#9).

**High**
6. `SendErrorResponse` leaks internal error chain (#14).
7. No rate limiting (#17).
8. `math/rand` for IDs (#25) — predictability concern, not catastrophic, but worth fixing.
9. Negative pagination accepted (#21).
10. No request size limits or query complexity limits on GraphQL.

**Medium**
11. No CORS (#52).
12. GraphQL introspection enabled by default.
13. No CSRF on state-changing endpoints (only matters once cookies are introduced).

---

# Performance Assessment

**Score: 5/10**

**Strengths**
- Pool configured (max open/idle).
- Indexed primary keys on `prod_id`, `order_id`.
- Reasonable HTTP timeouts.
- Zap is a high-performance logger (once it's not in development mode).

**Weaknesses**
- `DB_MAX_CONN_LIFETIME_SEC=10` (#23) causes connection churn.
- `select *` everywhere (#37) over-fetches.
- Per-request env var read in `repos/product_repo.go:69` and `services/order_service.go:101`.
- Per-call `logger.With(...)` allocation (#47).
- Double body read in handlers (#43).
- No caching — every read hits CockroachDB.
- No FK indexes (#33) — joins will full-scan as tables grow.
- Offset-based pagination is O(n) at large offsets — fine now, consider cursor-based pagination later.

---

# Maintainability Assessment

**Score: 3/10**

**Strengths**
- Consistent file naming (`snake_case.go`).
- Clear import grouping (stdlib → third-party → internal) in most files.
- CLAUDE.md and ROADMAP.md provide solid project context.
- Most error wrapping uses the agreed `"pkg.Method: %w"` pattern.

**Weaknesses**
- **Zero tests** (#7) — the single biggest maintainability hole.
- Schema/code drift on orders (#1) means the source of truth is unclear.
- Inconsistent error handling across product handlers (#15).
- Dead code (`Conflict` checks #35, `utils/constants.go` #49, panic'd `Delete` methods #4).
- Inconsistent receiver patterns (value vs pointer; `os` shadowing in `orderService` #40).
- Mixed SQL casing (#42).
- `utils` is a grab-bag (#39).
- Old `zap` (#41).
- No `go.sum` rotation script; no CI config (no `.github/workflows`, no `azure-pipelines.yml`).
- Long handler functions (createProduct is ~45 lines doing decode-validate-call-respond by hand five times across the codebase).

---

# Testing Assessment

**Score: 0/10**

- No `*_test.go` files exist anywhere.
- No mocks, fixtures, helpers, or testdata.
- No CI signal — `go test ./...` is a no-op.
- The most expensive bugs in this report (#1, #2, #5, #6, #12, #13, #15) would all be caught by a single passing service-layer test each.

**Priority test targets**
1. `services/order_service.go` — the broken-by-design module.
2. `utils/customErrors/errors.go` — error → HTTP status mapping (covers #11).
3. `utils/validations.go` — error formatter (covers #12).
4. `api/rest/product_handler.go` and `order_handler.go` — handler error paths.
5. `repos/product_repo.go` integration — query correctness against a test DB (catches #1-style drift).

---

# Refactoring Priorities

| Rank | Task | Severity | Estimated Effort |
|------|------|----------|------------------|
| 1 | Align Order model ↔ migration; move card data to `payments` | Critical | 1–2 h |
| 2 | Fix `defer txn.Rollback()` nil panic; add ctx to `BeginTransaction` | Critical | 30 min |
| 3 | Remove plaintext PAN storage and all body logging | Critical | 1 h |
| 4 | Fix stock decrement to atomic conditional UPDATE | Critical | 30 min |
| 5 | Replace float equality with integer cents (or epsilon as quick fix) | Critical | 30 min – 1 d |
| 6 | Remove `panic("unimplemented")` in `Delete` methods | Critical | 15 min |
| 7 | Add service-layer table-driven tests (Create, OutOfStock, NotFound, Update zero-value) | Critical | 1–2 d |
| 8 | Add `http.MaxBytesReader` and pagination bounds in all handlers | High | 1 h |
| 9 | Gate logger by `ENV`; switch to JSON in production | High | 30 min |
| 10 | Map `RecordNotFound → 404`; unify error handling across handlers | High | 1 h |
| 11 | Rewrite `FormatValidationErrors` (type-safe, multi-error, tag-aware) | High | 30 min |
| 12 | Refactor transactions out of `ProductRepo` interface (context-bound `TxManager`) | Medium | 2–4 h |
| 13 | Split `utils` into focused packages; rename `customErrors` → `apperrors` | Medium | 2 h |
| 14 | Add health check, rate limiting, CORS, and basic auth | Medium | 1 d |
| 15 | Integrate migration runner; add `.down.sql` files; add FK indexes | Medium | 1 d |
| 16 | Replace `select *` with explicit columns; replace `math/rand` IDs with `xid` | Low | 1 h |
| 17 | Update `zap` and other deps; commit `go.sum` | Low | 30 min |

---

# Quick Wins

1. **Fix `defer txn.Rollback()` nil panic** — 2 lines (`services/order_service.go:36-37`).
2. **`RecordNotFound → 404`** — 1 line (`utils/customErrors/errors.go:32`).
3. **Delete raw-body debug logs** — ~15 lines across three handlers.
4. **Add `http.MaxBytesReader`** — 3 lines per handler.
5. **Delete `utils/constants.go`** — 1 file.
6. **Replace `"failed to fetch order: %w"`** literal with proper zap fields (`repos/order_repo.go:45`).
7. **Make GraphQL `DeleteProductInput.prod_id` non-null** — 1 line.
8. **Return errors from GraphQL resolvers instead of `nil, nil`** — ~8 lines.
9. **Use `customErrors.RecordNotFound` (not `sql.ErrNoRows`) in `deleteProduct`** — 3 lines.
10. **Remove the `"6969"` magic check** — 4 lines (`services/order_service.go:65-68`).
11. **Increase `DB_MAX_CONN_LIFETIME_SEC` default to 1800** — 1 line.
12. **`server.Shutdown(ctx)` error logged** — 3 lines.

---

# Long-Term Improvements

1. Adopt Unit-of-Work or context-bound transaction management; remove `*sqlx.Tx` from interfaces.
2. Add authentication/authorization (JWT or session) middleware (ROADMAP Phase 7).
3. Replace direct card handling with a payment processor integration (Stripe).
4. Build comprehensive test suite (unit + handler + integration) + GitHub Actions CI.
5. Switch monetary values to integer cents or a decimal library end-to-end.
6. Add Prometheus `/metrics` + OpenTelemetry traces; ship logs/metrics/traces to one backend.
7. Cache the product catalog (Redis or in-process LRU) for read-heavy endpoints.
8. Integrate `golang-migrate` and ship migrations with the binary.
9. Cursor-based pagination once tables grow.
10. Idempotency keys for `POST /orders`.
11. API versioning (`/v1/...`).
12. Structured config struct loaded at startup, replacing ad-hoc env reads.

---

# Top 5 Highest Priority Fixes

| # | Fix | Severity | Effort |
|---|-----|----------|--------|
| 1 | Align `Order` model with migration; remove `CardNumber` from orders | Critical | 1 h |
| 2 | Fix nil-pointer panic on `BeginTransaction` failure | Critical | 5 min |
| 3 | Remove plaintext PAN storage and all raw-body logging | Critical | 1 h |
| 4 | Atomic stock decrement to prevent overselling | Critical | 30 min |
| 5 | Add a basic service-layer test suite | Critical | 1–2 d |

# Top 5 Easiest Wins

| # | Fix | Effort |
|---|-----|--------|
| 1 | Fix nil `defer txn.Rollback()` | 1 line |
| 2 | `RecordNotFound → 404` | 1 line |
| 3 | Delete raw-body debug logs | ~15 lines |
| 4 | Add `http.MaxBytesReader` per handler | 3 lines each |
| 5 | Remove `"6969"` magic | 4 lines |

# Top 5 Scalability Concerns

| # | Concern | Impact |
|---|---------|--------|
| 1 | Read-modify-write stock decrement (#6) | Overselling; can't scale concurrent writes |
| 2 | `DB_MAX_CONN_LIFETIME_SEC=10` (#23) | Connection storms under load |
| 3 | No caching, every read hits CockroachDB | DB-bound at scale |
| 4 | No FK indexes (#33) | Full-scan joins as orders grow |
| 5 | No request body size or `limit` cap (#8, #21) | OOM under malicious or accidental large requests |

# Top 5 Reliability Concerns

| # | Concern | Impact |
|---|---------|--------|
| 1 | Nil-pointer panic on transaction begin (#2) | Outage on transient DB hiccup |
| 2 | `panic("unimplemented")` Delete methods (#4) | Future route silently lands a crash |
| 3 | Order model ↔ schema drift (#1) | Orders subsystem 100% broken |
| 4 | No health/readiness endpoint (#31) | Orchestrators route traffic to broken instances |
| 5 | Dev-mode logger in production (#10) | DPanic surprises; log file fills disk |

---

# Appendix — Notes on the four input reports

This audit subsumes all four input reports. For traceability:

- **opus-cursor** caught most things, including several uniquely (missing `card_number` column in orders migration, zero-value update bug, `os` receiver shadowing, ctx-less `BeginTransaction`, dev-mode DPanic semantics, internal error leak in `SendErrorResponse`, sentinel naming convention). Most accurate and most useful.
- **deepseek** matched opus-cursor on breadth and added the float-comparison bug, the stock race, the GraphQL `variables`/`operationName` gap, missing FK indexes, `RecordNotFound → 404`, and `FormatValidationErrors` keeping only the last error. Some self-corrections mid-document that should have been edited out.
- **ring-2-6** covered the criticals cleanly but missed several nuances (float comparison, stock race, missing card_number column, inconsistent handler not-found handling).
- **minimax** hit the obvious criticals (PAN, body logging, panics) but contained two technical errors: claimed `math/rand` defaults to seed 1 (false since Go 1.20) and claimed `defer Rollback()` after `Commit()` is the bug (it's idiomatic; the real bug is the nil case, which minimax missed entirely). Shortest report and least reliable; the others should be preferred for ground truth.
