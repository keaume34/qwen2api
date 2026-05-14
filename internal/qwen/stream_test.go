package qwen

import (
	"io"
	"strings"
	"testing"
)

func TestStreamReaderParsesEvents(t *testing.T) {
	body := `: keepalive

data: {"choices":[{"delta":{"role":"assistant","content":"Hello"}}]}

data: {"choices":[{"delta":{"content":" world"}}]}

data: [DONE]

`
	r := NewStreamReader(strings.NewReader(body))
	var contents []string
	doneSeen := false
	for {
		evt, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if evt.Done {
			doneSeen = true
			continue
		}
		if evt.Delta != nil && len(evt.Delta.Choices) > 0 {
			contents = append(contents, evt.Delta.Choices[0].Delta.Content)
		}
	}
	got := strings.Join(contents, "")
	if got != "Hello world" {
		t.Errorf("got %q want %q", got, "Hello world")
	}
	if !doneSeen {
		t.Error("[DONE] sentinel not surfaced")
	}
}

func TestStreamReaderTolerantToBadChunk(t *testing.T) {
	body := "data: not-json\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"
	r := NewStreamReader(strings.NewReader(body))
	var got string
	for {
		evt, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if evt.Delta != nil && len(evt.Delta.Choices) > 0 {
			got += evt.Delta.Choices[0].Delta.Content
		}
	}
	if got != "ok" {
		t.Errorf("got %q want %q", got, "ok")
	}
}
