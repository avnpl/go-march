# Code Audit Report: go-march Backend

---

# Executive Summary

## Overall Codebase Quality Assessment

**Grade: C+ (Functional but Production-Unsafe)**

The codebase demonstrates solid architectural intent — a clean layered design (handlers → services → repos) with 5 API styles planned. The REST product CRUD works, GraphQL endpoints are partially implemented, and order creation exists but is critically broken. The code follows many Go conventions (context propagation, wrapper errors, zap logging) but has several production blockers that must be resolved before deployment.

## Main Risks

1. **Data corruption / insertion failure** — Column name mismatch between Go models and database schema makes order creation non-functional
2. **Panic on startup** — Nil transaction dereference if `BeginTransaction` fails
3. **Security** — Card numbers stored in plaintext, no rate limiting, no CORS, debug logging of raw request bodies
4. **Zero test coverage** — No tests exist for any package
5. **Multiple silent failures** — GraphQL resolvers return `(nil, nil)` on parse failures, making errors invisible

## Top 10 Critical Findings

| # | Finding | Severity | File |
|---|---------|----------|------|
| 1 | Column name mismatch: `amount` vs `total_price` breaks all order DB ops | Critical | `models/models.go`, `repos/order_repo.go`, `migrations/002_create_orders.up.sql` |
| 2 | Nil pointer dereference: `defer txn.Rollback()` when `BeginTransaction` fails | Critical | `services/order_service.go:50` |
| 3 | Zero tests in the entire repository | Critical | (project-wide) |
| 4 | GraphQL resolvers silently swallow nil input parse failures | High | `api/graphql/resolvers.go` |
| 5 | Logger always uses Development config (colored, file output) — no production mode | High | `utils/utils.go:48-87` |
| 6 | Card numbers stored in plaintext in DB and memory | High | `models/models.go:26` |
| 7 | No rate limiting or request body size limits | High | (project-wide) |
| 8 | `io.ReadAll` with no body size limit enables DoS | High | `api/rest/product_handler.go:60`, `api/rest/order_handler.go:54` |
| 9 | Debug logging of raw request bodies (PII/secrets exposure risk) | Medium | `api/rest/product_handler.go:68-71`, `api/rest/order_handler.go:62-65` |
| 10 | `panic("unimplemented")` in reachable `Delete()` methods | Medium | `repos/order_repo.go:72`, `services/order_service.go:113` |

## Technical Debt Assessment

**Heavy.** The project is in early development with significant infrastructure gaps: no tests, broken order persistence, incomplete error handling, no observability integration, no security hardening, and several API design inconsistencies. The code is well-structured for a learning project but not deployable to production without significant remediation.

## Readiness for Production Score: 2/10

The application would fail under any realistic load or security audit. Core data paths are broken, there is zero automated testing, no security controls, and the logging configuration is development-only.

---

# Detailed Findings

---

## [CRITICAL] Column Name Mismatch Breaks All Order Database Operations

### Location

- `models/models.go:22` — `Amount float64 \`db:"amount"\``
- `migrations/002_create_orders.up.sql:4` — `total_price DECIMAL(10, 2) NOT NULL`
- `repos/order_repo.go:30` — INSERT query references column `amount`

### Problem

The Go model `Order.Amount` has the struct tag `db:"amount"`, but the database column created by the migration is named `total_price`. The INSERT statement in `order_repo.go` references `amount` as a column name:

```sql
INSERT INTO orders (order_id, product_id, quantity, amount, ...) VALUES (...)
```

CockroachDB will reject this INSERT because the column `amount` does not exist. The `FetchByID` and `FetchAll` queries use `SELECT *`, but sqlx will not map the `total_price` column to `Amount` because the tag says `db:"amount"`, so `Amount` will always scan as `0`.

**Impact**: The entire order creation flow returns a 500 error on every attempt. Order reads silently return zero amounts. This is a **complete data-path failure** for all order operations.

### Recommendation

Align the model, migration, and queries. Either rename the DB column to `amount` or change the Go tag to `db:"total_price"`.

### Improved Example

```go
// models/models.go — align with migration
type Order struct {
    OrderID         string       `db:"order_id" json:"order_id"`
    ProductID       string       `db:"product_id" json:"product_id"`
    Quantity        int          `db:"quantity" json:"quantity"`
    Amount          float64      `db:"total_price" json:"amount"`
    // ...
}
```

And update the INSERT query:

