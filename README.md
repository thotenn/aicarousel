# AICarousel — Go

Go rewrite of [AICarousel](../), a multi-provider AI service router. Routes chat requests to
multiple AI providers (Cerebras, Groq, OpenRouter, Gemini, Z.ai, Ollama) with round-robin
rotation and automatic fallback on failure.

## Quick start

```bash
# 1. Copy and fill in your API keys
cp .env.template .env

# 2. Build all binaries
make build

# 3. Run interactive setup (initialises DB, configures providers)
./bin/aicarousel-setup

# 4. Start the server
./bin/aicarousel-server
```

Server listens on **port 7123** (override with `PORT` env var).

## Installation

**Requirements**: Go 1.23+

```bash
git clone <repo>
cd ai-carousel-go
make build          # produces bin/aicarousel-server, bin/aicarousel-setup, bin/aicarousel-apikey
```

## Running

```bash
# Development (with go run)
go run ./cmd/server

# Production binary
./bin/aicarousel-server

# Run database migrations manually
./bin/aicarousel-server migrate
```

## Docker

```bash
# Build and start (persists DB in named volume aicarousel-data)
docker compose up -d

# View logs
docker compose logs -f

# Stop (DB is preserved)
docker compose down
```

The compose file exposes port `${PORT:-7123}` and joins the external Docker network `shared-net`
(create it once with `docker network create shared-net`). Other containers can reach the service
as `aicarousel:7123` on that network.

## Configuration

Copy `.env.template` to `.env` and set:

| Variable | Description |
|---|---|
| `CEREBRAS_API_KEY` | Cerebras AI API key |
| `GROQ_API_KEY` | Groq API key |
| `OPENROUTER_API_KEY` | OpenRouter API key |
| `GEMINI_API_KEY` | Google Gemini API key |
| `ZAI_API_KEY` | Z.ai API key |
| `ZAI_BASE_URL` | Z.ai base URL (default: `https://api.z.ai/api/anthropic`) |
| `OLLAMA_ENABLED` | Set to `true` to enable local Ollama |
| `OLLAMA_BASE_URL` | Ollama base URL (default: `http://localhost:11434`) |
| `OLLAMA_NUM_CTX` | Ollama context window in tokens (default: `8192`) |
| `MODELS_WITHOUT_SYSTEM_ROLE` | Models whose template has no `system` role (default: `gemma`) |
| `PORT` | HTTP listen port (default: `7123`) |
| `DB_PATH` | SQLite DB path (default: `data/aicarousel.db`) |
| `MODELS_CONFIG` | JSON string that overrides `models.json` entirely |

### Models without a `system` role

Not every chat template has a dedicated `system` role. Gemma is the canonical
example: its template renders system messages exactly as user messages, so a
system prompt reaches the model as if the end user had typed it — small models
then quote or comment on those instructions instead of following them.

For any model matching `MODELS_WITHOUT_SYSTEM_ROLE` (default: `gemma`, matched
as a case-insensitive substring of the model name), the router merges every
system message into the first user turn, wrapped in explicit delimiters that
mark it as internal configuration. Set the variable to an empty value to disable
the adaptation, or to your own comma-separated list to extend it.

### Ollama context window

The Ollama adapter talks to the native `/api/chat` endpoint instead of the
OpenAI-compatible one, because the compatibility layer accepts no `options`
object and therefore cannot set `num_ctx`. Ollama's default context window is
4096 tokens (2048 on older installs) and it truncates from the front, dropping
the system prompt first. `OLLAMA_NUM_CTX` (default: 8192) sets it explicitly.

### Sampling parameters

`temperature`, `top_p`, `max_tokens` and `stop` from the incoming request are
forwarded to the provider and take precedence over the defaults in
`internal/providers/provparams`. Parameters the caller omits keep the provider
default.

### Models configuration

Edit `models.json` to control which models each provider uses and their fallback order:

```json
{
  "cerebras": {
    "default": "qwen-3-32b",
    "enableFallback": true,
    "models": ["qwen-3-32b", "llama-3.3-70b"]
  }
}
```

## CLI commands

### Interactive setup

```bash
./bin/aicarousel-setup
```

Menu options:
1. **Initial setup** — init DB, run migrations, sync providers
2. **Provider API keys** — set/view keys (writes to `.env`)
3. **Application API keys** — create, list, revoke app keys
4. **Enable/disable providers** — toggle and reorder round-robin priority
5. **Manage provider models** — add/edit/delete models, set default, toggle fallback
6. **System status** — DB health, active providers, key count

### API key management

```bash
./bin/aicarousel-apikey create "my-app"   # prints sk-... (shown once)
./bin/aicarousel-apikey list
./bin/aicarousel-apikey revoke <id>
./bin/aicarousel-apikey delete <id>
```

Exit codes: `0` = ok, `1` = usage error, `2` = runtime error.

## API endpoints

| Endpoint | Method | Auth | Compatible with |
|---|---|---|---|
| `/v1/chat/completions` | POST | Bearer | Cline, Codex, LiteLLM |
| `/v1/models` | GET | — | Cline, Codex |
| `/v1/messages` | POST | x-api-key | Claude Code |
| `/v1/messages/count_tokens` | POST | x-api-key | Claude Code |
| `/chat` | POST | Bearer | Legacy direct use |
| `/health` | GET | — | Health checks |

### Client setup

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

## Development

```bash
make test           # go test ./...
make test-race      # go test -race ./...
make coverage       # go test -coverprofile=coverage.out + go tool cover -func
make lint           # golangci-lint run ./...
make vet            # go vet ./...
```

Current coverage: **85%** overall. Race detector: green.
