// Command qwen2api launches the OpenAI-compatible HTTP gateway for chat.qwen.ai.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/keaume34/qwen2api/internal/config"
	"github.com/keaume34/qwen2api/internal/qwen"
	"github.com/keaume34/qwen2api/internal/server"
	"github.com/keaume34/qwen2api/internal/tokenpool"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "qwen2api:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.SlogLevel()}))
	slog.SetDefault(logger)

	if len(cfg.Tokens) == 0 {
		logger.Warn("starting with empty token pool; chat completions will fail until tokens are configured")
	}
	if len(cfg.APIKeys) == 0 {
		logger.Warn("starting without QWEN2API_API_KEY; the server is UNAUTHENTICATED")
	}

	pool := tokenpool.New(cfg.Tokens, time.Duration(cfg.CooldownSeconds)*time.Second)
	client := qwen.NewClient(qwen.ClientConfig{
		BaseURL:        cfg.BaseURL,
		UserAgent:      cfg.UserAgent,
		SsxmodItna:     cfg.SsxmodItna,
		Ssxmodi2:       cfg.SsxmodItna2,
		TimeoutSeconds: cfg.TimeoutSeconds,
	})

	srv := server.New(server.Deps{
		Config:    cfg,
		Logger:    logger,
		Qwen:      client,
		TokenPool: pool,
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv,
		ReadHeaderTimeout: 15 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("qwen2api listening", "addr", addr, "tokens", len(cfg.Tokens), "api_keys", len(cfg.APIKeys))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