```go
const insertOrderQuery = `
    INSERT INTO orders (order_id, product_id, quantity, total_price, created_at, status, shipping_address, card_number, notes)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING *`
```

Alternatively, rename the DB column by updating migration `002` and adding a corrective migration.

---

## [CRITICAL] Panic on Nil Transaction When BeginTransaction Fails

### Location

`services/order_service.go:49-50`

```go
txn, err := os.productRepo.BeginTransaction()
defer txn.Rollback()
```

### Problem

If `BeginTransaction()` returns a non-nil error, `txn` will be `nil`. The deferred `txn.Rollback()` will then dereference a nil pointer and **panic**, crashing the server. This is a guaranteed panic path — database connection failures, pool exhaustion, and CockroachDB maintenance windows will all trigger it.

### Impact

Any transient database error during order creation causes the **entire process to crash**, not just the request. This is the worst possible failure mode for a production service.

### Recommendation

Check `err` before deferring:

```go
txn, err := os.productRepo.BeginTransaction()
if err != nil {
    return models.Order{}, fmt.Errorf("order_service.Create: begin txn: %w", err)
}
defer txn.Rollback()
```

---

## [CRITICAL] Zero Tests in the Entire Repository

### Location

Project-wide. The glob `**/*_test.go` returned zero files.

### Problem

There is no automated test coverage whatsoever. No unit tests, no integration tests, no handler tests. Every bug ships undetected. No regression safety net for refactoring. A CI pipeline would have nothing to run.

### Impact

- Bugs (like the column name mismatch above) go undetected indefinitely
- Refactoring is high-risk with no regression guard
- Code reviews cannot be validated by test results
- Production incidents can only be caught via manual testing or user reports

### Recommendation

Start with table-driven unit tests for:
1. Service layer (mock the repo interfaces)
2. Validation error formatting
3. ID generation uniqueness/format
4. Error mapping (customErrors → HTTP status codes)

Then add handler-level tests with `httptest.NewRecorder()` and integration tests against a test database.

---

## [HIGH] GraphQL Resolvers Silently Swallow Nil Input Parse Failures

### Location

`api/graphql/resolvers.go` — all four resolver methods (lines 27-29, 72-74, 77-79, 119-121)

### Problem

When type assertions on input arguments fail, resolvers return `(nil, nil)`:

```go
input, ok := p.Args["input"].(map[string]interface{})
if !ok {
    return nil, nil  // silent null response, no error
}
```

The GraphQL library interprets `(nil, nil)` as "field resolved to null" — the client gets a `null` response with no error. A malformed or missing input object is silently consumed.

### Impact

Clients receive silent null responses for malformed queries instead of actionable error messages. This makes debugging difficult and masks real client bugs.

### Recommendation

```go
if !ok {
    return nil, errors.New("missing or invalid 'input' argument")
}
```

---

## [HIGH] Logger Hardcoded to Development Mode

### Location

`utils/utils.go:48-87`, specifically line 67: `loggerConfig.Development = true`

### Problem

Development mode produces colored, human-readable output to both `stdout` and `logs/app.log`. In production, you typically want:
- JSON structured logging (for log aggregators)
- Stdout only (container-friendly)
- No stack traces on non-error levels
- Performance-optimized encoding (not caller info on every line)

The `ENV` variable is read in `.env` but never used to configure the logger.

### Impact

- Log aggregation systems (ELK, Datadog, etc.) cannot parse colored console output
- File-based logging is lost in containerized/pod-based deployments (ephemeral storage)
- Performance overhead from development-mode encoding
- Stack traces on every log level (not just errors) adds noise

### Recommendation

```go
isProduction := os.Getenv("ENV") == "production"
if isProduction {
    loggerConfig.Encoding = "json"
    loggerConfig.OutputPaths = []string{"stdout"}
    loggerConfig.Development = false
    loggerConfig.DisableStacktrace = true
}
```

---

## [HIGH] Card Numbers Stored in Plaintext

### Location

`models/models.go:26` — `CardNumber string`, `repos/order_repo.go:30` — inserts raw card numbers

### Problem

Credit card numbers are stored in plaintext in the database. The `CardNumber` field in `CreateOrderReq` is accepted directly from the client and persisted without any encryption or tokenization.

### Impact

If the database is ever breached, all card numbers are immediately exposed. This likely violates PCI-DSS compliance requirements. Even in a learning project, this pattern should not be practiced.

