package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/keaume34/qwen2api/internal/config"
	"github.com/keaume34/qwen2api/internal/qwen"
	"github.com/keaume34/qwen2api/internal/tokenpool"
)

func newTestServer(t *testing.T, upstream *httptest.Server) http.Handler {
	t.Helper()
	cfg := config.Config{
		Port:            5001,
		APIKeys:         []string{"sk-test"},
		Tokens:          []config.Token{{Value: "tok-1"}},
		BaseURL:         upstream.URL,
		CooldownSeconds: 60,
		LogLevel:        "error",
	}
	client := qwen.NewClient(qwen.ClientConfig{
		BaseURL:        upstream.URL,
		TimeoutSeconds: 10,
	})
	pool := tokenpool.New(cfg.Tokens, time.Minute)
	return New(Deps{
		Config:    cfg,
		Logger:    slog.Default(),
		Qwen:      client,
		TokenPool: pool,
	})
}

func TestUnauthorizedWithoutKey(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()
	h := newTestServer(t, upstream)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got %d want 401", rec.Code)
	}
}

func TestHealth(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()
	h := newTestServer(t, upstream)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got %d want 200", rec.Code)
	}
}

// fakeUpstream mounts the minimum chat.qwen.ai endpoints used by qwen2api.
func fakeUpstream(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/chats/new", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"id":"chat-abc"}}`))
	})
	mux.HandleFunc("/api/v2/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		writeData := func(s string) {
			fmt.Fprintf(w, "data: %s\n\n", s)
			if fl != nil {
				fl.Flush()
			}
		}
		writeData(`{"choices":[{"delta":{"role":"assistant","content":"Hel","phase":"answer"}}]}`)
		writeData(`{"choices":[{"delta":{"content":"lo","phase":"answer"}}]}`)
		writeData(`[DONE]`)
	})
	mux.HandleFunc("/api/models", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen3-max"},{"id":"qwen-plus"}]}`))
	})
	return httptest.NewServer(mux)
}

func TestChatCompletionsNonStream(t *testing.T) {
	upstream := fakeUpstream(t)
	defer upstream.Close()
	h := newTestServer(t, upstream)

	body := `{"model":"qwen3-max","stream":false,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer sk-test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	choices, _ := resp["choices"].([]any)
	if len(choices) != 1 {
		t.Fatalf("choices = %v", choices)
	}
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	if msg["content"] != "Hello" {
		t.Errorf("content = %q want Hello", msg["content"])
	}
}

func TestChatCompletionsStream(t *testing.T) {
	upstream := fakeUpstream(t)
	defer upstream.Close()
	h := newTestServer(t, upstream)

	body := `{"model":"qwen3-max","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer sk-test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	out, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(out), "data: [DONE]") {
		t.Errorf("stream missing [DONE]; got: %s", out)
	}
	if !strings.Contains(string(out), `"role":"assistant"`) {
		t.Errorf("stream missing role; got: %s", out)
	}
	if !strings.Contains(string(out), `Hel`) || !strings.Contains(string(out), `lo`) {
		t.Errorf("stream missing content chunks; got: %s", out)
	}
}

func TestModelsListUpstream(t *testing.T) {
	upstream := fakeUpstream(t)
	defer upstream.Close()
	h := newTestServer(t, upstream)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer sk-test")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "qwen3-max") {
		t.Errorf("expected qwen3-max in body; got %s", rec.Body.String())
	}
}

func TestWrapThinking(t *testing.T) {
	cases := []struct {
		content  string
		phase    string
		thinkIn  bool
		want     string
		thinkOut bool
	}{
		{"abc", "think", false, "<think>abc", true},
		{"def", "think", true, "def", true},
		{"ghi", "answer", true, "</think>ghi", false},
		{"jkl", "answer", false, "jkl", false},
		{"", "", false, "", false},
	}
	for i, c := range cases {
		got, gotIn := wrapThinking(c.content, c.phase, c.thinkIn)
		if got != c.want || gotIn != c.thinkOut {
			t.Errorf("case %d: got (%q,%v) want (%q,%v)", i, got, gotIn, c.want, c.thinkOut)
		}
	}
}
