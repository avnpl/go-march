package graphql

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/avnpl/go-march/services"
	gql "github.com/graphql-go/graphql"
	"go.uber.org/zap"
)

type GraphQLHandler struct {
	schema gql.Schema
	logger *zap.Logger
}

func NewGraphQLHandler(productService services.ProductService, logger *zap.Logger) GraphQLHandler {
	schema, err := CreateNewSchema(productService, logger)
	if err != nil {
		logger.Fatal("failed to instantiate GraphQL Schema", zap.Error(err))
	}
	return GraphQLHandler{schema: schema, logger: logger}
}

func (h GraphQLHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/graphql", h.handleGraphQL)
}

func (h GraphQLHandler) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid HTTP Method", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}
	h.logger.Debug("graphql body", zap.String("body", string(bodyBytes)))

	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(bodyBytes, &params); err != nil {
		h.logger.Error("failed to decode GraphQL request", zap.Error(err))
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	if params.Query == "" {
		http.Error(w, "Query cannot be empty", http.StatusBadRequest)
		return
	}

	result := gql.Do(gql.Params{
		Schema:        h.schema,
		RequestString: params.Query,
		Context:       r.Context(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}
