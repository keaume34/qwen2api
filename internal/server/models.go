package server

import (
	"net/http"

	"github.com/keaume34/qwen2api/internal/openai"
	"github.com/keaume34/qwen2api/internal/qwen"
)

// staticModels is the fallback model list used when the upstream /api/models
// call is unavailable (e.g. no token configured yet).
var staticModels = []string{
	"qwen3-max",
	"qwen3-max-latest",
	"qwen-max",
	"qwen-max-latest",
	"qwen-plus",
	"qwen-plus-latest",
	"qwen-turbo",
	"qwen3-coder-plus",
	"qwen3-235b-a22b",
}

func (h *handlers) listModels(w http.ResponseWriter, r *http.Request) {
	now := unixNow()
	out := openai.ModelList{Object: "list"}

	token, err := h.deps.TokenPool.Take()
	if err == nil {
		resp, errModels := h.deps.Qwen.Models(r.Context(), token.Value)
		if errModels == nil {
			seen := map[string]bool{}
			for _, m := range resp.Data {
				if m.ID == "" || seen[m.ID] {
					continue
				}
				seen[m.ID] = true
				out.Data = append(out.Data, openai.Model{
					ID:      m.ID,
					Object:  "model",
					Created: now,
					OwnedBy: "qwen",
				})
			}
		} else {
			h.deps.Logger.Warn("upstream models fetch failed; falling back to static list", "err", errModels)
			if _, ok := errModels.(*qwen.UpstreamError); ok {
				h.deps.TokenPool.MarkBad(token.Value)
			}
		}
	}

	if len(out.Data) == 0 {
		for _, id := range staticModels {
			out.Data = append(out.Data, openai.Model{
				ID:      id,
				Object:  "model",
				Created: now,
				OwnedBy: "qwen",
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}