### Recommendation

- **Minimum**: Only store the last 4 digits (already present in the `payments` table as `card_last_four`). Do not store full card numbers.
- **Better**: Use a payment gateway token (Stripe, etc.) and store only the token.
- Remove `CardNumber` from the `orders` table entirely or encrypt it at rest.

---

## [HIGH] No Request Body Size Limit (DoS Vector)

### Location

`api/rest/product_handler.go:60`, `api/rest/order_handler.go:54`, `api/graphql/handler.go:36`

```go
bodyBytes, err := io.ReadAll(r.Body)
```

### Problem

`io.ReadAll` reads the entire request body into memory with no size limit. An attacker can send a multi-GB body to exhaust server memory (OOM → crash or OOM-kill).

### Impact

Single-request denial of service. An unauthenticated attacker can crash the server.

### Recommendation

```go
// Limit body size before reading
r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
bodyBytes, err := io.ReadAll(r.Body)
if err != nil {
    // handle error
}
```

Or use `http.MaxBytesReader` as middleware.

---

## [HIGH] No Rate Limiting

### Location

Project-wide — no middleware or rate limiting exists.

### Problem

Endpoints are unprotected against brute-force attacks, enumeration, or automated scraping. An attacker can:
- Enumerate product/order IDs at high speed
- Spam order creation
- Brute-force GraphQL queries

### Recommendation

Add a rate-limiting middleware (e.g., `golang.org/x/time/rate` or `github.com/ulule/limiter`).

