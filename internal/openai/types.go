// Package openai defines the OpenAI Chat Completions wire types that
// qwen2api accepts from clients and emits back to them.
package openai

import "encoding/json"

// ChatRequest mirrors the subset of OpenAI Chat Completions used by qwen2api.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	// Qwen extensions accepted but optional.
	EnableThinking *bool `json:"enable_thinking,omitempty"`
}

// ChatMessage allows both plain-string and array (multimodal) content.
type ChatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Name    string          `json:"name,omitempty"`
}

// Text returns the plain text view of Content. Multimodal arrays are flattened
// by concatenating any `text` parts.
func (m ChatMessage) Text() string {
	if len(m.Content) == 0 {
		return ""
	}
	// Try string first.
	var s string
	if err := json.Unmarshal(m.Content, &s); err == nil {
		return s
	}
	// Try array of parts.
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(m.Content, &parts); err == nil {
		out := ""
		for _, p := range parts {
			if p.Type == "text" || p.Type == "" {
				out += p.Text
			}
		}
		return out
	}
	return ""
}

// ChatCompletion is the non-streaming response envelope.
type ChatCompletion struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice is one completion in the non-streaming envelope.
type Choice struct {
	Index        int            `json:"index"`
	Message      ChatMessageOut `json:"message"`
	FinishReason string         `json:"finish_reason"`
}

// ChatMessageOut is the assistant message returned to the client.
type ChatMessageOut struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Usage carries token accounting.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk is one SSE chunk emitted to the client.
type StreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// StreamChoice is one delta in a stream chunk.
type StreamChoice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason,omitempty"`
}

// Delta is the incremental content for a stream chunk.
type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// ModelList is the OpenAI /v1/models envelope.
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// Model is one entry of /v1/models.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ErrorEnvelope is the OpenAI-style error response.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody describes the error.
type ErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}
