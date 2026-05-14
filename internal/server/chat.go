package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/keaume34/qwen2api/internal/openai"
	"github.com/keaume34/qwen2api/internal/qwen"
)

func (h *handlers) chatCompletions(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()
	var req openai.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON body: "+err.Error())
		return
	}
	if req.Model == "" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "messages must not be empty")
		return
	}

	token, err := h.deps.TokenPool.Take()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "no_upstream_token", "no Qwen token configured; set QWEN2API_TOKENS")
		return
	}

	upstreamReq := buildQwenRequest(req)
	chatID, err := h.deps.Qwen.NewChat(r.Context(), token.Value, upstreamReq.Model, upstreamReq.ChatType)
	if err != nil {
		h.handleUpstreamFailure(w, token.Value, err, "create chat session")
		return
	}
	upstreamReq.ChatID = chatID

	body, err := h.deps.Qwen.Completions(r.Context(), token.Value, upstreamReq)
	if err != nil {
		h.handleUpstreamFailure(w, token.Value, err, "open completion stream")
		return
	}
	defer func() {
		_ = body.Close()
	}()

	completionID := "chatcmpl-" + uuid.NewString()
	created := unixNow()
	if req.Stream {
		h.proxyStream(w, body, completionID, created, req.Model)
		return
	}
	h.aggregateStream(w, body, completionID, created, req.Model)
}

func (h *handlers) handleUpstreamFailure(w http.ResponseWriter, token string, err error, action string) {
	var upstream *qwen.UpstreamError
	if errors.As(err, &upstream) {
		h.deps.Logger.Warn("upstream error", "action", action, "status", upstream.Status, "body", truncate(upstream.Body, 256))
		if upstream.Status == http.StatusUnauthorized || upstream.Status == http.StatusForbidden {
			h.deps.TokenPool.MarkBad(token)
		}
		writeError(w, http.StatusBadGateway, "upstream_error", fmt.Sprintf("qwen %s failed: %d", action, upstream.Status))
		return
	}
	h.deps.Logger.Error("transport error", "action", action, "err", err)
	writeError(w, http.StatusBadGateway, "upstream_error", "transport error: "+err.Error())
}

func buildQwenRequest(req openai.ChatRequest) qwen.CompletionRequest {
	chatType := chatTypeFromModel(req.Model)
	thinkingEnabled := false
	if req.EnableThinking != nil {
		thinkingEnabled = *req.EnableThinking
	} else if strings.HasSuffix(req.Model, "-thinking") || strings.Contains(req.Model, "thinking") {
		thinkingEnabled = true
	}

	msgs := make([]qwen.Message, 0, len(req.Messages))
	for i, m := range req.Messages {
		qm := qwen.Message{
			Role:     m.Role,
			Content:  m.Text(),
			ChatType: chatType,
		}
		// Only the last user message in the conversation triggers the
		// `feature_config.thinking_enabled` toggle upstream — but mirroring
		// it on every message is harmless and matches the reference clients.
		_ = i
		if thinkingEnabled {
			qm.FeatureConfig = &qwen.FeatureConfig{ThinkingEnabled: true}
		}
		msgs = append(msgs, qm)
	}

	return qwen.CompletionRequest{
		ChatType:    chatType,
		SubChatType: chatType,
		ChatMode:    "normal",
		Model:       baseModelID(req.Model),
		Messages:    msgs,
		SessionID:   uuid.NewString(),
		ID:          uuid.NewString(),
	}
}

func chatTypeFromModel(model string) string {
	switch {
	case strings.Contains(model, "search"):
		return "search"
	case strings.Contains(model, "image"):
		return "t2i"
	case strings.Contains(model, "video"):
		return "t2v"
	default:
		return "t2t"
	}
}

