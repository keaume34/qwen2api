package openai

import (
	"encoding/json"
	"testing"
)

func TestChatMessageText(t *testing.T) {
	cases := map[string]string{
		`"hello"`: "hello",
		`[{"type":"text","text":"a"},{"type":"text","text":"b"}]`: "ab",
		`[{"type":"image_url","image_url":{"url":"x"}}]`:          "",
		`""`: "",
	}
	for in, want := range cases {
		m := ChatMessage{Role: "user", Content: json.RawMessage(in)}
		if got := m.Text(); got != want {
			t.Errorf("Text(%s) = %q want %q", in, got, want)
		}
	}
}
