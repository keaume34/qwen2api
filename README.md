# qwen2api

OpenAI-compatible API gateway for `chat.qwen.ai`, written in Go. Inspired by the architecture of [CJackHwang/ds2api](https://github.com/CJackHwang/ds2api) but rewritten from scratch for the Qwen web protocol, with a clean and minimal core (chi router + upstream client + token pool).

Languages: **English** | [Tiếng Việt](README.vi.md)

> **Disclaimer**: This project is for learning, research, and personal experimentation only. It is not affiliated with Alibaba Cloud / Qwen. Use at your own risk; you are responsible for complying with the upstream service's Terms of Service and any applicable laws. No warranty of any kind.

## Features

- **OpenAI-compatible endpoints**:
  - `GET  /v1/models`
  - `POST /v1/chat/completions` (streaming + non-streaming)
  - `GET  /healthz`, `GET  /readyz`
- **Multi-token pool** with round-robin rotation and per-token cooldown on failure
- **Static token config** (env or JSON file) **or** auto-login via the bundled Playwright helper (`scripts/login.mjs`)
- **Streaming** translation between upstream Qwen SSE and OpenAI-style `data: ...` chunks
- **Thinking-mode passthrough**: emits `<think>...</think>` segments inside the assistant content, matching common OpenAI proxies
- **Docker**, `docker-compose`, and **Vercel** deployment supported
- Zero external runtime dependencies beyond the Go standard library + [chi](https://github.com/go-chi/chi) router

## Quick Start

### 1. Get a Qwen token

1. Open <https://chat.qwen.ai/> in a browser and sign in.
2. Open DevTools → Console.
3. Run `localStorage.getItem("token")` and copy the value (without the surrounding quotes).

Alternatively, use the bundled Playwright helper (Node 20+ required):

```bash
cd scripts
npm install playwright
npx playwright install chromium
node login.mjs --email you@example.com --password 'your-pass'
# Token will be printed to stdout. Save to config or pass via env.
```

### 2. Run with Docker (recommended)

```bash
docker run -d --name qwen2api -p 5001:5001 \
  -e QWEN2API_API_KEY=sk-your-local-key \
  -e QWEN2API_TOKENS='eyJ...token1,eyJ...token2' \
  ghcr.io/keaume34/qwen2api:latest
```

### 3. Run from source

```bash
go build -o qwen2api ./cmd/qwen2api
QWEN2API_API_KEY=sk-local QWEN2API_TOKENS='eyJ...' ./qwen2api
```

### 4. Call it like OpenAI

```bash
curl http://localhost:5001/v1/chat/completions \
  -H "Authorization: Bearer sk-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3-max",
    "stream": true,
    "messages": [{"role":"user","content":"Hello!"}]
  }'
```

## Configuration

Configuration is read in this order (later overrides earlier):

1. `config.json` next to the binary (or `QWEN2API_CONFIG_PATH`)
2. Environment variables

| Env var | Type | Description |
| --- | --- | --- |
| `QWEN2API_PORT` | int | Listen port (default `5001`) |
| `QWEN2API_API_KEY` | string | Comma-separated list of client API keys. If empty, the API is **unauthenticated** (not recommended). |
| `QWEN2API_TOKENS` | string | Comma-separated list of Qwen `Bearer` tokens. |
| `QWEN2API_BASE_URL` | string | Upstream base URL. Default `https://chat.qwen.ai`. |
| `QWEN2API_SSXMOD_ITNA` | string | Optional `ssxmod_itna` cookie value (anti-bot fingerprint). |
| `QWEN2API_SSXMOD_ITNA2` | string | Optional `ssxmod_itna2` cookie value. |
| `QWEN2API_USER_AGENT` | string | Override the upstream `User-Agent`. |
| `QWEN2API_TIMEOUT_SECONDS` | int | Upstream non-stream timeout (default `120`). |
| `QWEN2API_LOG_LEVEL` | `debug`/`info`/`warn`/`error` | Default `info`. |
| `QWEN2API_COOLDOWN_SECONDS` | int | Per-token cooldown after a failed request (default `60`). |

### `config.json` example

```json
{
  "port": 5001,
  "api_keys": ["sk-local-1", "sk-local-2"],
  "tokens": [
    {"value": "eyJ...token1", "name": "primary"},
    {"value": "eyJ...token2", "name": "backup"}
  ],
  "base_url": "https://chat.qwen.ai",
  "ssxmod_itna": "",
  "ssxmod_itna2": ""
}
```

## Architecture

```
client ──▶ chi router ──▶ auth middleware ──▶ openai handler
                                                    │
                                                    ├── convert OpenAI request → Qwen request
                                                    │
                                                    ├── tokenpool.Take() ──▶ qwen.Client
                                                    │                          │
                                                    │                          ├── POST /api/v2/chats/new   (allocate chat_id)
                                                    │                          └── POST /api/v2/chat/completions?chat_id=...
                                                    │                                       │
                                                    │                                       └── upstream SSE (always stream)
                                                    │
                                                    └── translate Qwen SSE → OpenAI SSE / aggregate for non-stream
```

Key design choices:

- **Upstream always streams**: `chat.qwen.ai` doesn't reliably support `stream=false`. The server always requests streaming from upstream and either re-emits to the client (stream) or aggregates into a final `chat.completion` (non-stream).
- **Per-token rotation**: requests pick the next healthy token. A token that returns 401/403 is put on cooldown.
- **No PoW**: Qwen does not use a DeepSeek-style proof-of-work challenge.
- **`ssxmod_itna*` cookies are optional**: many endpoints work without them. If your IP gets rate-limited, copy the cookies from your browser and set them via env or config.

## Endpoints

### `GET /v1/models`

Returns the dynamic list from `GET https://chat.qwen.ai/api/models`, reshaped into the OpenAI `Model` schema. Falls back to a static list if upstream is unreachable.

### `POST /v1/chat/completions`

OpenAI Chat Completions. Supported request fields:

- `model` (required) — e.g. `qwen3-max`, `qwen-plus`, `qwen3-coder-plus`, `qwen-max-latest`
- `messages` (required) — `system` / `user` / `assistant` roles, text content
- `stream` (bool) — both true and false are supported
- `temperature`, `top_p`, `max_tokens` — currently advisory (forwarded as-is when upstream accepts them)
- `enable_thinking` (bool, optional, Qwen extension)

Multimodal image inputs (`content` arrays with `image_url`) are accepted but the URL is forwarded as-is; the project does not currently upload images to Qwen OSS — for that, see [Rfym21/Qwen2API](https://github.com/Rfym21/Qwen2API).

## Development

```bash
go build ./...
go vet ./...
gofmt -l .
go test ./...
```

A reference GitHub Actions workflow lives at [`docs/ci.yml.example`](docs/ci.yml.example) — copy it to `.github/workflows/ci.yml` to run the same gates on every PR.

## Deploy on Vercel

`vercel.json` builds the `cmd/qwen2api` binary as a Go runtime function. Set `QWEN2API_API_KEY` and `QWEN2API_TOKENS` in the Vercel dashboard.

## Acknowledgements

- Architecture inspiration: [CJackHwang/ds2api](https://github.com/CJackHwang/ds2api) (AGPL-3.0)
- Protocol reference: [Rfym21/Qwen2API](https://github.com/Rfym21/Qwen2API)

This project does **not** copy code from those repositories; it implements the chat.qwen.ai protocol independently in Go and is licensed under MIT (see `LICENSE`).