// baseModelID strips qwen2api-specific suffixes (e.g. `-thinking`, `-search`)
// before forwarding to the upstream model field.
func baseModelID(model string) string {
	suffixes := []string{"-thinking-search", "-image-edit", "-deep-research", "-thinking", "-search", "-video", "-image"}
	for _, s := range suffixes {
		if strings.HasSuffix(model, s) {
			return strings.TrimSuffix(model, s)
		}
	}
	return model
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// proxyStream re-emits upstream events as OpenAI-style SSE chunks.
func (h *handlers) proxyStream(w http.ResponseWriter, body io.Reader, id string, created int64, model string) {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	reader := qwen.NewStreamReader(body)
	roleSent := false
	inThinking := false

	flush := func() {
		if flusher != nil {
			flusher.Flush()
		}
	}

	emit := func(chunk openai.StreamChunk) {
		raw, err := json.Marshal(chunk)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", raw)
		flush()
	}

	for {
		evt, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			h.deps.Logger.Warn("stream read error", "err", err)
			break
		}
		if evt.Done {
			break
		}
		if evt.Delta == nil || len(evt.Delta.Choices) == 0 {
			continue
		}
		choice := evt.Delta.Choices[0]
		role := ""
		if !roleSent {
			role = "assistant"
			roleSent = true
		}
		text := choice.Delta.Content
		text, inThinking = wrapThinking(text, choice.Delta.Phase, inThinking)
		if text == "" && role == "" && choice.FinishReason == nil {
			continue
		}
		chunk := openai.StreamChunk{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   model,
			Choices: []openai.StreamChoice{{
				Index:        0,
				Delta:        openai.Delta{Role: role, Content: text},
				FinishReason: choice.FinishReason,
			}},
		}
		emit(chunk)
	}

	// Close out an open <think> if upstream finished mid-phase.
	if inThinking {
		emit(openai.StreamChunk{
			ID: id, Object: "chat.completion.chunk", Created: created, Model: model,
			Choices: []openai.StreamChoice{{Delta: openai.Delta{Content: "</think>"}}},
		})
	}

	stop := "stop"
	emit(openai.StreamChunk{
		ID: id, Object: "chat.completion.chunk", Created: created, Model: model,
		Choices: []openai.StreamChoice{{Index: 0, Delta: openai.Delta{}, FinishReason: &stop}},
	})
	fmt.Fprint(w, "data: [DONE]\n\n")
	flush()
}

// wrapThinking inserts <think>/</think> markers when upstream toggles phase
// between "think" and "answer". Returns the updated text and the new
// `inThinking` flag.
func wrapThinking(content, phase string, inThinking bool) (string, bool) {
	switch phase {
	case "think":
		if !inThinking {
			return "<think>" + content, true
		}
		return content, true
	case "answer", "":
		if inThinking {
			return "</think>" + content, false
		}
		return content, false
	default:
		return content, inThinking
	}
}

// aggregateStream consumes the upstream stream and returns a single
// chat.completion JSON envelope (for non-stream client requests).
func (h *handlers) aggregateStream(w http.ResponseWriter, body io.Reader, id string, created int64, model string) {
	reader := qwen.NewStreamReader(body)
	var content strings.Builder
	inThinking := false
	finishReason := "stop"

	for {
		evt, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			h.deps.Logger.Warn("stream read error", "err", err)
			break
		}
		if evt.Done {
			break
		}
		if evt.Delta == nil || len(evt.Delta.Choices) == 0 {
			continue
		}
		choice := evt.Delta.Choices[0]
		text, next := wrapThinking(choice.Delta.Content, choice.Delta.Phase, inThinking)
		inThinking = next
		content.WriteString(text)
		if choice.FinishReason != nil {
			finishReason = *choice.FinishReason
		}
	}
	if inThinking {
		content.WriteString("</think>")
	}

	resp := openai.ChatCompletion{
		ID:      id,
		Object:  "chat.completion",
		Created: created,
		Model:   model,
		Choices: []openai.Choice{{
			Index:        0,
			Message:      openai.ChatMessageOut{Role: "assistant", Content: content.String()},
			FinishReason: finishReason,
		}},
	}
	writeJSON(w, http.StatusOK, resp)
}
