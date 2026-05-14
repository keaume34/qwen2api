package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/keaume34/qwen2api/internal/openai"
)

type handlers struct {
	deps Deps
}

func (h *handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handlers) ready(w http.ResponseWriter, _ *http.Request) {
	status := http.StatusOK
	body := map[string]any{"status": "ok", "tokens": h.deps.TokenPool.Size()}
	if h.deps.TokenPool.Size() == 0 {
		status = http.StatusServiceUnavailable
		body["status"] = "no-tokens"
	}
	writeJSON(w, status, body)
}

// authMiddleware enforces the configured client API keys when any are set.
func (h *handlers) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(h.deps.Config.APIKeys) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		key := bearerOrQuery(r)
		if !h.deps.Config.AuthorizedKey(key) {
			writeError(w, http.StatusUnauthorized, "invalid_api_key", "missing or invalid API key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerOrQuery(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(h, "Bearer ") {
			return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		}
		return strings.TrimSpace(h)
	}
	if k := r.Header.Get("X-Api-Key"); k != "" {
		return strings.TrimSpace(k)
	}
	return strings.TrimSpace(r.URL.Query().Get("api_key"))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(openai.ErrorEnvelope{
		Error: openai.ErrorBody{Message: msg, Type: code, Code: code},
	})
}

func unixNow() int64 { return time.Now().Unix() }
