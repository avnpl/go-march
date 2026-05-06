package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/avnpl/go-march/api/graphql"
	"github.com/avnpl/go-march/api/rest"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"

	"github.com/avnpl/go-march/repos"
	"github.com/avnpl/go-march/services"
	"github.com/avnpl/go-march/utils"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalln("Error loading .env file...")
	}

	logger := utils.BuildLogger()
	defer logger.Sync()

	db := utils.GetDBPoolObject(logger)
	defer db.Close()

	validate := validator.New(validator.WithRequiredStructEnabled())

	// Initialize the layers
	productRepo := repos.NewPGProductRepo(db, logger)
	productService := services.NewProductService(productRepo, logger)
	productHandler := rest.NewProductHandler(productService, logger, validate)
	gqlHandler := graphql.NewGraphQLHandler(productService, logger)

	// Set up the HTTP server
	mux := http.NewServeMux()
	productHandler.RegisterRoutes(mux)
	gqlHandler.RegisterRoutes(mux)

	port := utils.GetEnvVarString("PORT", ":8013", logger)

	srv := &http.Server{
		Addr:         port,
		Handler:      utils.RequestIDMiddleware(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start the server in a separate GR
	go func() {
		logger.Info("listening on " + port)
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
