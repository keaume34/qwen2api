// Package handler exposes qwen2api as a Vercel Go serverless function.
//
// Vercel's Go runtime invokes the exported `Handler` symbol for every request.
// We construct the chi router lazily on first invocation so that the
// configuration is read from the environment exposed by Vercel.
package handler

import (
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/keaume34/qwen2api/internal/config"
	"github.com/keaume34/qwen2api/internal/qwen"
	"github.com/keaume34/qwen2api/internal/server"
	"github.com/keaume34/qwen2api/internal/tokenpool"
)

var (
	once       sync.Once
	httpServer http.Handler
	initErr    error
)

func build() {
	cfg, err := config.Load()
	if err != nil {
		initErr = err
		return
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.SlogLevel()}))
	pool := tokenpool.New(cfg.Tokens, time.Duration(cfg.CooldownSeconds)*time.Second)
	client := qwen.NewClient(qwen.ClientConfig{
		BaseURL:        cfg.BaseURL,
		UserAgent:      cfg.UserAgent,
		SsxmodItna:     cfg.SsxmodItna,
		Ssxmodi2:       cfg.SsxmodItna2,
		TimeoutSeconds: cfg.TimeoutSeconds,
	})
	httpServer = server.New(server.Deps{
		Config:    cfg,
		Logger:    logger,
		Qwen:      client,
		TokenPool: pool,
	})
}

// Handler is the entrypoint invoked by Vercel's Go runtime.
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(build)
	if initErr != nil {
		http.Error(w, "qwen2api init error: "+initErr.Error(), http.StatusInternalServerError)
		return
	}
	httpServer.ServeHTTP(w, r)
}
