package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/siddhant2408/nimbus/internal/logger"
)

var version = "dev"

func main() {
	logger.Init()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://nimbus:nimbus@localhost:5433/nimbus?sslmode=disable"
	}

	// Connect to database
	ctx := context.Background()
	pool, err := newDBPool(ctx, dbURL)
	if err != nil {
		slog.Error("unable to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("unable to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to database")
	logPoolConfig(pool)

	// router
	r := newRouterWithOptions(pool, routerOptions{})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}
	slog.Info("server starting", "port", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
