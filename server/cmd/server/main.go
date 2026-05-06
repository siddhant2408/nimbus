package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/siddhant2408/nimbus/internal/events"
	"github.com/siddhant2408/nimbus/internal/logger"
	"github.com/siddhant2408/nimbus/internal/realtime"
	db "github.com/siddhant2408/nimbus/pkg/db/generated"
)

var version = "dev"

func main() {
	logger.Init()

	dbURL := os.Getenv("DATABASE_URL")
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

	bus := events.New()
	hub := realtime.NewHub()
	go hub.Run()

	//TODO reuse db.new call
	queries := db.New(pool)
	hub.SetAuthorizer(newScopeAuthorizer(queries))

	//add listeners
	registerListeners(bus, hub)
	registerSubscriberListeners(bus, queries)
	registerActivityListeners(bus, queries)
	registerNotificationListeners(bus, queries)

	// router
	r := newRouterWithOptions(pool, bus, hub, routerOptions{})

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
