<!-- 8a75ae33-bfc7-4201-816f-816eca1070c9 da745f7b-27f3-4481-a3cb-ff969e5b1912 -->
# GraphQL and REST API Integration Plan

## Architecture Overview

**Key Principle**: Both REST and GraphQL use the same service layer. Service and repository layers remain unchanged. Only the API handlers differ.

**Proposed Structure**:

```
main.go
  ├── Initialize shared resources (DB, Logger)
  ├── Initialize services (using shared resources)
  ├── Register REST routes (api/rest package)
  └── Register GraphQL routes (api/graphql package)
       │
       └── Both use same service instances

api/rest/rest.go
  ├── NewRESTRouter(productService, logger) -> *http.ServeMux
  └── Sets up all REST routes and handlers

api/graphql/graphql.go
  ├── BuildGraphQLSchema(productService, logger) -> *graphql.Schema
  └── NewGraphQLHandler(schema) -> http.Handler
```

**Benefits**:

- Clean separation: Each API type has its own package
- Shared resources: DB pool and logger instances shared efficiently
- Service layer reuse: Both APIs call same service methods
- Maintainable: Changes to business logic only affect service layer
- Single port: Both APIs on same port initially (simpler setup)

## Current Issues Identified

1. **Architecture Violation**: GraphQL resolvers directly call repository layer, bypassing the service layer
2. **Resource Management**: Creating new DB connections and loggers inside resolver functions (called on every request)
3. **Missing GraphQL Server**: No HTTP handler setup for GraphQL endpoint
4. **Incomplete Schema**: Only queries defined, no mutations for create/update/delete
5. **Context Issues**: Passing `nil` context instead of request context
6. **Routing in main.go**: REST routing logic should be in `api/rest/rest.go` package
7. **Missing REST Router Function**: Need a function to create and return configured REST router
8. **Path Variables**: Need to use Go 1.22+ path variables properly (r.PathValue)

---

## Category 1: Refactor REST API Routing

### Step 1.1: Create REST Router Function in rest.go

**Why**: Move routing logic from main.go to the rest package for better separation of concerns. This allows main.go to focus on initialization and orchestration.

**Summary**: Create `NewRESTRouter()` function in `api/rest/rest.go` that accepts ProductService and logger, sets up all REST routes, and returns a configured `*http.ServeMux`.

**Details**:

- Function signature: `NewRESTRouter(svc services.ProductService, log *zap.Logger) *http.ServeMux`
- Create ProductHandler instance inside the function
- Set up all routes: `/product`, `/products`, `/product/{id}`
- Handle HTTP methods properly (GET, POST, PATCH, DELETE)
- Return the configured mux

**Pseudo Code**:

```go
// api/rest/rest.go
func NewRESTRouter(svc services.ProductService, log *zap.Logger) *http.ServeMux {
    h := NewProductHandler(svc, log)
    mux := http.NewServeMux()
    
    mux.HandleFunc("/product", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodPost:
            h.CreateProduct(w, r)
        case http.MethodPatch:
            h.UpdateProduct(w, r)
        default:
            utils.SendJSONError(w, http.StatusMethodNotAllowed, "Invalid HTTP Method")
        }
    })
    
    mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodGet {
            h.FetchAllProducts(w, r)
        } else {
            utils.SendJSONError(w, http.StatusMethodNotAllowed, "Invalid HTTP Method")
        }
    })
    
    mux.HandleFunc("/product/{id}", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodGet:
            h.FetchProduct(w, r)
        case http.MethodDelete:
            h.DeleteProduct(w, r)
        default:
            utils.SendJSONError(w, http.StatusMethodNotAllowed, "Invalid HTTP Method")
        }
    })
    
    return mux
}
```

---

## Category 2: Fix GraphQL Implementation

### Step 2.1: Refactor GraphQL Schema Builder

**Why**: Centralize schema creation and accept dependencies (service, logger) as parameters instead of creating them inside resolvers. This follows dependency injection pattern and prevents resource leaks.

**Summary**: Create `BuildGraphQLSchema()` function that takes ProductService and logger as parameters, builds the complete GraphQL schema (queries + mutations), and returns it.

**Details**:

