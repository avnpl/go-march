package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gql "github.com/avnpl/go-march/api/graphql"
	"github.com/avnpl/go-march/api/rest"
	"github.com/graphql-go/graphql"

	"github.com/avnpl/go-march/repos"
	"github.com/avnpl/go-march/services"
	"github.com/avnpl/go-march/utils"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

func main() {
	logger := utils.BuildLogger()
	defer logger.Sync()

	db := utils.GetDBPoolObject(logger)
	defer db.Close()

	// Initialize the layers
	productRepo := repos.NewPGProductRepo(db)
	productService := services.NewProductService(productRepo, logger)
	productHandler := rest.NewProductHandler(productService, logger)

	// Set up the HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/product", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			productHandler.UpdateProduct(w, r)
		case http.MethodPost:
			productHandler.CreateProduct(w, r)
		default:
			utils.SendJSONError(w, http.StatusMethodNotAllowed, "Invalid HTTP Method")
		}
	})
	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			productHandler.FetchAllProducts(w, r)
		} else {
			utils.SendJSONError(w, http.StatusMethodNotAllowed, "Invalid HTTP Method")
		}
	})
	mux.HandleFunc("/product/{id}", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			productHandler.DeleteProduct(w, r)
		case http.MethodGet:
			productHandler.FetchProduct(w, r)
		default:
			utils.SendJSONError(w, http.StatusMethodNotAllowed, "Invalid HTTP Method")
		}
	})

	if err := gql.NewSchema(productService); err != nil {
		logger.Fatal("failed to instantiate GraphQL Schema", zap.Error(err))
	}

	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			utils.SendJSONError(w, http.StatusMethodNotAllowed, "Invalid HTTP Method")
			return
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			utils.SendInternalError(w)
			return
		}
		logger.Debug("graphql body", zap.String("body", string(bodyBytes)))

		var params struct {
			Query string `json:"query"`
		}
		err = json.Unmarshal(bodyBytes, &params)
		if err != nil {
			logger.Error("something went wrong decoding the request")
			utils.SendJSONError(w, http.StatusBadRequest, "Invalid Request")
			return
		}

		if params.Query == "" {
			utils.SendJSONError(w, http.StatusBadRequest, "Query cannot be empty")
			return
		}

		result := graphql.Do(graphql.Params{
			Schema:        gql.Schema,
			RequestString: params.Query,
			Context:       r.Context(),
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(result)
	})

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start the server in a separate GR
	go func() {
		logger.Info("listening on :8080")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("serve error", zap.Error(err))
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("Shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	logger.Info("goodbye")
}
