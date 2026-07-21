<p align="center">
  <h1 align="center">⚡ CodeBuddy CLI Gateway</h1>
  <p align="center">Lightweight, high-performance Go gateway for CodeBuddy Global API. Single binary, zero runtime deps, built for CLI terminal.</p>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/go-1.23+-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/binary-11MB-blue?style=flat-square" alt="Binary Size">
  <img src="https://img.shields.io/badge/port-2069-orange?style=flat-square" alt="Port">
  <img src="https://img.shields.io/badge/models-34-purple?style=flat-square" alt="Models">
</p>

---

## ✨ Features

- 🚀 **Single Binary** — no Node.js, no Python, no runtime dependencies
- 🔄 **Smart Key Rotation** — round-robin + auto-retry on 401/403/429/5xx
- 🌐 **Proxy Support** — HTTP, HTTPS, SOCKS5, SOCKS5H via `.env`
- 💻 **TUI Dashboard** — live request log, key pool status, usage stats
- 🤖 **Auto Setup** — interactive wizard when API key is missing
- 🔐 **Router Auth** — auto-generated per-machine `rtr_` key
- 🔁 **Setup Retry** — re-enter keys on validation failure, no restart needed
- 📡 **Connection Test** — validates API key on startup
- 🎯 **OpenAI-Compatible** — drop-in for any OpenAI-compatible tool

---

## 📦 Quick Start

### Install Global Command

```bash
sudo cp cb-router /usr/local/bin/buddy
```

### Run

```bash
buddy
```

### Run in Background (screen)

```bash
screen -S buddy buddy
```

- `Ctrl+A` then `D` — detach (buddy keeps running)
- `screen -r buddy` — reattach
- `screen -ls` — list sessions

### First Run — Interactive Setup

If no API key is detected, the setup wizard runs automatically:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  🚀 CodeBuddy CLI Gateway — Setup
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Enter your CodeBuddy API key(s) (ck_...)
Press Enter on an empty line when done.

Key: ck_your_api_key_here
   Checking... ✅ key valid, CodeBuddy reachable
   Key #1 saved.

Key:
```

### Setup Retry on Failure

If the API key fails validation, you are prompted to re-enter — no restart needed:

```
🔍 Validating CodeBuddy API key... ❌ invalid key (401/403)

⚠️  Key validation failed. Press Enter to re-enter keys, or type 'q' to quit.
```

---

## ⚙️ Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `2069` | Listening port |
| `API_KEYS` | — | Comma-separated CodeBuddy API keys (`ck_...`) |
| `PROXY` | *(empty/direct)* | HTTP/HTTPS/SOCKS5 proxy URL |

> **Router Auth Key** (`rtr_...`) is auto-generated per-machine using `hostname + machine-id`. Stable across restarts, unique per device. No `.env` entry needed.

### Example `.env`

```env
PORT=2069
API_KEYS=ck_your_key_here,ck_another_key_here
PROXY=socks5://user:pass@host:1080
```

### Proxy Formats

```env
PROXY=http://proxy.example.com:8080
PROXY=https://user:pass@proxy.example.com:443
PROXY=socks5://127.0.0.1:1080
PROXY=socks5://user:pass@proxy.example.com:1080
```

---

## 🔌 API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check + key count + model count |
| `/v1/models` | GET | List all 34 available models |
| `/v1/chat/completions` | POST | Forward request to CodeBuddy API |

### Health Check

```bash
curl http://localhost:2069/health
```

### Chat Completions

```bash
curl http://localhost:2069/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer rtr_..." \
  -d '{
    "model": "claude-sonnet-4.6",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello!"}
    ],
    "stream": true
  }'
```

---

## 🧠 Available Models

<details>
<summary><b>Click to expand full model list (34 models)</b></summary>

**Claude**

- `claude-opus-4.7-1m`
- `claude-opus-4.6`
- `claude-sonnet-4.6`
- `claude-haiku-4.5`

**GPT**

- `gpt-5.5`
- `gpt-5.4`
- `gpt-5.3-codex`
- `gpt-5.2-codex`
- `gpt-5.2`
- `gpt-5.1`

**Gemini**

- `gemini-3.5-flash`
- `gemini-3.1-pro`
- `gemini-3.1-flash-lite`
- `gemini-3.0-pro`
- `gemini-3.0-flash`
- `gemini-2.5-pro`
- `gemini-2.5-flash`

**GLM**

- `glm-5.2`
- `glm-5.1`
- `glm-5.0`
- `glm-5v-turbo`
- `glm-5.0-turbo`
- `glm-4.7`
- `glm-4.6`
- `glm-4.6v`

**Kimi**

- `kimi-k2.6`
- `kimi-k2.5`

**DeepSeek**

- `deepseek-v4-pro`
- `deepseek-v4-flash`
- `deepseek-v3.1`

**Others**

- `minimax-m2.5`
- `minimax-m2.7`
- `hunyuan-2.0-instruct`
- `o4-mini`

</details>

---

## 🔧 Project Structure

```
codebuddy-router/
├── main.go                    # Entry point + setup + banner
├── internal/
│   ├── config/
│   │   └── config.go          # .env loader + machine key generator
│   ├── router/
│   │   ├── models.go          # 34 model registry
│   │   ├── proxy.go           # HTTP forwarding + proxy
│   │   ├── rotate.go          # Round-robin key pool
│   │   └── server.go          # HTTP handlers + auth
│   └── tui/
│       └── app.go             # Live TUI dashboard
├── .env                       # Config (never commit)
└── go.mod / go.sum
```

---

## 🖥️ TUI Dashboard

```
CodeBuddy CLI Gateway
───────────────────────────────────────────────────────────────

  Base URL  : http://0.0.0.0:2069/v1
  Models    : 34
  Keys      : 1 (1 active)
  API Key   : rtr_16c9369819187db59273446de1a4eaf84dd3191e14d77d21

  Log
  Total request: 142  ✅: 138  ❌: 4  ⚡: 12/min

  07:54:12   200   3.2s   claude-sonnet-4.6      k1
  07:54:10   429   1.8s   gpt-5.5                k1   rate limit
  07:54:08   200   5.1s   claude-opus-4.7-1m     k1
  07:54:05   200   2.7s   gemini-3.5-flash       k1
  07:54:02   200   4.0s   gpt-5.4                k1
  07:53:58   200   3.5s   claude-sonnet-4.6      k1
  07:53:55   200   2.9s   claude-haiku-4.5       k1
  07:53:50   200   1.8s   gpt-5.1                k1

  [m] minimize  [l] clear  [q] quit
```

**Keybinds:** `q` Quit · `l` Clear Log · `m` Minimize/Expand

---

## 🔗 Client Integration

```bash
# Claude Code
claude code --baseUrl http://localhost:2069/v1 --apiKey $(buddy key)

# OpenCode / Any OpenAI-Compatible tool
export OPENAI_API_BASE=http://localhost:2069/v1
export OPENAI_API_KEY=rtr_...
```

---

## 🐛 Troubleshooting

| Issue | Fix |
|-------|-----|
| Port conflict | Change `PORT` in `.env` |
| `unauthorized: invalid router key` | Restart `buddy` — RouterKey is deterministic per-machine |
| `all keys exhausted` | API keys expired — test with `curl` first |
| `invalid key (401/403)` | Re-enter a valid key — setup wizard will prompt automatically |
| Proxy failed | Verify: `curl -x socks5://... https://www.codebuddy.ai` |
| TUI not showing (headless) | Normal — no TTY, falls back to log mode |
