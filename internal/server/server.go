// Package server wires the chi router, middleware, and OpenAI-compatible
// handlers for qwen2api.
package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/keaume34/qwen2api/internal/config"
	"github.com/keaume34/qwen2api/internal/qwen"
	"github.com/keaume34/qwen2api/internal/tokenpool"
)

// Deps holds the wired dependencies for the HTTP layer.
type Deps struct {
	Config    config.Config
	Logger    *slog.Logger
	Qwen      *qwen.Client
	TokenPool *tokenpool.Pool
}

// New returns the configured http.Handler.
func New(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	h := &handlers{deps: deps}

	r.Get("/healthz", h.health)
	r.Get("/readyz", h.ready)

	// OpenAI-compatible surface, accessible both at /v1/* and at the root for
	// clients that strip the version prefix.
	r.Group(func(r chi.Router) {
		r.Use(h.authMiddleware)
		r.Get("/v1/models", h.listModels)
		r.Get("/models", h.listModels)
		r.Post("/v1/chat/completions", h.chatCompletions)
		r.Post("/chat/completions", h.chatCompletions)
	})

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
