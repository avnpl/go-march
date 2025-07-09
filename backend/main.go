package main

import (
	"context"
	"log"
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
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func buildLogger() (*zap.Logger, error) {
	// Start from the ProductionConfig so you get JSON output, but
	// raise the level to DEBUG and add caller info + stacktrace.
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel) // DEBUG+
	loggerConfig.Encoding = "console"
	loggerConfig.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	loggerConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	loggerConfig.EncoderConfig.TimeKey = "ts"
	// Build it, enabling caller info and Error‐level stacktraces:
	return loggerConfig.Build(
		zap.AddCaller(),                   // include file:line in every log
		zap.AddStacktrace(zap.ErrorLevel), // attach stacktrace on Error+
	)
}

func main() {
	// Setup logging
	//enc := zap.NewProductionEncoderConfig()
	//enc.EncodeTime = zapcore.ISO8601TimeEncoder
	//core := zapcore.NewCore(zapcore.NewConsoleEncoder(enc), zapcore.Lock(os.Stdout), zapcore.DebugLevel)
	//logger := zap.New(core)

	logger, err := buildLogger()
	if err != nil {
		log.Fatal("cannot build logger", err)
	}
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

	// Initialize the layers
	repo := repos.NewPGProductRepo(db)
	svc := services.NewProductService(repo, logger)
	h := handlers.NewProductHandler(svc, logger)

	// Set up the HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/products", h.CreateProduct)
	mux.HandleFunc("/product/{id}", h.FetchProduct)

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
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
