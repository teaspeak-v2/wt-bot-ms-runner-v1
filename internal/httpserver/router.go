package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/teaspeak-v2/wt-bot-ms-runner-v1/internal/handlers"
	"github.com/teaspeak-v2/wt-bot-ms-runner-v1/internal/middleware"
)

// RouterDeps contains the handler dependencies.
type RouterDeps struct {
	RunnerHandler *handlers.RunnerHandler
	HealthHandler *handlers.HealthHandler
	ServiceAPIKey string
}

// NewRouter creates the HTTP router.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Get("/", deps.HealthHandler.Root)
	r.Get("/healthz", deps.HealthHandler.Healthz)
	r.Get("/readyz", deps.HealthHandler.Readyz)

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(middleware.ServiceKey(deps.ServiceAPIKey))
		deps.RunnerHandler.Routes(api)
	})

	return r
}