- Function signature: `BuildGraphQLSchema(svc services.ProductService, log *zap.Logger) (*graphql.Schema, error)`
- Remove all DB and logger creation from inside functions
- Create both Query and Mutation types
- Return a configured `graphql.Schema` object

**Pseudo Code**:

```go
// api/graphql/graphql.go
func BuildGraphQLSchema(
    svc services.ProductService,
    log *zap.Logger,
) (*graphql.Schema, error) {
    productType := createProductType()
    queryType := createQueryType(svc, log, productType)
    mutationType := createMutationType(svc, log, productType)
    
    schema, err := graphql.NewSchema(graphql.SchemaConfig{
        Query:    queryType,
        Mutation: mutationType,
    })
    return schema, err
}
```

### Step 2.2: Refactor Query Type Creation

**Why**: Remove resource creation from inside the function, use dependency injection pattern. Use service layer instead of direct repo calls.

**Summary**: Modify `createQueryType()` to accept ProductService and logger as parameters, use them in resolvers. Add query for single product by ID.

**Details**:

- Change function signature to accept `services.ProductService` and `*zap.Logger`
- Remove DB and logger creation from inside function
- Use service methods instead of direct repo calls
- Use proper context from `graphql.ResolveParams.Context`
- Add `product(id: ID!)` query for fetching single product

**Pseudo Code**:

```go
func createQueryType(
    svc services.ProductService,
    log *zap.Logger,
    productType *graphql.Object,
) *graphql.Object {
    return graphql.NewObject(graphql.ObjectConfig{
        Name: "Query",
        Fields: graphql.Fields{
            "products": &graphql.Field{
                Type: graphql.NewList(productType),
                Resolve: func(p graphql.ResolveParams) (interface{}, error) {
                    res, err := svc.GetAllProducts(p.Context)
                    if err != nil {
                        log.Error("GetAllProducts failed", zap.Error(err))
                        return nil, err
                    }
                    return res, nil
                },
            },
            "product": &graphql.Field{
                Type: productType,
                Args: graphql.FieldConfigArgument{
                    "id": &graphql.ArgumentConfig{
                        Type: graphql.NewNonNull(graphql.Int),
                    },
                },
                Resolve: func(p graphql.ResolveParams) (interface{}, error) {
                    id, _ := p.Args["id"].(int64)
                    res, err := svc.GetProductByID(p.Context, id)
                    if err != nil {
                        log.Error("GetProductByID failed", zap.Error(err))
                        return nil, err
                    }
                    return res, nil
                },
            },
        },
    })
}
```

### Step 2.3: Create Mutation Type

**Why**: GraphQL needs mutations for create, update, and delete operations to match REST API functionality.

**Summary**: Create `createMutationType()` function that defines GraphQL mutations for creating, updating, and deleting products.

**Details**:

- Create mutation type with three fields: `createProduct`, `updateProduct`, `deleteProduct`
- Use service layer methods
- Define input types for mutations (CreateProductInput, UpdateProductInput)
- Handle errors properly and return appropriate GraphQL errors

**Pseudo Code**:

```go
func createMutationType(
    svc services.ProductService,
    log *zap.Logger,
    productType *graphql.Object,
) *graphql.Object {
    createProductInput := graphql.NewInputObject(graphql.InputObjectConfig{
        Name: "CreateProductInput",
        Fields: graphql.InputObjectConfigFieldMap{
            "name": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
            "price": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.Float)},
            "stock": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.Int)},
        },
    })
    
    updateProductInput := graphql.NewInputObject(graphql.InputObjectConfig{
        Name: "UpdateProductInput",
        Fields: graphql.InputObjectConfigFieldMap{
            "prod_id": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.Int)},
            "name": &graphql.InputObjectFieldConfig{Type: graphql.String},
            "price": &graphql.InputObjectFieldConfig{Type: graphql.Float},
            "stock": &graphql.InputObjectFieldConfig{Type: graphql.Int},
        },
    })
    
    return graphql.NewObject(graphql.ObjectConfig{
        Name: "Mutation",
        Fields: graphql.Fields{
            "createProduct": &graphql.Field{
                Type: productType,
                Args: graphql.FieldConfigArgument{
                    "input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(createProductInput)},
                },
                Resolve: func(p graphql.ResolveParams) (interface{}, error) {
                    input := p.Args["input"].(map[string]interface{})
                    req := &models.CreateProductReq{
                        Name:  input["name"].(string),
                        Price: input["price"].(float64),
                        Stock: input["stock"].(int),
                    }
                    return svc.CreateProduct(p.Context, req)
                },
            },
            "updateProduct": &graphql.Field{
                Type: productType,
                Args: graphql.FieldConfigArgument{
                    "input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(updateProductInput)},
                },
                Resolve: func(p graphql.ResolveParams) (interface{}, error) {
                    input := p.Args["input"].(map[string]interface{})
                    // Convert to UpdateProductReq
                    return svc.UpdateProduct(p.Context, req)
                },
            },
            "deleteProduct": &graphql.Field{
                Type: productType,
                Args: graphql.FieldConfigArgument{
                    "id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
                },
                Resolve: func(p graphql.ResolveParams) (interface{}, error) {
                    id := p.Args["id"].(int64)
                    return svc.DeleteProduct(p.Context, id)
                },
            },
        },
    })
}
```

