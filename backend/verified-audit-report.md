# Final Code Audit Report — go-march Backend

**Auditor:** Synthesized verdict over four independent reviews (`ring-2-6`, `deepseek`, `opus-cursor`, `minimax`), with every finding re-checked against the current code on `p1/rest-completion` @ `661c925`.
**Date:** 2026-05-19
**Scope:** All Go source, SQL migrations, and `go.mod` under `backend/`.

This document is the single source of truth. Each finding lists exact file paths and line numbers verified in-tree, the problem, its impact, the recommended fix, an improved example, and a confidence level. Findings are ordered by severity — fix them top-down.

---

# Executive Summary

## Overall Assessment

`go-march` is an early-stage learning backend with a sound layered architecture (handlers → services → repositories) and a clear intent to demonstrate five API styles sharing one service layer. The product CRUD path is mostly functional. There are zero automated tests.

The codebase reads cleanly and follows many Go idioms (context propagation, wrapped errors, structured logging, dependency injection), but it is not deployable in its current state.

## Main Risks

1. Zero test coverage — none of the above would be caught by CI.
2. Inconsistent not-found handling across product handlers (#7).
3. GraphQL resolvers swallow input errors as `(nil, nil)` (#10).
4. No pagination bounds on `limit`/`offset` (#13).

## Production Readiness Score: **2/10**

The application would fail the very first realistic order request, would crash on any DB hiccup, and would not pass a basic security review.

## Top 6 Critical Findings (capsule)

| # | Finding | Severity |
|---|---------|----------|
| 1 | Zero tests in the repository | Critical |
| 2 | No pagination bounds on `limit`/`offset` | High |
| 3 | `GetEnvVarInteger` silently falls back on parse error | High |
| 4 | No health/readiness endpoint | Medium |
| 5 | `select *` everywhere — fragile to schema evolution | Medium |

## Technical Debt Assessment

Heavy. The architecture is sound, but the implementation has correctness, security, and reliability defects that span every layer (transaction handling, validation, error mapping, observability, dependency hygiene). Plan for two focused sprints: one for correctness/security blockers, one for hardening (testing, observability, rate limiting).

---

# Detailed Findings

---


## 7. [HIGH] Inconsistent not-found handling across product handlers

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

## 8. [HIGH] `order_repo.FetchByID` returns unwrapped error and logs `%w` literal

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

## 9. [HIGH] No authentication, authorization, rate limiting, or CORS

### Location
`main.go` — middleware chain is just `RequestIDMiddleware`.

### Problem
Every endpoint is anonymous:
- `POST /products`, `DELETE /products/{id}` — anyone can mutate the catalog.
- `POST /orders` — anyone can place orders with arbitrary card data.
- `/graphql` — full read/write on products.

No rate limiting, so a single attacker can brute-force ID enumeration. No CORS, so browser frontends can't call the API at all.

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

## 10. [HIGH] GraphQL resolvers swallow input errors as `(nil, nil)`

### Location
- `api/graphql/resolvers.go:27-30` — `GetProductByID`
- `api/graphql/resolvers.go:74-77, 79-82` — `UpdateProduct` (input and prod_id)
- `api/graphql/resolvers.go:122-125, 127-130` — `DeleteProduct` (input and prod_id)

### Problem
Every type-assertion on `p.Args` returns `(nil, nil)` on failure. The GraphQL library interprets that as "the field resolved to null with no error." Clients see `null` and no indication of what went wrong.

### Impact
- Silent failures on malformed queries.
- Painful debugging for API consumers.
- Masks the schema-level bug that `DeleteProductInput.prod_id` is nullable (finding #11) — without the `nil, nil` swallowing, the type-assertion error would surface it.

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

## 11. [HIGH] `DeleteProductInput.prod_id` is not `NewNonNull`

### Location
`api/graphql/types.go:44-49`

```go
"prod_id": &graphql.InputObjectFieldConfig{
 Type: graphql.String,
 Description: "The ID of the product to be deleted",
},
```

### Problem
`UpdateProductInput.prod_id` is `graphql.NewNonNull(graphql.String)`. `DeleteProductInput.prod_id` is plain `graphql.String`. The schema allows `deleteProduct(input: {})` which then silently no-ops (see finding #10).

### Impact
- Schema inconsistency.
- A delete mutation can be issued with no ID and the server reports success with no error.

### Recommendation
```go
"prod_id": &graphql.InputObjectFieldConfig{
 Type: graphql.NewNonNull(graphql.String),
 Description: "The ID of the product to be deleted",
},
```

### Confidence: **High**

---

## 12. [HIGH] GraphQL handler ignores `variables` and `operationName`

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
 Query string `json:"query"`
 Variables map[string]interface{} `json:"variables"`
 OperationName string `json:"operationName"`
}
// ...
result := gql.Do(gql.Params{
 Schema: h.schema,
 RequestString: params.Query,
 VariableValues: params.Variables,
 OperationName: params.OperationName,
 Context: r.Context(),
})
```

Also replace the three `http.Error` plain-text responses (`api/graphql/handler.go:33, 48, 54`) with `utils.SendJSONError` so the GraphQL surface returns JSON like the REST surface.

### Confidence: **High**

---

## 13. [HIGH] No bounds on `limit`; negative `limit`/`offset` accepted

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

## 14. [HIGH] `BeginTransaction` takes no `context.Context`

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

## 16. [HIGH] `GetEnvVarInteger` silently falls back on parse error

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

## 17. [MEDIUM] `*sqlx.Tx` leaks into the `ProductRepo` public interface

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

## 18. [MEDIUM] `godotenv.Load` is fatal on missing `.env`

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

## 19. [MEDIUM] `server.Shutdown` error ignored

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

## 20. [MEDIUM] No health/readiness endpoint

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

## 21. [MEDIUM] `Conflict` sentinel is never produced

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

## 22. [MEDIUM] No timeout on DB calls or transactions

### Location
- All repo methods rely on the request context only.
- HTTP server has `WriteTimeout: 10*time.Second` (`main.go:62`), but that's the outer bound.
- Order creation transaction (`services/order_service.go:Create`) has no per-step timeout.

### Problem
A slow query inside an active transaction holds a pool connection and a row lock. Under load this cascades — when the pool is exhausted other requests queue, the HTTP write timeout fires, but the goroutine remains until the DB returns.

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

## 23. [MEDIUM] `select *` everywhere — fragile to schema evolution

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
const fetchProduct = "select " + productColumns + " from products where prod_id = $1"
```

### Confidence: **High**

---

## 24. [MEDIUM] `customErrors` package name violates Go conventions

### Location
`utils/customErrors/` directory and import path.

### Problem
Go package names should be lowercase, no underscores, no camelCase. `customErrors` violates this. It's also nested under `utils` for no reason — sentinel errors are imported throughout and the deep path adds noise.

### Recommendation
Rename to `apperrors` (or `errs`) and move to top level.

```
backend/
├── apperrors/
│ └── errors.go
```

Also rename the sentinels themselves to the `ErrXxx` convention with lowercase messages: `var ErrRecordNotFound = errors.New("record not found")`. Capitalised messages produce awkward wrapped errors like `"order_service.Create: Out of Stock"`.

### Confidence: **High**

---

## 25. [MEDIUM] `utils` is a grab-bag package

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

## 26. [LOW] GraphQL handler uses `http.Error` instead of JSON

### Location
`api/graphql/handler.go:33, 48, 54` — `http.Error(w, "Invalid HTTP Method", ...)`

### Problem
Returns plain-text bodies on the GraphQL endpoint while the REST endpoint returns JSON. Inconsistent for a client that expects a uniform error shape.

### Recommendation
Use `utils.SendJSONError`.

### Confidence: **High**

---

## 27. [LOW] `WithRequestID` allocates per log call

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

## 28. [LOW] `logs/app.log` grows unbounded

### Location
`utils/utils.go:77` — `OutputPaths = []string{"stdout", "logs/app.log"}`

### Problem
No rotation. The file grows until the disk fills up, at which point logger writes start failing silently.

### Recommendation
Either drop the file output entirely in production (let container log drivers handle aggregation) or use `gopkg.in/natefinch/lumberjack.v2`.

### Confidence: **High**

---

## 29. [LOW] No `SIGQUIT` handling on shutdown

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
- Transaction concerns leak out of the repo (#17).
- `BeginTransaction` is on `ProductRepo` — orchestrating across `OrderRepo` and `ProductRepo` via the *product* repo's transaction is awkward and surprising.
- `OrderService` depends on both `OrderRepo` and `ProductRepo` directly; no `PaymentRepo` yet despite a payments table.
- `utils` is a grab-bag (#25).
- No `Config` or `App` struct — env vars are read ad-hoc (`utils.GetEnvVarInteger` at the call site in repos and services).
- No domain layer; all behaviour is in services; that's fine for now but watch for service bloat.
- Empty stub packages (`grpc`, `soap`, `admin`) carry no value.
- No observability — no metrics, no tracing, no health probe (#20).

---

# Concurrency Assessment

**Score: 6/10**

**Strengths**
- HTTP server has `ReadTimeout`, `WriteTimeout`, `IdleTimeout`.
- DB pool is configured (max open/idle, lifetime).
- No bespoke goroutines; nothing to leak.
- Context plumbed through every layer.
- Nil-pointer panic on `defer txn.Rollback()` — **FIXED**.
- Stock decrement race — **FIXED** (atomic conditional UPDATE).
- DB connection lifetime — **FIXED** (30 min default, max idle time configured).

**Other issues**
- `BeginTransaction` doesn't accept a context (#14).
- No per-step timeouts in services or repos (#22).
- No goroutine or thread cap; `debug.SetMaxThreads` not used.

---

# Security Assessment

**Score: 1/10**

**Critical**
1. Full PAN stored, logged, and serialized in JSON .
2. Plaintext PAN in `payments.card_number` migration and seed data .
3. No authentication or authorization (#9).
4. Request bodies logged at debug .

**High**
5. `SendErrorResponse` leaks internal error chain .
6. No rate limiting (#9).
7. `math/rand` for IDs — predictability concern, not catastrophic, but worth fixing.
8. Negative pagination accepted (#13).
9. No request size limits or query complexity limits on GraphQL.

**Medium**
11. No CORS .
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
- `select *` everywhere (#23) over-fetches.
- Per-request env var read in `repos/product_repo.go:69` and `services/order_service.go:101`.
- Per-call `logger.With(...)` allocation (#27).
- Double body read in handlers .
- No caching — every read hits CockroachDB.
- No FK indexes — joins will full-scan as tables grow.
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
- **Zero tests** — the single biggest maintainability hole.
- Schema/code drift on orders means the source of truth is unclear.
- Inconsistent error handling across product handlers (#7).
- Dead code (`Conflict` checks #35, `utils/constants.go` #49, panic'd `Delete` methods #4).
- Inconsistent receiver patterns (value vs pointer; `os` shadowing in `orderService` #40).
- Mixed SQL casing .
- `utils` is a grab-bag (#25).
- Old `zap` .
- No `go.sum` rotation script; no CI config (no `.github/workflows`, no `azure-pipelines.yml`).
- Long handler functions (createProduct is ~45 lines doing decode-validate-call-respond by hand five times across the codebase).

---

# Testing Assessment

**Score: 0/10**

- No `*_test.go` files exist anywhere.
- No mocks, fixtures, helpers, or testdata.
- No CI signal — `go test ./...` is a no-op.
- The most expensive bugs in this report (#10, #12, #13) would all be caught by a single passing service-layer test each.

**Priority test targets**
1. `services/order_service.go` — the broken-by-design module.
2. `utils/customErrors/errors.go` — error → HTTP status mapping (covers #11).
3. `api/rest/product_handler.go` and `order_handler.go` — handler error paths.
5. `repos/product_repo.go` integration — query correctness against a test DB (catches #1-style drift).

---

# Refactoring Priorities

| Rank | Task | Severity | Estimated Effort |
|------|------|----------|------------------|
| 1 | Align Order model ↔ migration; move card data to `payments` | Critical | 1–2 h |
| 2 | Remove plaintext PAN storage and all body logging | Critical | 1 h |
| 3 | Remove `panic("unimplemented")` in `Delete` methods | Critical | 15 min |
| 4 | Add service-layer table-driven tests (Create, OutOfStock, NotFound, Update zero-value) | Critical | 1–2 d |
| 5 | Gate logger by `ENV`; switch to JSON in production | High | 30 min |
| 6 | Map `RecordNotFound → 404`; unify error handling across handlers | High | 1 h |
| 7 | Add pagination bounds on `limit`/`offset` | High | 1 h |
| 8 | Refactor transactions out of `ProductRepo` interface (context-bound `TxManager`) | Medium | 2–4 h |
| 9 | Split `utils` into focused packages; rename `customErrors` → `apperrors` | Medium | 2 h |
| 10 | Add health check, rate limiting, CORS, and basic auth | Medium | 1 d |
| 11 | Integrate migration runner; add `.down.sql` files; add FK indexes | Medium | 1 d |
| 12 | Replace `select *` with explicit columns; replace `math/rand` IDs with `xid` | Low | 1 h |
| 13 | Update `zap` and other deps; commit `go.sum` | Low | 30 min |

---

# Quick Wins

1. **`RecordNotFound → 404`** — 1 line (`utils/customErrors/errors.go:32`).
2. **Delete raw-body debug logs** — ~15 lines across three handlers.
3. **Delete `utils/constants.go`** — 1 file.
4. **Replace `"failed to fetch order: %w"`** literal with proper zap fields (`repos/order_repo.go:45`).
5. **Make GraphQL `DeleteProductInput.prod_id` non-null** — 1 line.
6. **Return errors from GraphQL resolvers instead of `nil, nil`** — ~8 lines.
7. **Use `customErrors.RecordNotFound` (not `sql.ErrNoRows`) in `deleteProduct`** — 3 lines.
8. **Remove the `"6969"` magic check** — 4 lines (`services/order_service.go:65-68`).
9. **`server.Shutdown(ctx)` error logged** — 3 lines.

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
| 2 | Remove plaintext PAN storage and all raw-body logging | Critical | 1 h |
| 3 | Add a basic service-layer test suite | Critical | 1–2 d |

# Top 5 Easiest Wins

| # | Fix | Effort |
|---|-----|--------|
| 1 | `RecordNotFound → 404` | 1 line |
| 2 | Delete raw-body debug logs | ~15 lines |
| 3 | Delete `utils/constants.go` | 1 file |
| 4 | `server.Shutdown(ctx)` error logged | 3 lines |
| 5 | Remove `"6969"` magic | 4 lines |

# Top 5 Scalability Concerns

| # | Concern | Impact |
|---|---------|--------|
| 1 | No caching, every read hits CockroachDB | DB-bound at scale |
| 2 | No FK indexes | Full-scan joins as orders grow |
| 3 | Offset-based pagination is O(n) at large offsets | Degraded UX as tables grow |

# Top 3 Reliability Concerns

| # | Concern | Impact |
|---|---------|--------|
| 1 | `panic("unimplemented")` Delete methods | Future route silently lands a crash |
| 2 | Order model ↔ schema drift | Orders subsystem 100% broken |
| 3 | No health/readiness endpoint (#20) | Orchestrators route traffic to broken instances |

---

# Appendix — Notes on the four input reports

This audit subsumes all four input reports. For traceability:

- **opus-cursor** caught most things, including several uniquely (missing `card_number` column in orders migration, zero-value update bug, `os` receiver shadowing, ctx-less `BeginTransaction`, dev-mode DPanic semantics, internal error leak in `SendErrorResponse`, sentinel naming convention). Most accurate and most useful.
- **deepseek** matched opus-cursor on breadth and added the float-comparison bug, the stock race, the GraphQL `variables`/`operationName` gap, missing FK indexes, `RecordNotFound → 404`, and `FormatValidationErrors` keeping only the last error. Some self-corrections mid-document that should have been edited out.
- **ring-2-6** covered the criticals cleanly but missed several nuances (float comparison, stock race, missing card_number column, inconsistent handler not-found handling).
- **minimax** hit the obvious criticals (PAN, body logging, panics) but contained two technical errors: claimed `math/rand` defaults to seed 1 (false since Go 1.20) and claimed `defer Rollback()` after `Commit()` is the bug (it's idiomatic; the real bug is the nil case, which minimax missed entirely). Shortest report and least reliable; the others should be preferred for ground truth.