```go
func RateLimitMiddleware(next http.Handler) http.Handler {
    limiter := rate.NewLimiter(rate.Every(time.Second), 10) // 10 req/s
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !limiter.Allow() {
            utils.SendJSONError(w, http.StatusTooManyRequests, "Rate limit exceeded")
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

---

## [MEDIUM] Debug Logging of Raw Request Bodies (PII Risk)

### Location

`api/rest/product_handler.go:68-71`, `api/rest/order_handler.go:62-65`

```go
reqString := string(bodyBytes)
log.Debug(ctx, h.logger, "raw body",
    zap.Int("length", len(bodyBytes)),
    zap.String("body", strings.ReplaceAll(reqString, " ", "")),
)
```

### Problem

Raw request bodies are logged at debug level, which includes `CreateOrderReq.CardNumber` (credit card numbers). The ROADMAP itself states: *"Never log request bodies (may contain PII/secrets)"*. Even though this is debug level, debug logging is currently the default (`LOG_LEVEL=debug` in `.env`).

### Impact

Card numbers and potentially other PII are written to log files. In production with debug logging accidentally enabled, this creates a serious data exposure risk.

### Recommendation

Remove raw body logging entirely, or redact sensitive fields. Do not log request bodies.

---

## [MEDIUM] Panic in Unimplemented Delete Methods

### Location

`repos/order_repo.go:72-73`, `services/order_service.go:113-114`

```go
func (or orderRepo) Delete() {
    panic("unimplemented")
}
```

### Problem

The `Delete()` method is part of the public `OrderRepo` interface. If any caller invokes it (e.g., a future handler), it will crash the server. Even if not currently wired to a route, the interface contract is broken.

### Recommendation

Either remove the method from the interface (YAGNI — don't define methods you don't use) or return a proper error:

```go
func (or orderRepo) Delete(ctx context.Context, id string) error {
    return errors.New("order deletion not yet implemented")
}
```

---

## [MEDIUM] FormatValidationErrors Only Handles "required" Tag

### Location

`utils/validations.go:9-21`

```go
switch e.Tag() {
case "required":
    message = fmt.Sprintf("%s is required", e.StructField())
default:
    message = "Invalid Request"
}
```

### Problem

Validations like `gt=0`, `min=0`, `len=16`, `numeric`, `omitempty` are all collapsed to generic "Invalid Request". This removes all specificity from client-side error handling.

### Impact

Clients cannot distinguish "price must be greater than 0" from "stock must be at least 0" from "card number must be 16 digits". Poor developer experience.

### Recommendation

```go
switch e.Tag() {
case "required":
    message = fmt.Sprintf("%s is required", e.StructField())
case "gt":
    message = fmt.Sprintf("%s must be greater than %s", e.StructField(), e.Param())
case "min":
    message = fmt.Sprintf("%s must be at least %s", e.StructField(), e.Param())
case "len":
    message = fmt.Sprintf("%s must be exactly %s characters", e.StructField(), e.Param())
case "numeric":
    message = fmt.Sprintf("%s must be numeric", e.StructField())
default:
    message = fmt.Sprintf("Invalid value for %s", e.StructField())
}
```

---

## [MEDIUM] Delete Product REST Handler Returns 200 with Body Instead of 204 No Content

### Location

`api/rest/product_handler.go:244-247`

```go
w.WriteHeader(http.StatusOK)
json.NewEncoder(w).Encode(prod)
```

### Problem

REST best practice for DELETE is to return `204 No Content` with an empty body. The ROADMAP confirms this: *"DELETE returns 200 with JSON; Phase 1 target is 204 No Content."* The current implementation returns 200 with the deleted product body.

### Recommendation

```go
w.WriteHeader(http.StatusNoContent)
```

---

## [MEDIUM] `fmt.Errorf` Format String Leak in Order Repo

### Location

`repos/order_repo.go:45`

```go
log.Error(ctx, or.logger, "failed to fetch order: %w", zap.Error(err))
```

### Problem

The `%w` verb is a `fmt.Errorf` convention for error wrapping, not a zap placeholder. Zap treats this as a literal string — the log output will contain the literal text `%w` instead of the error message. This is inconsistent with how other log calls in the codebase work (zap's structured logging doesn't use format verbs in the message parameter).

### Recommendation

```go
log.Error(ctx, or.logger, "failed to fetch order", zap.Error(err))
```

---

## [MEDIUM] ID Generation Uses `math/rand` Instead of `crypto/rand`

### Location

`utils/utils.go:129-139`

```go
for range 7 {
    result.WriteByte(charSet[rand.Intn(36)])
}
```

### Problem

`math/rand` is not cryptographically secure. In Go 1.20+, the global source is auto-seeded, making it unpredictable across restarts. However, within a single process, knowledge of the internal state (e.g., through a side-channel) could allow ID prediction. For product/order IDs, this may enable enumeration attacks.

### Recommendation

For production, use `crypto/rand` for ID generation, or at minimum use a UUID library.

---

## [MEDIUM] Transaction Parameter Leaks DB Implementation into Service Layer

### Location

`repos/product_repo.go:18` — `FetchByID(txn *sqlx.Tx, ctx context.Context, id string)`

### Problem

The `ProductRepo` interface forces callers to pass a `*sqlx.Tx` (or nil) as the first argument to `FetchByID`. This leaks database transaction semantics into the service layer, which should be persistence-agnostic. The `OrderService` must know it's inside a transaction and pass the tx object, coupling it to the repo's implementation.

### Recommendation

Refactor to one of these patterns:
- **Unit of Work**: The service calls `repo.BeginTransaction()`, then uses the resulting context. The repo internally manages the transaction binding via context.
- **Separate interfaces**: `ProductReader` (no transaction) and `ProductWriter` (with transaction support).

---

## [MEDIUM] Negative Pagination Parameters Not Handled

### Location

`repos/product_repo.go:65-87`, `repos/order_repo.go:51-69`

### Problem

If `limit` or `offset` is passed as a negative integer, the SQL query is constructed with negative values. While CockroachDB may reject this with an error, it's an unhandled edge case.

### Recommendation

Validate at handler or service level:

```go
if limit < 0 {
    limit = 0
}
if offset < 0 {
    offset = 0
}
```

---

## [MEDIUM] No Graceful Server Startup Detection

### Location

`main.go:68-73`

```go
go func() {
    logger.Info("listening on " + port)
    if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        logger.Fatal("serve error", zap.Error(err))
    }
}()
```

### Problem

The server start is launched in a goroutine with no way to confirm it is actually listening before the signal handler is registered. If the server fails to start (e.g., port in use), the process does exit via `logger.Fatal`, but there's a theoretical race where the signal could be registered yet the server is not yet accepting connections.

### Recommendation

Use a channel to signal successful startup:

```go
ready := make(chan struct{})
go func() {
    close(ready)
    if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        logger.Fatal("serve error", zap.Error(err))
    }
}()
<-ready
```

---

## [MEDIUM] `godotenv.Load` Failure is Fatal

### Location

`main.go:27-29`

```go
if err := godotenv.Load(".env"); err != nil {
    log.Fatalln("Error loading .env file...")
}
```

### Problem

If `.env` doesn't exist (e.g., in a containerized deployment using environment variables injected directly), the application crashes immediately. Many deployments provide env vars through the OS, not a `.env` file. The application should work without a `.env` file as long as the required env vars are set at the OS level.

### Recommendation

```go
if err := godotenv.Load(".env"); err != nil {
    logger.Warn(".env file not found, using OS environment variables")
}
```

---

## [MEDIUM] `utils/constants.go` is Empty Dead Code

### Location

`utils/constants.go:1-3`

```go
package utils

