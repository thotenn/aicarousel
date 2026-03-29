# AICarousel

Multi-provider AI service router with automatic round-robin rotation and fallback. Compatible with Cline, Claude Code, Codex, and any OpenAI/Anthropic-compatible client.

## Features

- **Multi-Provider Support**: Cerebras, Groq, OpenRouter, Gemini, Z.ai, Ollama
- **Automatic Failover**: Round-robin rotation with automatic retry on failure
- **Intra-Provider Fallback**: Try multiple models within a provider before switching
- **Configurable Models**: JSON-based model configuration with per-provider settings
- **API Compatibility**: OpenAI and Anthropic API formats supported
- **Authentication**: SQLite-based API key management
- **Interactive CLI**: Unified setup and configuration interface
- **Streaming**: SSE streaming for all endpoints

## Quick Start

### Local Development

```bash
bun install
bun run setup
bun run dev
```

### Docker Compose

```bash
cp .env.template .env
# Edit .env with your API keys
docker compose up -d
```

## Installation

### Option A: Local (Bun)

```bash
bun install
bun run setup    # Interactive CLI for all configuration
bun run dev      # Development with auto-reload
bun run start    # Production
```

### Option B: Docker Compose

```bash
cp .env.template .env
# Configure your API keys in .env
docker compose up -d
```

The Docker setup includes a persistent volume for the SQLite database.

Server runs on `http://localhost:7123` (configurable via `PORT` env var).

### Interactive Setup CLI

```bash
bun run setup
```

1. **Setup** - Initialize database and run migrations
2. **Providers API Keys** - Configure provider API keys
3. **Applications API Keys** - Create/manage application API keys for authentication
4. **Active Providers** - Enable/disable providers and set rotation order
5. **Models Management** - Add/edit/delete models, set defaults, toggle fallback, reorder
6. **System State** - View current configuration status

## Client Configuration

First, create an API key via the CLI (`bun run setup` > option 3) or command line:

```bash
bun run api-key create "my-client"
# Save the returned key (sk-xxx...) - shown only once!
```

### Cline (VS Code Extension)

```
API Provider: OpenAI Compatible
Base URL: http://localhost:7123/v1
API Key: sk-your-api-key
Model ID: aicarousel
```

### Claude Code

```bash
export ANTHROPIC_BASE_URL=http://localhost:7123
export ANTHROPIC_API_KEY=sk-your-api-key
```

### Codex CLI

```bash
export OPENAI_API_BASE=http://localhost:7123/v1
export OPENAI_API_KEY=sk-your-api-key
```

### LiteLLM / Other OpenAI-Compatible

```bash
export OPENAI_API_BASE=http://localhost:7123/v1
export OPENAI_API_KEY=sk-your-api-key
```

## API Endpoints

| Endpoint                    | Method | Auth     | Format    | Compatible With       |
| --------------------------- | ------ | -------- | --------- | --------------------- |
| `/v1/chat/completions`      | POST   | Required | OpenAI    | Cline, Codex, LiteLLM |
| `/v1/models`                | GET    | Public   | OpenAI    | Cline, Codex          |
| `/v1/messages`              | POST   | Required | Anthropic | Claude Code           |
| `/v1/messages/count_tokens` | POST   | Required | Anthropic | Claude Code           |
| `/chat`                     | POST   | Required | Legacy    | Direct use            |
| `/health`                   | GET    | Public   | JSON      | Health checks         |

**Authentication**: Include API key as `Authorization: Bearer sk-xxx` or `x-api-key: sk-xxx`

## Example Requests

### OpenAI Format (Cline, Codex)

```bash
curl -X POST http://localhost:7123/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-api-key" \
  -d '{
    "model": "aicarousel",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

### Anthropic Format (Claude Code)

```bash
curl -X POST http://localhost:7123/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-your-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "aicarousel",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

## Commands

```bash
# Interactive setup (recommended)
bun run setup

# Server
bun run dev              # Development with auto-reload
bun run start            # Production

# API Key Management (CLI alternative)
bun run api-key create "name"   # Create new API key
bun run api-key list            # List all API keys
bun run api-key revoke <id>     # Revoke an API key
bun run api-key delete <id>     # Delete an API key

# Database
bun run db:migrate       # Run migrations
bun run db:rollback      # Rollback last migration
```

## Models Configuration

Models are configured in `models.json` at the project root:

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

### Fallback Behavior

- **`enableFallback: true`**: When a model fails, tries other models in the same provider before moving to next provider
- **`enableFallback: false`**: Only tries the default model, then moves to next provider
- **Model order**: First model in the array is tried first (after default), determines fallback priority

Manage models via CLI: `bun run setup` → option 5

## Environment Variables

Create a `.env` file (copy from `.env.template`):

```env
# Server
PORT=7123

# Provider API Keys (configure at least one)
CEREBRAS_API_KEY=your-key
GROQ_API_KEY=your-key
OPENROUTER_API_KEY=your-key
GEMINI_API_KEY=your-key
ZAI_API_KEY=your-key
# ZAI_BASE_URL=https://api.z.ai/api/anthropic

# Ollama (local LLM)
OLLAMA_ENABLED=true
OLLAMA_BASE_URL=http://localhost:11434

# Database path (default: ./data/aicarousel.db)
# DB_PATH=/data/aicarousel.db

# Models configuration (optional, overrides models.json)
# MODELS_CONFIG={"cerebras":{"default":"qwen-3-32b","enableFallback":true,"models":["qwen-3-32b"]}}
```

### MODELS_CONFIG

You can override `models.json` entirely via the `MODELS_CONFIG` environment variable. This is useful for deployments where you can only set env vars (e.g., Coolify, Railway). If set, the env var takes priority over the file.

## License

MIT
