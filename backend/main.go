package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/avnpl/go-march/api/graphql"
	myGrpc "github.com/avnpl/go-march/api/grpc"
	"github.com/avnpl/go-march/api/rest"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"

	pb "github.com/avnpl/go-march/api/grpc/proto"
	"github.com/avnpl/go-march/repos"
	"github.com/avnpl/go-march/services"
	"github.com/avnpl/go-march/utils"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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

	// Initialize the Product layers
	productRepo := repos.NewProductRepo(db, logger)
	productService := services.NewProductService(productRepo, logger)
	productHandler := rest.NewProductHandler(productService, logger, validate)

	// Initialize the Order layers
	orderRepo := repos.NewOrderRepo(db, logger)
	orderService := services.NewOrderService(orderRepo, productRepo, logger)
	orderHandler := rest.NewOrderHandler(orderService, logger, validate)

	// Initialize the GraphQL handler
	gqlHandler := graphql.NewGraphQLHandler(productService, orderService, logger)

	// Set up the HTTP server
	mux := http.NewServeMux()
	productHandler.RegisterRoutes(mux)
	orderHandler.RegisterRoutes(mux)
	gqlHandler.RegisterRoutes(mux)

	port := utils.GetEnvVarString("PORT", "8013", logger)

	// Middlewares
	handler := utils.LimitBodySize(1 << 20)(mux)
	handler = utils.RequestIDMiddleware(handler)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start the server in a separate GR
	go func() {
		logger.Info("listening on " + port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("serve error", zap.Error(err))
		}
	}()

	// Initialize the Analytics & gRPC layers
	analyticsService := services.NewAnalyticsService(orderRepo, logger)
	analyticsHandler := myGrpc.NewAnalyticsHandler(analyticsService, logger)

	grpcServer := grpc.NewServer()
	pb.RegisterAnalyticsServiceServer(grpcServer, analyticsHandler)

	// Start the gRPC server in a separate GR
	go func() {
		lis, err := net.Listen("tcp", ":9090")
		if err != nil {
			logger.Fatal("failed to listen for gRPC", zap.Error(err))
		}

		logger.Info("gRPC server listening on :9090")
		err = grpcServer.Serve(lis)
		if err != nil {
			logger.Fatal("failed to serve gRPC", zap.Error(err))
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("Shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// shut down http server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown failed", zap.Error(err))
	}

	// shut down the grpc server
	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()

	select {
	case <-grpcStopped:
		// do nothing. gRPC server stopped successfully
	case <-ctx.Done():
		logger.Error("gRPC server shutdown failed", zap.Error(ctx.Err()))
		grpcServer.Stop()
	}

	logger.Info("goodbye")
}