const ()
```

### Problem

Empty file that serves no purpose. Dead code that should be removed.

### Recommendation

Delete the file.

---

## [MEDIUM] Inconsistent SQL Keyword Casing

### Location

`repos/product_repo.go:36` uses uppercase `INSERT INTO`, `repos/product_repo.go:47` uses lowercase `select`.

### Problem

Codebase mixes `INSERT INTO ... VALUES` with `select * from`. The ROADMAP identifies this as tech debt.

### Recommendation

Standardize on lowercase SQL keywords throughout (more common in Go/SQLx community).

---

## [LOW] No CORS Configuration

### Location

`main.go` — middleware chain has no CORS handler.

### Problem

If the API is consumed by browser-based clients, all cross-origin requests will be blocked.

### Recommendation

Add CORS middleware:

```go
func CORSMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusOK)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

(Use a specific origin in production, not `*`.)

---

## [LOW] `.env` File Contains Production Credentials in Repository

### Location

`.env` — contains real database credentials (`avnpl:CDEL-LpTeRrr5i8F7ji8IQ@go-march-db-8756.j77.aws-ap-south-1.cockroachlabs.cloud`).

### Problem

If this repository is shared or public, real credentials are exposed.

### Recommendation

- Create `.env.example` with placeholder values
- Add `.env` to `.gitignore`
- Rotate the exposed credentials immediately

---

## [LOW] GraphQL Handler Uses `http.Error` Instead of JSON

### Location

`api/graphql/handler.go:33,48,54`

```go
http.Error(w, "Invalid HTTP Method", http.StatusMethodNotAllowed)
http.Error(w, "Invalid Request", http.StatusBadRequest)
```

### Problem

Returns plain-text error responses, inconsistent with the REST API's JSON error responses.

### Recommendation

Use `utils.SendJSONError(w, statusCode, "message")` for consistency.

---

## [LOW] `sql.ErrNoRows` Checked Directly in Product Handler Delete

### Location

`api/rest/product_handler.go:236`

```go
if errors.Is(err, sql.ErrNoRows) {
```

### Problem

The handler checks for `sql.ErrNoRows` directly, but the service wraps errors. The sentinel will not propagate through `fmt.Errorf("...: %w", err)` wrapping from the service or repo. It should use `customErrors.RecordNotFound` instead.

### Recommendation

The service should convert `sql.ErrNoRows` to `customErrors.RecordNotFound`, and the handler should check for that sentinel.

---

## [LOW] `go.mod` Dependency Versions

### Location

`go.mod`

### Problem

`go.uber.org/zap v1.13.0` is significantly behind current (v1.27.x). `graphql-go/graphql v0.8.1` may have unpatched issues. No `go.sum` file is present in the repo.

### Recommendation

Run `go get -u` and commit `go.sum`.

---

# Architecture Assessment

**Strengths:**
- Clean layered architecture (handler → service → repo) is well-implemented
- Service layer is API-agnostic and could be shared across REST/GraphQL/SOAP/gRPC
- Interface-based service layer enables testability through mocking
- Consistent use of context propagation throughout all layers
- Good separation of concerns between packages

**Weaknesses:**
- Transaction management leaks into the service layer via the `txn *sqlx.Tx` parameter pattern
- No central mechanism to manage transaction boundaries at the service level
- The `ProductRepo.FetchByID` signature is awkward — most callers pass `nil` for the transaction parameter
- No shared validation between REST and GraphQL input types
- Admin and SOAP/gRPC packages are empty stubs with no structure

**Recommendations:**
- Adopt a Unit of Work pattern or use context-based transaction management to decouple transaction logic from the service layer
- Consider a shared `validators` package for cross-API validation
- Define `OrderRepo.Delete(ctx, id)` with proper parameters before any caller exists

---

# Concurrency Assessment

**Strengths:**
- Database connection pool is configured with max open/idle connections and lifetime
- HTTP server has read/write/idle timeouts
- Graceful shutdown is implemented

