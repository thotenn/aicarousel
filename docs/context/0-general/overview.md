# AICarousel — Overview

## What it is

AICarousel is a multi-provider AI service router written in Go. It acts as an intelligent proxy
that receives requests in OpenAI or Anthropic format and distributes them across multiple AI
providers automatically, with round-robin rotation and automatic fallback on failure.

It lets you use a single URL and a single API key in your clients (Cline, Claude Code, Codex,
etc.) while the system internally manages which provider handles each request.

## Problem it solves

- **Rate limits**: If Groq hits its tokens-per-minute limit, the next request automatically
  goes to Cerebras or Gemini.
- **Availability**: If a provider is down, the system detects it and moves to the next one
  without the client noticing.
- **Cost**: Distributes load across free or low-cost providers to maximize usage without paying.
- **Single access point**: Clients like Claude Code or Cline point to one URL regardless of
  which provider responds.

## Supported providers

| Provider   | API type              | Env variable            |
|------------|-----------------------|-------------------------|
| Cerebras   | OpenAI-compatible     | `CEREBRAS_API_KEY`      |
| Groq       | OpenAI-compatible     | `GROQ_API_KEY`          |
| OpenRouter | OpenAI-compatible     | `OPENROUTER_API_KEY`    |
| Gemini     | Native SSE (Google)   | `GEMINI_API_KEY`        |
| Z.ai       | Anthropic-compatible  | `ZAI_API_KEY`           |
| Ollama     | OpenAI-compatible     | `OLLAMA_ENABLED=true`   |

## Exposed endpoints

| Endpoint                        | Method | Auth      | Compatible with             |
|---------------------------------|--------|-----------|-----------------------------|
| `/v1/chat/completions`          | POST   | Bearer    | Cline, Codex, LiteLLM       |
| `/v1/models`                    | GET    | Public    | Cline, Codex                |
| `/v1/models/{id}`               | GET    | Public    | Cline, Codex                |
| `/v1/messages`                  | POST   | x-api-key | Claude Code                 |
| `/v1/messages/count_tokens`     | POST   | x-api-key | Claude Code                 |
| `/chat`                         | POST   | Bearer    | Direct use (legacy)         |
| `/health`                       | GET    | Public    | Health checks               |

**Authentication**: `Authorization: Bearer sk-xxx` or `x-api-key: sk-xxx` header.
Public routes (`/health`, `/v1/models`) require no key.

## Layered architecture

```
Client (Cline / Claude Code / Codex)
        |
        v
HTTP Server :7123
  └── Middleware chain
        ├── CORS
        ├── Recover (panic → 500)
        └── auth.Gate (API Key validation)
              |
              v
        HTTP Handlers
        ├── /v1/chat/completions  →  OpenAI handler
        ├── /v1/messages          →  Anthropic handler
        ├── /chat                 →  Legacy handler
        └── /health               →  Health handler
              |
              v
        chat.Router (round-robin + fallback)
              |
        ┌─────┴──────┬────────┬──────────┐
        v            v        v          v
     Cerebras      Groq   Gemini      Z.ai / Ollama
     adapter      adapter  adapter    adapters
        |
        v
  SSE stream → Formatter (OpenAI or Anthropic) → Client
```

## Routing logic

### Round-robin

The router maintains a shared index (mutex-protected) that advances with each successful
request. With 3 active providers [Cerebras, Groq, Gemini]:

- Request 1 → Cerebras
- Request 2 → Groq
- Request 3 → Gemini
- Request 4 → Cerebras (wraps around)

### Intra-provider fallback

If a provider has `enableFallback: true` and multiple models, when the default model fails,
it tries the remaining models in that provider before moving to the next provider.

```
Groq / llama-3.3-70b-versatile → fails (rate limit)
Groq / llama-3.1-8b-instant    → fails (also saturated)
→ moves to Cerebras
```

### Cross-provider fallback

When a provider fails completely (or runs out of models), the router moves to the next one
in rotation order.

### First-chunk probe

To detect failures quickly, the router uses a timeout (`FIRST_CHUNK_TIMEOUT_MS`, default 3000ms)
that cancels the request if the provider does not emit its first response chunk within that time.
This prevents a slow provider from blocking the system indefinitely.

The budget covers the dial as well: `Provider.Chat()` blocks until the upstream returns response
headers, and a local backend loading a cold model can stay there far longer than the probe window.

Providers with legitimately slow starts get their own budget via
`FIRST_CHUNK_TIMEOUT_MS_<PROVIDER>` (e.g. `FIRST_CHUNK_TIMEOUT_MS_OLLAMA=30000`). Keep every
budget below the calling client's own timeout: if the caller gives up first, the request context
dies and no provider can answer it.

### Circuit breaker

A provider/model that fails `PROVIDER_BREAKER_FAILURES` times in a row (default 3) is skipped
for `PROVIDER_BREAKER_COOLDOWN_MS` (default 5 min). Losing the probe costs real time: without
this, every request that lands on a dead provider's slot in the rotation waits out its whole
probe budget first. If a pass finds every candidate cooling down, the router retries them
ignoring the breaker — a slow answer beats no answer.

### Cancelled requests

If the caller disconnects mid-probe, the router aborts the carousel instead of trying the
remaining providers — a dead context makes every one of them fail instantly, which used to fill
the log with one "provider failed" line per provider and hide the single real cause.

## Request lifecycle

```
1. Request arrives at the handler
2. Handler converts to internal []chat.ChatMessage format
3. Router selects provider (round-robin)
4. Router probes the provider with timeout
5. If provider responds: chunk stream → formatter → SSE to client
6. If fails: tries next model or provider (see fallback above)
7. If the caller is gone: aborts immediately, no further providers are tried
   (a cancellation never counts against a provider's breaker)
8. If all fail: returns 503 "All AI services failed"
```

## Models configuration

The `models.json` file at the repo root defines available models per provider:

```json
{
  "cerebras": {
    "default": "qwen-3-32b",
    "enableFallback": true,
    "models": ["qwen-3-32b", "llama-3.3-70b"]
  },
  "groq": {
    "default": "llama-3.3-70b-versatile",
    "enableFallback": true,
    "models": ["llama-3.3-70b-versatile", "llama-3.1-8b-instant"]
  }
}
```

Can be fully overridden with the `MODELS_CONFIG` env var (JSON string), useful in deployments
where only env vars can be configured (Coolify, Railway, Fly.io, etc.).

## Database

Pure-Go SQLite (`modernc.org/sqlite`, no CGO). Two main tables:

- **`api_keys`**: Application keys with SHA-256 hash, visible prefix, active/revoked state,
  and usage counter.
- **`provider_settings`**: Enabled/disabled state and rotation priority of each provider.

Migrations are embedded in the binary with SHA-256 checksums for integrity. They run
automatically on every server start.

## Security

- API Keys are stored only as SHA-256 hashes — never in plain text.
- Plain text is shown only at creation time, never again.
- Every route except `/health` and `/v1/models` requires a valid, active key.
- Revoked keys return 401 immediately.
