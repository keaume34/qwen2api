// Package qwen wraps the upstream chat.qwen.ai HTTP/SSE protocol.
package qwen

// Message is a single chat turn sent upstream. Upstream requires `extra`
// (object) and `feature_config` (with `output_schema` and `thinking_enabled`)
// on every message — leaving them out yields a 400 Bad_Request.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	// ChatType pins the per-message chat type (Qwen extension). Optional.
	ChatType string `json:"chat_type,omitempty"`
	// Extra is required by upstream (even if empty).
	Extra map[string]interface{} `json:"extra"`
	// FeatureConfig.thinking_enabled toggles the thinking mode for that turn.
	FeatureConfig *FeatureConfig `json:"feature_config"`
}

// FeatureConfig matches the upstream `feature_config` schema.
type FeatureConfig struct {
	ThinkingEnabled bool   `json:"thinking_enabled"`
	OutputSchema    string `json:"output_schema"`
}

// CompletionRequest is the payload sent to /api/v2/chat/completions.
type CompletionRequest struct {
	Stream            bool      `json:"stream"`
	IncrementalOutput bool      `json:"incremental_output"`
	ChatType          string    `json:"chat_type"`
	SubChatType       string    `json:"sub_chat_type"`
	ChatMode          string    `json:"chat_mode"`
	Model             string    `json:"model"`
	Messages          []Message `json:"messages"`
	SessionID         string    `json:"session_id"`
	ID                string    `json:"id"`
	ChatID            string    `json:"chat_id,omitempty"`
}

// NewChatRequest is the payload for /api/v2/chats/new.
type NewChatRequest struct {
	Title     string   `json:"title"`
	Models    []string `json:"models"`
	ChatMode  string   `json:"chat_mode"`
	ChatType  string   `json:"chat_type"`
	Timestamp int64    `json:"timestamp"`
}

// NewChatResponse mirrors the relevant fields of the upstream /chats/new reply.
type NewChatResponse struct {
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

// StreamDelta is one incremental chunk returned by upstream SSE.
type StreamDelta struct {
	Choices []struct {
		Delta struct {
			Role    string `json:"role"`
			Content string `json:"content"`
			// Phase is "answer" or "think" — Qwen embeds reasoning content in a separate phase.
			Phase string `json:"phase,omitempty"`
			Name  string `json:"name,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Usage *Usage `json:"usage,omitempty"`
}

// Usage carries upstream token accounting when reported.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ModelsResponse mirrors GET /api/models.
type ModelsResponse struct {
	Data []ModelInfo `json:"data"`
}

// ModelInfo is one entry from /api/models.
type ModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}