**Weaknesses:**
- The order creation service has a **critical race condition**: if `BeginTransaction` fails, `defer txn.Rollback()` panics on nil. This is the #1 most dangerous concurrency bug.
- No optimistic or pessimistic locking for stock decrement. Under concurrent orders, stock could go negative. The `UpdateProductStock` does a simple `SET stock = $1` based on the read value, creating a classic read-modify-write race if two orders arrive simultaneously.
- No context timeouts are applied at the service or repo level. Slow queries can hold connections up to the HTTP server's write timeout (10s).
- No mutex or synchronization for in-memory state (not applicable yet, but relevant when WebSocket is introduced)

**Recommendations:**
- Fix the nil-transaction panic immediately
- Use `UPDATE products SET stock = stock - $1 WHERE prod_id = $2` (atomic decrement) instead of reading-modify-writing
- Add request-scoped timeouts: `context.WithTimeout(r.Context(), 5*time.Second)` at the handler level

---

# Security Assessment

**Critical Issues:**
1. Full credit card numbers stored in plaintext in both the `orders` table and Go models
2. Raw request bodies logged at debug level (includes card numbers with current default log level)
3. No request body size limits — memory exhaustion DoS possible
4. No rate limiting — endpoints open to brute-force

**Medium Issues:**
5. No CORS policy — cross-origin attacks possible from browser clients
6. `.env` file with production credentials potentially committed
7. Predictable ID generation using `math/rand`
8. No CSRF protection on state-changing endpoints

**Recommendations:**
1. Never store full card numbers; only store last 4 digits
2. Add body size limits via middleware
3. Add rate limiting middleware
4. Remove body logging; implement a redaction filter
5. Add CORS middleware with specific allowed origins
6. Add `.env` to `.gitignore`, create `.env.example`
7. Use `crypto/rand` or UUIDs for ID generation

---

# Performance Assessment

**Strengths:**
- Database connection pooling is configured
- Read queries are appropriately paginated
- Zap is a high-performance logger

**Weaknesses:**
1. Request bodies are read into memory twice (once as `[]byte`, then re-decoded) — unnecessary allocation
2. `NamedQueryContext` for UPDATE in `product_repo.go` is slower than direct `ExecContext` with positional args; acceptable but worth noting
3. No caching layer — repeated reads for the same product hit the database every time
4. GraphQL handler does not implement DataLoader pattern; each resolved field may result in an independent database query (N+1 problem when the schema evolves to include nested types)
5. Debug logging includes field serialization overhead even when debug output is not consumed

**Recommendations:**
- Read body once and pass the byte slice to the decoder
- Add request-level caching (e.g., `go-cache` or `ristretto`) for frequently-read products
- Prepare DataLoader patterns before implementing GraphQL order/product relationships

---

# Maintainability Assessment

**Strengths:**
- Clear package structure and naming
- Interfaces defined at service layer for mockability
- Custom error types with HTTP status mapping
- Consistent error wrapping pattern

**Weaknesses:**
1. `utils` package is a dumping ground — contains logging, errors, context, middleware, validations, utilities. Should be split.
2. `customErrors` package name uses plural `errors` conflicting with the standard library `errors` package naming convention
3. `log` package name shadows the standard library `log` package (imported in `utils/utils.go:6`)
4. Empty files: `utils/constants.go`, `api/grpc/grpc.go`, `api/soap/soap.go`, `api/admin/admin.go`
5. No `go.sum` committed
6. REST handler functions are long (createProduct is ~45 lines), could be factored into helper methods
7. Pagination parameter parsing is duplicated between product and order handlers

**Recommendations:**
- Split `utils` into `utils/log`, `utils/errors`, `utils/context`, `utils/middleware`, `utils/validation`
- Rename `customErrors` to `apperrors` or `errors` (with proper module path)
- Rename `log` package to `zlog` or `logging` to avoid shadowing
- Create shared pagination helper
- Remove empty package stubs or add TODO stubs with `// TODO` comments

---

# Testing Assessment

**Grade: F — No tests exist.**

The repository contains zero test files. No unit tests, integration tests, handler tests, or end-to-end tests.

**Priority test targets (in order):**
1. `services/order_service.go` — the Create method has critical bugs that tests would catch immediately
2. `utils/customErrors/errors.go` — HTTP status code mapping
3. `utils/validations.go` — error message formatting
4. `api/rest/product_handler.go` — handler error paths
5. `repos/product_repo.go` — query correctness
6. `utils/utils.go` — ID generation format and uniqueness

