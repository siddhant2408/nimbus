package main

import (
	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
)

type router struct {
	chi.Router
}

type routerOptions struct{}

func newRouterWithOptions(pool *pgxpool.Pool, opts routerOptions) chi.Router {
	r := &router{
		chi.NewRouter(),
	}
	return r.addHealthEndPoints(pool)
}

func (r *router) addHealthEndPoints(pool *pgxpool.Pool) *router {
	health := newServerHealth(pool)
	// Health / readiness checks
	r.Get("/health", health.liveHandler)
	r.Get("/readyz", health.readyHandler)
	r.Get("/healthz", health.readyHandler)
	return r
}
