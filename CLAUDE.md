# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AICarousel is a multi-provider AI service router built in Go. It routes chat requests to
multiple AI providers (Cerebras, Groq, OpenRouter, Gemini, Z.ai, Ollama) with automatic
round-robin rotation and fallback on failure.

Compatible with: Cline (OpenAI format), Claude Code (Anthropic format), Codex CLI (OpenAI format).

## Commands

```bash
go build ./...            # Build all binaries into bin/
make build                # Same via Makefile
make test                 # go test ./...
make test-race            # go test -race ./...
make coverage             # Coverage report (target ≥ 85%)
make lint                 # golangci-lint run ./...
make vet                  # go vet ./...

# Run the server
./bin/aicarousel-server

# Interactive setup CLI
./bin/aicarousel-setup

# API key management
./bin/aicarousel-apikey create "name"
./bin/aicarousel-apikey list
./bin/aicarousel-apikey revoke <id>
./bin/aicarousel-apikey delete <id>

# Docker
docker compose up -d
docker compose logs -f
```

Server runs on port 7123 (or `PORT` env var).

## Architecture

```
cmd/
├── server/main.go        # HTTP server entry point
├── setup/main.go         # Interactive CLI entry point
└── apikey/main.go        # API key management CLI
internal/
├── config/               # Config loading (godotenv, typed Cfg)
├── chat/                 # Router: round-robin + fallback + streaming
├── providers/            # Provider adapters + registry
│   ├── cerebras/         # Cerebras AI (OpenAI-compatible)
│   ├── groq/             # Groq (OpenAI-compatible)
│   ├── openrouter/       # OpenRouter (OpenAI-compatible)
│   ├── gemini/           # Google Gemini (native SSE)
│   ├── zai/              # Z.ai (Anthropic-compatible)
│   ├── ollama/           # Ollama local (OpenAI-compatible)
│   ├── sseparse/         # SSE event parser shared by adapters
│   └── provparams/       # Provider params leaf package
├── formatters/           # SSE output formatters
│   ├── openai/           # OpenAI streaming + non-streaming format
│   └── anthropic/        # Anthropic 6-event sequence format
├── httpapi/              # HTTP handlers + middleware
│   ├── openai/           # /v1/chat/completions, /v1/models
│   ├── anthropic/        # /v1/messages, /v1/messages/count_tokens
│   ├── legacy/           # /chat (plain-text stream)
│   └── health/           # /health
├── auth/                 # API key authentication middleware
├── db/                   # SQLite (modernc.org/sqlite, pure Go)
│   ├── apikeys/          # API keys CRUD
│   ├── provsettings/     # Provider enable/disable + priority
│   └── migrations/       # Embedded SQL migrations (checksummed)
├── modelsconfig/         # models.json CRUD + RWMutex cache
├── cli/                  # Interactive setup CLI menus
└── util/                 # id (chatcmpl-*, msg-*), secure (sk-*, SHA-256)
testutil/                 # Test helpers: DB, mocks, transport tracker
models.json               # Provider model config (default, fallback, list)
```

## API Endpoints

| Endpoint | Method | Auth | Compatible With |
|---|---|---|---|
| `/v1/chat/completions` | POST | Bearer | Cline, Codex, LiteLLM |
| `/v1/models` | GET | public | Cline, Codex |
| `/v1/messages` | POST | x-api-key | Claude Code |
| `/v1/messages/count_tokens` | POST | x-api-key | Claude Code |
| `/chat` | POST | Bearer | Legacy direct |
| `/health` | GET | public | Health checks |

**Authentication**: `Authorization: Bearer sk-xxx` or `x-api-key: sk-xxx`

### Request flow

1. Request arrives → middleware chain: CORS → recover → auth.Gate → mux
2. Handler decodes request, maps to `[]chat.ChatMessage` plus a `chat.Options`
   carrying the caller's sampling params (`temperature`, `top_p`, `max_tokens`, `stop`)
3. `chat.Router.Handle()` picks next provider (round-robin), probes with first-chunk timeout
   (the probe covers the dial too — see below)
4. Before each attempt, `chat.AdaptMessagesForModel()` reshapes the messages for
   the chosen model's chat template (see below)
5. On failure: intra-provider model fallback (if enabled), then cross-provider fallback
6. Formatter converts stream to SSE (OpenAI or Anthropic format)

### System-role adaptation

`internal/chat/systemrole.go` handles models whose chat template has no dedicated
`system` role — Gemma renders system messages exactly like user messages, so the
system prompt reaches the model as if the user had typed it, and small models end
up quoting or commenting on it. For models matching `MODELS_WITHOUT_SYSTEM_ROLE`
(default `gemma`), the router merges the system messages into the first user turn
behind explicit delimiters. It runs in `tryProvider`, not `Handle`, because the
target model is only known once a provider has been picked and fallback may land
on a different one.

### Probe budget and cancelled requests

The first-chunk probe in `chat.tryProvider` covers `p.Chat()` as well as the wait
for the first chunk. `p.Chat()` blocks until the upstream returns response
headers, and a local Ollama loading a cold model can sit there for a minute —
outside the probe, that made the router wait indefinitely while the caller timed
out. Per-provider budgets come from `FIRST_CHUNK_TIMEOUT_MS_<PROVIDER>` and are
wired in through `chat.WithProviderTimeouts`.

Once the caller's context is cancelled, `Handle` stops the carousel instead of
dialling every remaining provider with a dead context — those attempts all fail
instantly with `context canceled` and read like a fleet-wide outage in the log.

