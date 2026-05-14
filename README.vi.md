# qwen2api

Cổng API tương thích OpenAI cho `chat.qwen.ai`, viết bằng Go. Lấy cảm hứng kiến trúc từ [CJackHwang/ds2api](https://github.com/CJackHwang/ds2api) nhưng viết lại hoàn toàn từ đầu cho protocol Qwen web, core gọn nhẹ (chi router + upstream client + token pool).

Ngôn ngữ: [English](README.md) | **Tiếng Việt**

> **Disclaimer**: Project này chỉ dùng cho mục đích học tập, nghiên cứu, thử nghiệm cá nhân. Không liên kết với Alibaba Cloud / Qwen. Bạn tự chịu trách nhiệm tuân thủ Terms of Service của upstream và luật pháp liên quan. Không bảo hành.

## Tính năng

- **Endpoint tương thích OpenAI**:
  - `GET  /v1/models`
  - `POST /v1/chat/completions` (cả streaming và non-streaming)
  - `GET  /healthz`, `GET  /readyz`
- **Token pool đa tài khoản**, xoay vòng round-robin, cooldown từng token khi lỗi
- Cấu hình token **tĩnh** (env hoặc JSON) **hoặc** auto-login qua script Playwright (`scripts/login.mjs`)
- **Streaming** chuyển đổi giữa Qwen SSE upstream và format OpenAI `data: ...`
- **Thinking-mode passthrough**: nhả các đoạn `<think>...</think>` trong nội dung assistant
- Hỗ trợ **Docker**, `docker-compose`, **Vercel**
- Runtime gọn: chỉ stdlib Go + router [chi](https://github.com/go-chi/chi)

## Bắt đầu nhanh

### 1. Lấy token Qwen

1. Mở <https://chat.qwen.ai/> trên trình duyệt và đăng nhập.
2. Mở DevTools → Console.
3. Chạy `localStorage.getItem("token")` và copy giá trị (bỏ dấu ngoặc kép).

Hoặc dùng script Playwright kèm theo (cần Node 20+):

```bash
cd scripts
npm install playwright
npx playwright install chromium
node login.mjs --email you@example.com --password 'your-pass'
# Token sẽ in ra stdout. Lưu vào config hoặc truyền qua env.
```

### 2. Chạy với Docker (khuyến nghị)

```bash
docker run -d --name qwen2api -p 5001:5001 \
  -e QWEN2API_API_KEY=sk-your-local-key \
  -e QWEN2API_TOKENS='eyJ...token1,eyJ...token2' \
  ghcr.io/keaume34/qwen2api:latest
```

### 3. Chạy từ source

```bash
go build -o qwen2api ./cmd/qwen2api
QWEN2API_API_KEY=sk-local QWEN2API_TOKENS='eyJ...' ./qwen2api
```

### 4. Gọi như OpenAI

```bash
curl http://localhost:5001/v1/chat/completions \
  -H "Authorization: Bearer sk-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3-max",
    "stream": true,
    "messages": [{"role":"user","content":"Xin chào!"}]
  }'
```

## Cấu hình

Đọc theo thứ tự (sau ghi đè trước):

1. `config.json` cùng thư mục binary (hoặc `QWEN2API_CONFIG_PATH`)
2. Biến môi trường

| Env var | Kiểu | Mô tả |
| --- | --- | --- |
| `QWEN2API_PORT` | int | Port lắng nghe (mặc định `5001`) |
| `QWEN2API_API_KEY` | string | Danh sách API key của client, cách nhau bởi dấu phẩy. Bỏ trống = **không xác thực** (không khuyến nghị). |
| `QWEN2API_TOKENS` | string | Danh sách token Bearer Qwen, cách nhau bởi dấu phẩy. |
| `QWEN2API_BASE_URL` | string | URL upstream. Mặc định `https://chat.qwen.ai`. |
| `QWEN2API_SSXMOD_ITNA` | string | Cookie `ssxmod_itna` (anti-bot fingerprint, tùy chọn). |
| `QWEN2API_SSXMOD_ITNA2` | string | Cookie `ssxmod_itna2` (tùy chọn). |
| `QWEN2API_USER_AGENT` | string | Override `User-Agent`. |
| `QWEN2API_TIMEOUT_SECONDS` | int | Timeout request non-stream (mặc định `120`). |
| `QWEN2API_LOG_LEVEL` | `debug`/`info`/`warn`/`error` | Mặc định `info`. |
| `QWEN2API_COOLDOWN_SECONDS` | int | Cooldown từng token sau lỗi (mặc định `60`). |

## Kiến trúc

```
client ──▶ chi router ──▶ auth middleware ──▶ openai handler
                                                    │
                                                    ├── convert OpenAI → Qwen
                                                    │
                                                    ├── tokenpool.Take() ──▶ qwen.Client
                                                    │                          │
                                                    │                          ├── POST /api/v2/chats/new
                                                    │                          └── POST /api/v2/chat/completions?chat_id=…
                                                    │
                                                    └── chuyển Qwen SSE → OpenAI SSE / gộp cho non-stream
```

Lựa chọn thiết kế:

- **Upstream luôn stream**: `chat.qwen.ai` không hỗ trợ ổn định `stream=false`. Server luôn yêu cầu stream từ upstream, sau đó re-emit cho client (stream) hoặc gộp thành `chat.completion` (non-stream).
- **Xoay token theo round-robin**: token nào 401/403 sẽ bị cooldown.
- **Không có PoW**: Qwen không dùng challenge proof-of-work kiểu DeepSeek.
- **Cookie `ssxmod_itna*` tùy chọn**: nhiều endpoint chạy được mà không cần. Nếu IP bị rate-limit, copy cookie từ browser vào env.

## License

MIT — xem `LICENSE`.

## Ghi nhận

- Cảm hứng kiến trúc: [CJackHwang/ds2api](https://github.com/CJackHwang/ds2api) (AGPL-3.0)
- Tham khảo protocol: [Rfym21/Qwen2API](https://github.com/Rfym21/Qwen2API)

Project này **không** copy code từ hai repo trên; tự implement protocol chat.qwen.ai bằng Go và license MIT.
