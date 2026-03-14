# go-march

A Go backend that implements the same minimal product inventory system across **5 different API architectures**. The goal is to show how REST, GraphQL, SOAP, gRPC, and WebSocket each solve a different problem — using clean, idiomatic Go with no ORM.

Built as a learning project for someone moving from Spring Boot / Express.js to Go.

---

## What It Does

A single backend managing products and orders, exposed through five API styles — each chosen for a reason:

| API | Purpose | Status |
|-----|---------|--------|
| REST | Product CRUD | ✅ Working |
| GraphQL | Flexible data fetching | ✅ Working (products only) |
| SOAP | Transactional order placement | 🚧 Stubbed |
| gRPC | Analytics procedures | 🚧 Stubbed |
| WebSocket | Real-time order/analytics streaming | 🚧 Stubbed |

---

## Quick Start

**Prerequisites:** Go 1.23+, CockroachDB (or PostgreSQL)

```bash
# Clone and navigate
git clone https://github.com/avnpl/go-march.git
cd go-march/backend

# Create environment file
echo "DATABASE_URL=postgresql://user:pass@localhost:26257/inventory?sslmode=disable" > .env
echo "LOG_LEVEL=debug" >> .env

# Run
go run main.go
# Server starts on :8080
```

---

## REST API

Base URL: `http://localhost:8080`

### Products

```bash
# Create a product
curl -X POST http://localhost:8080/product \
  -H "Content-Type: application/json" \
  -d '{"prod_name": "Widget", "price": 9.99, "stock": 100}'

# Get all products
curl http://localhost:8080/products

# Get a product by ID
curl http://localhost:8080/product/{id}

# Update a product
curl -X PATCH http://localhost:8080/product \
  -H "Content-Type: application/json" \
  -d '{"prod_id": "abc123", "price": 12.99}'

# Delete a product
curl -X DELETE http://localhost:8080/product/{id}
```

---

## GraphQL API

Endpoint: `POST http://localhost:8080/graphql`

```bash
# Fetch all products
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ products { prod_id prod_name price stock } }"}'

# Fetch a single product
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ product(id: \"abc123\") { prod_name price stock } }"}'

# Update a product
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "mutation { updateProduct(prod_id: \"abc123\", price: 14.99) { prod_name price } }"}'
```

---

## Project Structure

```
backend/
├── main.go              # Entry point, server setup, route registration
├── api/
│   ├── rest/            # HTTP handlers
│   ├── graphql/         # GraphQL schema, types, queries, mutations, resolvers
│   ├── grpc/            # gRPC server (stubbed)
│   └── soap/            # SOAP handler (stubbed)
├── services/            # Business logic
├── repos/               # Database access (raw SQL via sqlx)
├── models/              # Structs for products, orders, requests
└── utils/               # Logger, DB pool, error helpers, validation
```

---

## Tech Stack

- **Language:** Go 1.23
- **Database:** CockroachDB via `jackc/pgx/v5` + `jmoiron/sqlx`
- **Logging:** `go.uber.org/zap`
- **GraphQL:** `github.com/graphql-go/graphql`
- **Routing:** Standard library `http.ServeMux` (Go 1.22+)
- **No ORM** — all queries are raw SQL

---

## Development

```bash
go build -o bin/server main.go   # Build
go test ./...                    # Test
golangci-lint run                # Lint
go fmt ./...                     # Format
```

See [`backend/CLAUDE.md`](backend/CLAUDE.md) for agent/LLM context and conventions, and [`backend/.cursor/plans/dev.plan.md`](backend/.cursor/plans/dev.plan.md) for the implementation roadmap.
