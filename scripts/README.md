# scripts/

Helper utilities. None of these are required for the Go server to run; they
exist purely to assist with provisioning tokens.

## `login.mjs` — Playwright auto-login

Drives `chat.qwen.ai` with a headless Chromium and extracts the auth token
from `localStorage` after sign-in.

```bash
npm install playwright
npx playwright install chromium
node login.mjs --email you@example.com --password 'your-pass'
```

The token is printed to stdout on success. Wire the result into
`QWEN2API_TOKENS` (comma-separated for multiple accounts) or `config.json`.

> The login page DOM can change without notice. If the selectors stop
> matching, run with `--headless false` and update the selectors in
> `login.mjs`.
