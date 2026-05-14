// Package config loads qwen2api runtime configuration from environment
// variables and an optional JSON file. Environment variables take precedence.
package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Token is a single upstream Qwen credential.
type Token struct {
	Value string `json:"value"`
	Name  string `json:"name,omitempty"`
}

// Config holds the resolved runtime configuration.
type Config struct {
	Port            int      `json:"port"`
	APIKeys         []string `json:"api_keys"`
	Tokens          []Token  `json:"tokens"`
	BaseURL         string   `json:"base_url"`
	SsxmodItna      string   `json:"ssxmod_itna"`
	SsxmodItna2     string   `json:"ssxmod_itna2"`
	UserAgent       string   `json:"user_agent"`
	TimeoutSeconds  int      `json:"timeout_seconds"`
	CooldownSeconds int      `json:"cooldown_seconds"`
	LogLevel        string   `json:"log_level"`
}

// Default returns the baseline configuration. Empty slices indicate
// "no value configured", which callers decide how to surface.
func Default() Config {
	return Config{
		Port:            5001,
		BaseURL:         "https://chat.qwen.ai",
		UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36 Edg/143.0.0.0",
		TimeoutSeconds:  120,
		CooldownSeconds: 60,
		LogLevel:        "info",
	}
}

// Load resolves config from $QWEN2API_CONFIG_PATH (or ./config.json if present)
// and overlays environment variables on top.
func Load() (Config, error) {
	cfg := Default()
	path := os.Getenv("QWEN2API_CONFIG_PATH")
	if path == "" {
		if _, err := os.Stat("config.json"); err == nil {
			path = "config.json"
		}
	}
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("read %s: %w", path, err)
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return cfg, fmt.Errorf("parse %s: %w", path, err)
		}
	}

	if v := os.Getenv("QWEN2API_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("QWEN2API_PORT: %w", err)
		}
		cfg.Port = n
	}
	if v := os.Getenv("QWEN2API_API_KEY"); v != "" {
		cfg.APIKeys = splitCSV(v)
	}
	if v := os.Getenv("QWEN2API_TOKENS"); v != "" {
		for _, raw := range splitCSV(v) {
			cfg.Tokens = append(cfg.Tokens, Token{Value: raw})
		}
	}
	if v := os.Getenv("QWEN2API_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("QWEN2API_SSXMOD_ITNA"); v != "" {
		cfg.SsxmodItna = v
	}
	if v := os.Getenv("QWEN2API_SSXMOD_ITNA2"); v != "" {
		cfg.SsxmodItna2 = v
	}
	if v := os.Getenv("QWEN2API_USER_AGENT"); v != "" {
		cfg.UserAgent = v
	}
	if v := os.Getenv("QWEN2API_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("QWEN2API_TIMEOUT_SECONDS: %w", err)
		}
		cfg.TimeoutSeconds = n
	}
	if v := os.Getenv("QWEN2API_COOLDOWN_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("QWEN2API_COOLDOWN_SECONDS: %w", err)
		}
		cfg.CooldownSeconds = n
	}
	if v := os.Getenv("QWEN2API_LOG_LEVEL"); v != "" {
		cfg.LogLevel = strings.ToLower(v)
	}

	if cfg.Port <= 0 || cfg.Port > 65535 {
		return cfg, fmt.Errorf("invalid port: %d", cfg.Port)
	}
	return cfg, nil
}

// SlogLevel maps the textual level to a slog.Leveler.
func (c Config) SlogLevel() slog.Level {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// AuthorizedKey returns true if the given client key is allowed. When no API
// keys are configured the server runs in open mode and every request is
// accepted.
func (c Config) AuthorizedKey(key string) bool {
	if len(c.APIKeys) == 0 {
		return true
	}
	for _, k := range c.APIKeys {
		if k != "" && key == k {
			return true
		}
	}
	return false
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
