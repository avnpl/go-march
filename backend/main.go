package main

import (
	"context"
	"github.com/avnpl/go-march/utils"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/avnpl/go-march/handlers"
	"github.com/avnpl/go-march/repos"
	"github.com/avnpl/go-march/services"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// --- Logger ---
	enc := zap.NewProductionEncoderConfig()
	enc.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(zapcore.NewJSONEncoder(enc), zapcore.Lock(os.Stdout), zapcore.InfoLevel)
	logger := zap.New(core)
	defer logger.Sync()

	// --- DB ---
	dsn := utils.GetEnvVar("DB_URL")
	if dsn == "" {
		logger.Fatal("DB_URL not set")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		logger.Fatal("db connect failed", zap.Error(err))
	}
	defer db.Close()

	// --- DI Layers ---
	repo := repos.NewPostgresProductRepo(db)
	//validator := validate.NewValidator()
	svc := services.NewProductService(repo, logger)
	h := handlers.NewProductHandler(svc, logger)

	// --- HTTP Setup ---
	mux := http.NewServeMux()
	mux.HandleFunc("/products", h.CreateProduct)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// --- Run Server ---
	go func() {
		logger.Info("listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("serve error", zap.Error(err))
		}
	}()

	// --- Graceful Shutdown ---
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	logger.Info("goodbye")
}
