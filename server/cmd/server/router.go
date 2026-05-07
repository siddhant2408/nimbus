package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/siddhant2408/nimbus/internal/events"
	"github.com/siddhant2408/nimbus/internal/handler"
	"github.com/siddhant2408/nimbus/internal/middleware"
	"github.com/siddhant2408/nimbus/internal/realtime"
	"github.com/siddhant2408/nimbus/internal/storage"
	db "github.com/siddhant2408/nimbus/pkg/db/generated"
)

var defaultOrigins = []string{
	"http://localhost:3001", // Next.js dev
	"http://localhost:5173", // electron-vite dev
}

type router struct {
	chi.Router
}

type routerOptions struct{}

func newRouterWithOptions(pool *pgxpool.Pool, bus *events.Bus, hub *realtime.Hub, opts routerOptions) chi.Router {
	r := &router{
		chi.NewRouter(),
	}

	r.addGlobalMiddleWare()

	queries := db.New(pool)
	localStore := storage.NewLocalStorageFromEnv()
	h := handler.New(queries, pool, localStore, bus, hub)

	return r.addHealthEndPoints(pool).
		addAuthEndpoints(pool, h).
		addWebsocketEndpoints(hub, queries).
		addProtectedAPIRoutes(queries, h).
		addFileServingEndpoints(localStore)
}

func (r *router) addGlobalMiddleWare() {
	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(middleware.ClientMetadata)
	r.Use(middleware.RequestLogger)
	// if opts.HTTPMetrics != nil {
	// 	r.Use(opts.HTTPMetrics.Middleware)
	// }
	r.Use(chimw.Recoverer)
	r.Use(middleware.ContentSecurityPolicy)
	origins := allowedOrigins()

	// Share allowed origins with WebSocket origin checker.
	realtime.SetAllowedOrigins(origins)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Workspace-ID", "X-Workspace-Slug", "X-Request-ID", "X-Agent-ID", "X-Task-ID", "X-CSRF-Token", "X-Client-Platform", "X-Client-Version", "X-Client-OS"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
}

func (r *router) addFileServingEndpoints(store storage.Storage) *router {
	// Local file serving (when using local storage)
	if local, ok := store.(*storage.LocalStorage); ok {
		r.Get("/uploads/*", func(w http.ResponseWriter, r *http.Request) {
			file := strings.TrimPrefix(r.URL.Path, "/uploads/")
			local.ServeFile(w, r, file)
		})
	}
	return r
}

func (r *router) addHealthEndPoints(pool *pgxpool.Pool) *router {
	health := newServerHealth(pool)
	r.Get("/health", health.liveHandler)
	r.Get("/readyz", health.readyHandler)
	r.Get("/healthz", health.readyHandler)
	r.Get("/health/realtime", realtimeMetricsHandler())
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
		r.Post("/api/me/starter-content/import", h.ImportStarterContent)
		r.Post("/api/me/starter-content/dismiss", h.DismissStarterContent)

		r.Route("/api/workspaces", func(r chi.Router) {
			r.Get("/", h.ListWorkspaces)
			r.Post("/", h.CreateWorkspace)
			r.Route("/{id}", func(r chi.Router) {
				// Member-level access
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireWorkspaceMemberFromURL(queries, "id"))
					r.Get("/", h.GetWorkspace)
					r.Get("/members", h.ListMembersWithUser)
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

			// Inbox
			r.Route("/api/inbox", func(r chi.Router) {
				r.Get("/", h.ListInbox)
				r.Get("/unread-count", h.CountUnreadInbox)
				r.Post("/mark-all-read", h.MarkAllInboxRead)
				r.Post("/archive-all", h.ArchiveAllInbox)
				r.Post("/archive-all-read", h.ArchiveAllReadInbox)
				r.Post("/archive-completed", h.ArchiveCompletedInbox)
				r.Post("/{id}/read", h.MarkInboxRead)
				r.Post("/{id}/archive", h.ArchiveInboxItem)
			})

			// Projects
			r.Route("/api/projects", func(r chi.Router) {
				r.Get("/search", h.SearchProjects)
				r.Get("/", h.ListProjects)
				r.Post("/", h.CreateProject)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", h.GetProject)
					r.Put("/", h.UpdateProject)
					r.Delete("/", h.DeleteProject)
					r.Get("/resources", h.ListProjectResources)
					r.Post("/resources", h.CreateProjectResource)
					r.Delete("/resources/{resourceId}", h.DeleteProjectResource)
				})
			})

			// Pins
			r.Route("/api/pins", func(r chi.Router) {
				r.Get("/", h.ListPins)
				r.Post("/", h.CreatePin)
				r.Put("/reorder", h.ReorderPins)
				r.Delete("/{itemType}/{itemId}", h.DeletePin)
			})

			// Notification preferences
			r.Route("/api/notification-preferences", func(r chi.Router) {
				r.Get("/", h.GetNotificationPreferences)
				r.Put("/", h.UpdateNotificationPreferences)
			})
		})
	})
	return r
}

func allowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("FRONTEND_ORIGIN"))
	}
	if raw == "" {
		return defaultOrigins
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return defaultOrigins
	}
	return origins
}