### Sampling parameters

`chat.Options` uses pointer fields so "unset" is distinguishable from a zero
value. `applyOptions` in `cmd/server/main.go` overlays them on
`provparams.DefaultParams`; anything the caller omits keeps the provider default.

### Ollama native endpoint

`internal/providers/ollama` uses Ollama's native `/api/chat`, not the
OpenAI-compatible `/v1/chat/completions`: only the native one accepts an
`options` object, which is where `num_ctx` lives. Without it the context window
stays at Ollama's 4096-token default, which truncates long system prompts from
the front. The stream is NDJSON (one JSON object per line), and reasoning models'
`thinking` field is deliberately dropped.

`OLLAMA_KEEP_ALIVE` keeps the model resident between requests, which is what
makes the prompt-prefix cache pay off: the system prompt is identical across
conversations, so a warm model only evaluates the new turns. `keepAliveValue`
picks the JSON type per Ollama's parser — a number is seconds (negative =
forever), a string goes through `time.ParseDuration`, which rejects `"-1"`.

## Models Configuration

`models.json` at repo root controls each provider's models and fallback behavior:

```json
{
  "cerebras": {
    "default": "qwen-3-32b",
    "enableFallback": true,
    "models": ["qwen-3-32b", "llama-3.3-70b"]
  }
}
```

Override entirely with `MODELS_CONFIG` env var (JSON string).

## Environment Variables

Copy `.env.template` to `.env`:

| Variable | Description |
|---|---|
| `CEREBRAS_API_KEY` | Cerebras API key |
| `GROQ_API_KEY` | Groq API key |
| `OPENROUTER_API_KEY` | OpenRouter API key |
| `GEMINI_API_KEY` | Google Gemini API key |
| `ZAI_API_KEY` | Z.ai API key |
| `ZAI_BASE_URL` | Z.ai base URL (default: `https://api.z.ai/api/anthropic`) |
| `OLLAMA_ENABLED` | `true` to enable local Ollama |
| `OLLAMA_BASE_URL` | Ollama URL (default: `http://localhost:11434`) |
| `OLLAMA_NUM_CTX` | Ollama context window in tokens (default: `8192`) |
| `OLLAMA_KEEP_ALIVE` | How long Ollama keeps the model in RAM: duration (`30m`), seconds (`3600`), `0` (unload now), `-1` (until the service stops). Default: 5 min |
| `MODELS_WITHOUT_SYSTEM_ROLE` | Models whose template has no `system` role (default: `gemma`) |
| `PORT` | Listen port (default: `7123`) |
| `DB_PATH` | SQLite path (default: `data/aicarousel.db`) |
| `MODELS_CONFIG` | JSON override for `models.json` |
| `FIRST_CHUNK_TIMEOUT_MS` | Provider probe timeout ms, dial included (default: `3000`) |
| `FIRST_CHUNK_TIMEOUT_MS_<PROVIDER>` | Per-provider probe timeout override (e.g. `FIRST_CHUNK_TIMEOUT_MS_OLLAMA=30000`) |

## Testing

```bash
make test           # go test ./...
make test-race      # go test -race ./...  <- mandatory for CI
make coverage       # coverage report
make lint           # golangci-lint (0 issues required)
```

**IMPORTANT: Every new package, handler, or provider adapter MUST have tests.**

Coverage target: >= 85% overall. Race detector must be green.

Test structure:
```
internal/<pkg>/<file>_test.go   # package-level tests (same package)
testutil/                        # shared: NewTestDB, OkProvider, TrackTransport
```

### Key test patterns

- Provider tests: success, request headers, non-200 error, FD-leak (N=100 cancelled probes)
- Handler tests: httptest.NewServer + SSE responses via httptest.ResponseRecorder
- DB tests: testutil.NewTestDB() for real SQLite (no mocks)
- Race tests: go test -race required; router_race_test.go runs N concurrent Handle() calls

## Adding a New Provider

1. Create `internal/providers/<name>/client.go` implementing `chat.Provider`
   - `New(apiKey string, params provparams.Params) chat.Provider` (production constructor)
   - `newClient(url, apiKey string, params provparams.Params, h *http.Client) *client` (testable)
   - `Name() string`, `Key() string`, `Model() string`
   - `Chat(ctx, msgs) (<-chan chat.StreamChunk, error)` with `defer resp.Body.Close() //nolint:errcheck`
2. Create `internal/providers/<name>/client_test.go` with full test suite
3. Register in `internal/providers/registry.go`
4. Add to `models.json`
5. Add API key to `.env.template`

## Client Configuration

**Cline (VS Code)**
```
API Provider: OpenAI Compatible
Base URL: http://localhost:7123/v1
API Key: sk-your-key
Model ID: aicarousel
```

**Claude Code**
```bash
export ANTHROPIC_BASE_URL=http://localhost:7123
export ANTHROPIC_API_KEY=sk-your-key
```

**Codex CLI**
```bash
export OPENAI_API_BASE=http://localhost:7123/v1
export OPENAI_API_KEY=sk-your-key
```

## Deployment

```bash
# Docker Compose (recommended)
docker compose up -d
```

Persistent volume `aicarousel-data` -> `/app/data`. External Docker network `shared-net` allows
other containers to reach the service as `aicarousel:7123`.

Rollback: the `pre-go-cutover-backup` branch preserves the original TypeScript/Bun sources.
