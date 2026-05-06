package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/siddhant2408/nimbus/internal/events"
	"github.com/siddhant2408/nimbus/internal/handler"
	"github.com/siddhant2408/nimbus/internal/middleware"
	"github.com/siddhant2408/nimbus/internal/storage"
	db "github.com/siddhant2408/nimbus/pkg/db/generated"
)

type router struct {
	chi.Router
}

type routerOptions struct{}

func newRouterWithOptions(pool *pgxpool.Pool, bus *events.Bus, opts routerOptions) chi.Router {
	r := &router{
		chi.NewRouter(),
	}
	queries := db.New(pool)
	localStore := storage.NewLocalStorageFromEnv()
	h := handler.New(queries, pool, localStore, bus)

	return r.addHealthEndPoints(pool).
		addAuthEndpoints(pool, h).
		addProtectedAPIRoutes(queries, h)
}

func (r *router) addHealthEndPoints(pool *pgxpool.Pool) *router {
	health := newServerHealth(pool)
	r.Get("/health", health.liveHandler)
	r.Get("/readyz", health.readyHandler)
	r.Get("/healthz", health.readyHandler)
	return r
}

func (r *router) addAuthEndpoints(pool *pgxpool.Pool, h *handler.Handler) *router {
	r.Post("/auth/login", h.Login)
	r.Post("/auth/logout", h.Logout)
	r.Route("/api/tokens", func(r chi.Router) {
		r.Get("/", h.ListPersonalAccessTokens)
		r.Post("/", h.CreatePersonalAccessToken)
		r.Delete("/{id}", h.RevokePersonalAccessToken)
	})
	return r
}

func (r *router) addProtectedAPIRoutes(queries *db.Queries, h *handler.Handler) *router {
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(queries, nil))

		// --- User-scoped routes (no workspace context required) ---
		r.Get("/api/me", h.GetMe)
		r.Patch("/api/me", h.UpdateMe)
		r.Patch("/api/me/onboarding", h.PatchOnboarding)
		r.Post("/api/me/onboarding/complete", h.CompleteOnboarding)

		r.Route("/api/workspaces", func(r chi.Router) {
			r.Get("/", h.ListWorkspaces)
			r.Post("/", h.CreateWorkspace)
			r.Route("/{id}", func(r chi.Router) {
				// Member-level access
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireWorkspaceMemberFromURL(queries, "id"))
					r.Get("/", h.GetWorkspace)
				})
				// Admin-level access
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireWorkspaceRoleFromURL(queries, "id", "owner", "admin"))
					r.Put("/", h.UpdateWorkspace)
					r.Patch("/", h.UpdateWorkspace)
				})
				// Owner-only access
				r.With(middleware.RequireWorkspaceRoleFromURL(queries, "id", "owner")).Delete("/", h.DeleteWorkspace)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireWorkspaceMember(queries))
			// Assignee frequency
			r.Get("/api/assignee-frequency", h.GetAssigneeFrequency)

			// Issues
			r.Route("/api/issues", func(r chi.Router) {
				r.Get("/search", h.SearchIssues)
				r.Get("/child-progress", h.ChildIssueProgress)
				r.Get("/", h.ListIssues)
				r.Post("/", h.CreateIssue)
				//r.Post("/quick-create", h.QuickCreateIssue)
				r.Post("/batch-update", h.BatchUpdateIssues)
				r.Post("/batch-delete", h.BatchDeleteIssues)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", h.GetIssue)
					r.Put("/", h.UpdateIssue)
					r.Delete("/", h.DeleteIssue)
					r.Post("/comments", h.CreateComment)
					r.Get("/comments", h.ListComments)
					r.Get("/timeline", h.ListTimeline)
					r.Get("/subscribers", h.ListIssueSubscribers)
					r.Post("/subscribe", h.SubscribeToIssue)
					r.Post("/unsubscribe", h.UnsubscribeFromIssue)
					//r.Get("/active-task", h.GetActiveTaskForIssue)
					//r.Post("/tasks/{taskId}/cancel", h.CancelTask)
					//r.Post("/rerun", h.RerunIssue)
					//r.Get("/task-runs", h.ListTasksByIssue)
					//r.Get("/usage", h.GetIssueUsage)
					r.Post("/reactions", h.AddIssueReaction)
					r.Delete("/reactions", h.RemoveIssueReaction)
					r.Get("/attachments", h.ListAttachments)
					r.Get("/children", h.ListChildIssues)
					r.Get("/labels", h.ListLabelsForIssue)
					r.Post("/labels", h.AttachLabel)
					r.Delete("/labels/{labelId}", h.DetachLabel)
				})
			})
		})
	})
	return r
}
