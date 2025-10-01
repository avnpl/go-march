package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/avnpl/go-march/handlers"
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
	repo := repos.NewPGProductRepo(db)
	svc := services.NewProductService(repo, logger)
	h := handlers.NewProductHandler(svc, logger)

	// Set up the HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/product", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			h.UpdateProduct(w, r)
		case http.MethodPost:
			h.CreateProduct(w, r)
		default:
			utils.SendJSONError(w, http.StatusMethodNotAllowed, "Invalid HTTP Method")
		}
	})
	mux.HandleFunc("/product/{id}", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			h.DeleteProduct(w, r)
		case http.MethodGet:
			h.FetchProduct(w, r)
		default:
			utils.SendJSONError(w, http.StatusMethodNotAllowed, "Invalid HTTP Method")
		}
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