### Step 2.4: Create GraphQL HTTP Handler

**Why**: GraphQL needs an HTTP endpoint handler to process GraphQL requests. The `graphql-go/handler` package provides this functionality.

**Summary**: Create `NewGraphQLHandler()` function that takes a schema and returns an `http.Handler` configured for GraphQL endpoint.

**Details**:

- Use `graphql-go/handler` package
- Configure handler with schema
- Set up for POST requests (standard GraphQL)
- Optionally support GET for GraphiQL playground

**Pseudo Code**:

```go
import "github.com/graphql-go/handler"

func NewGraphQLHandler(schema *graphql.Schema) http.Handler {
    h := handler.New(&handler.Config{
        Schema:   schema,
        Pretty:   true,
        GraphiQL: true, // Enable GraphiQL playground
    })
    return h
}
```

---

## Category 3: Update main.go Integration

### Step 3.1: Refactor main.go to Use New Router Functions

**Why**: main.go should focus on initialization and orchestration, not routing logic. Use the new router functions from rest and graphql packages.

**Summary**: Update main.go to call `NewRESTRouter()` and `NewGraphQLHandler()`, then register both on the same mux/server.

**Details**:

- Keep DB and logger initialization in main.go
- Initialize services in main.go
- Call `rest.NewRESTRouter()` to get REST mux
- Call `graphql.BuildGraphQLSchema()` to get schema
- Call `graphql.NewGraphQLHandler()` to get GraphQL handler
- Register GraphQL handler at `/graphql` endpoint
- Combine REST routes and GraphQL handler on same mux
- Start single HTTP server on one port

**Pseudo Code**:

```go
func main() {
    logger := utils.BuildLogger()
    defer logger.Sync()
    
    db := utils.GetDBPoolObject(logger)
    defer db.Close()
    
    // Initialize the layers
    repo := repos.NewPGProductRepo(db)
    svc := services.NewProductService(repo, logger)
    
    // Set up REST API
    restMux := rest.NewRESTRouter(svc, logger)
    
    // Set up GraphQL API
    graphqlSchema, err := graphql.BuildGraphQLSchema(svc, logger)
    if err != nil {
        logger.Fatal("Failed to build GraphQL schema", zap.Error(err))
    }
    graphqlHandler := graphql.NewGraphQLHandler(graphqlSchema)
    
    // Combine both APIs on same mux
    mux := http.NewServeMux()
    mux.Handle("/", restMux) // REST routes
    mux.Handle("/graphql", graphqlHandler) // GraphQL endpoint
    
    srv := &http.Server{
        Addr:         ":8080",
        Handler:      mux,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  120 * time.Second,
    }
    
    // Start server and graceful shutdown...
}
```

---

## Category 4: Fix Issues in Existing Code

### Step 4.1: Fix ProductHandler Status Codes

**Why**: Some handlers return incorrect HTTP status codes (e.g., `http.StatusCreated` for GET requests).

**Summary**: Fix HTTP status codes in `api/rest/product_handler.go`:

- `FetchProduct`: Change from `StatusCreated` to `StatusOK`
- `DeleteProduct`: Change from `StatusCreated` to `StatusOK` (or `StatusNoContent` if not returning body)