---

# Refactoring Priorities

1. **Fix column name mismatch** (Critical) — Align `models/models.go` with the database schema
2. **Fix nil transaction panic** (Critical) — Check error before deferring rollback
3. **Add tests** (Critical) — At minimum, table-driven tests for service + error mapping
4. **Remove raw body logging** (High) — Security risk
5. **Add body size limits** (High) — DoS prevention
6. **Add production logger mode** (High) — JSON output for log aggregation
7. **Refactor transaction management** (Medium) — Use Unit of Work or context-based binding
8. **Remove plaintext card storage** (High) — Compliance risk
9. **Consolidate `utils` package** (Medium) — Split into focused sub-packages
10. **Add rate limiting** (High) — Production readiness

---

# Quick Wins

1. **Fix `order_repo.go` column name** — Change `amount` → `total_price` in INSERT query and add `db:"total_price"` tag to model field
2. **Add nil-check before `defer txn.Rollback()`** — 2-line fix, prevents crash
3. **Remove raw body logging** — Delete the `log.Debug` calls that log request bodies
4. **Add `http.MaxBytesReader`** — 1-line addition in each handler
5. **Remove empty `constants.go`** — Delete dead file
6. **Fix `%w` in zap log call** — Remove format verb from `order_repo.go:45`
7. **Add error returns for `(nil, nil)` in GraphQL resolvers** — Change to `return nil, errors.New("...")`
8. **Use `customErrors.RecordNotFound` in product handler delete** — Instead of checking `sql.ErrNoRows` directly in the handler

---

# Long-Term Improvements

1. Implement proper transaction management (Unit of Work pattern)
2. Add authentication middleware (Phase 7)
3. Add comprehensive test suite with CI pipeline
4. Add rate limiting and CORS middleware
5. Replace `math/rand` with `crypto/rand` for ID generation
6. Add Redis/Memcached caching layer for read-heavy endpoints
7. Implement database migration runner (`golang-migrate` or atlas)
8. Add health check endpoint and metrics (Prometheus)
9. Add request tracing (OpenTelemetry)
10. Implement structured error responses with correlation IDs
11. Separate `utils` into focused sub-packages
12. Add input validation for pagination parameters (negative values, excessive limits)
13. Update all dependencies to latest versions and commit `go.sum`
14. Add `.env.example` and `.gitignore`

---

# Summary Tables

## Top 5 Highest Priority Fixes

| # | Fix | Severity | Effort |
|---|-----|----------|--------|
| 1 | Fix column name mismatch (`amount` vs `total_price`) | Critical | 10 min |
| 2 | Fix nil pointer panic on failed transaction | Critical | 5 min |
| 3 | Write first test suite (service layer) | Critical | 2-3 hours |
| 4 | Remove plaintext card storage | High | 30 min |
| 5 | Add request body size limits | High | 10 min |

## Top 5 Easiest Wins

| # | Fix | Effort |
|---|-----|--------|
| 1 | Fix `amount`/`total_price` column name | 10 min |
| 2 | Fix nil transaction panic | 5 min |
| 3 | Remove raw body debug logging | 5 min |
| 4 | Delete empty `constants.go` | 1 min |
| 5 | Fix `%w` literal in zap log call | 1 min |

## Top 5 Scalability Concerns

| # | Concern | Impact |
|---|---------|--------|
| 1 | No connection pool tuning for CockroachDB serverless | Under-provisioned connections during load spikes |
| 2 | Read-modify-write stock decrement is not atomic | Negative stock under concurrency |
| 3 | No caching — every request hits the database | DB becomes bottleneck at scale |
| 4 | No pagination enforcement — unlimited result sets possible | Memory exhaustion |
| 5 | No request timeouts at service/repo level | Slow queries hold connections indefinitely |

## Top 5 Reliability Concerns

| # | Concern | Impact |
|---|---------|--------|
| 1 | Nil transaction panic crashes entire process | Full outage on any DB error during order creation |
| 2 | No retry logic for transient DB errors | Single DB blip = user-facing error |
| 3 | GraphQL silent null responses on bad input | Client cannot detect or recover from errors |
| 4 | No graceful degradation — all endpoints share same pool | One slow query affects all endpoints |
| 5 | No health check endpoint | Orchestrator cannot detect unhealthy instances |