**Details**: Review each handler method and ensure correct status codes according to REST conventions.

### Step 4.2: Fix Error Handling in FetchProduct and DeleteProduct

**Why**: In `FetchProduct` and `DeleteProduct`, the `id` parsing error is not checked before calling service.

**Summary**: Add error check after `strconv.ParseInt()` in both handlers.

**Details**: Check if `err != nil` after parsing ID and return `400 Bad Request` if parsing fails.

**Pseudo Code**:

```go
id, err := strconv.ParseInt(idStr, 10, 64)
if err != nil {
    h.log.Error("Invalid ID format", zap.Error(err))
    utilErrs.SendJSONError(w, http.StatusBadRequest, "Invalid ID format")
    return
}
```

### Step 4.3: Fix UpdateByID Context Parameter

**Why**: In `repos/product_repo.go`, `UpdateByID` takes `*context.Context` (pointer) which is unusual. Should be `context.Context` (value).

**Summary**: Change `UpdateByID` signature to use `context.Context` instead of `*context.Context`.

**Details**:

- Update interface in `ProductRepo`
- Update implementation in `pgProductRepo`
- Update service call in `product_service.go`

---

## Category 5: Future Extensibility (Planning)

### Step 5.1: Design for Multiple API Types

**Why**: You plan to add SOAP, WebSocket, and gRPC later. The current architecture should support this.

**Summary**: Document the pattern for adding new API types:

1. Create new package under `api/` (e.g., `api/soap/`, `api/grpc/`)
2. Each package exports a function to create its handler/router
3. All handlers accept shared services and logger
4. Register in main.go on same or different ports

**Details**:

- For same port: Register on same mux with different paths
- For different ports: Start separate HTTP servers in goroutines
- Use nginx for routing when needed
- All share same DB pool and logger instance

**Example Structure**:

```
api/
  ├── rest/
  │   └── rest.go (NewRESTRouter)
  ├── graphql/
  │   └── graphql.go (NewGraphQLHandler)
  ├── soap/
  │   └── soap.go (NewSOAPHandler) - future
  └── grpc/
      └── grpc.go (NewGRPCServer) - future
```

---

## Summary of Changes

**Files to Modify**:

1. `api/rest/rest.go` - Add `NewRESTRouter()` function
2. `api/graphql/graphql.go` - Complete refactor: dependency injection, mutations, HTTP handler
3. `main.go` - Use new router functions, register both APIs
4. `api/rest/product_handler.go` - Fix status codes and error handling
5. `repos/product_repo.go` - Fix context parameter type
6. `services/product_service.go` - Fix context parameter in UpdateProduct call

**Files to Create**:

- None (all modifications to existing files)

**Dependencies**:

- `github.com/graphql-go/handler` - Already in go.mod (indirect), may need to make direct

---

## Testing Strategy

After implementation:

1. Test REST endpoints: `/product`, `/products`, `/product/{id}`
2. Test GraphQL endpoint: `/graphql` (POST for queries/mutations, GET for GraphiQL)
3. Verify both APIs use same service layer (check logs)
4. Verify shared resources (DB pool, logger)
5. Test error handling in both APIs

---

## Notes

- Both APIs will be on same port (`:8080`) initially
- REST: `/product`, `/products`, `/product/{id}`
- GraphQL: `/graphql`
- Later, you can move to separate ports and use nginx for routing
- All API types will share the same service/repo layers

### To-dos

- [ ] Create NewRESTRouter() function in api/rest/rest.go to move routing logic from main.go
- [ ] Refactor BuildGraphQLSchema() to accept service and logger as dependencies
- [ ] Refactor createQueryType() to use service layer and proper context
- [ ] Create createMutationType() with create, update, delete mutations
- [ ] Create NewGraphQLHandler() function to return HTTP handler for GraphQL endpoint
- [ ] Update main.go to use NewRESTRouter() and NewGraphQLHandler(), register both on same mux
- [ ] Fix HTTP status codes in product_handler.go (FetchProduct, DeleteProduct should return StatusOK)
- [ ] Add error check after strconv.ParseInt() in FetchProduct and DeleteProduct handlers
- [ ] Change UpdateByID to use context.Context instead of *context.Context in repo and service