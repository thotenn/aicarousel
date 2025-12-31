# AICarousel

Multi-provider AI service router with automatic round-robin rotation and fallback. Compatible with Cline, Claude Code, Codex, and any OpenAI/Anthropic-compatible client.

## Features

- **Multi-Provider Support**: Cerebras, Groq, OpenRouter, Gemini
- **Automatic Failover**: Round-robin rotation with automatic retry on failure
- **API Compatibility**: OpenAI and Anthropic API formats supported
- **Authentication**: SQLite-based API key management
- **Interactive CLI**: Unified setup and configuration interface
- **Streaming**: SSE streaming for all endpoints

## Quick Start

```bash
# Install dependencies
bun install

# Run interactive setup
bun run setup

# Start server
bun run dev
```

## Installation

### 1. Install Dependencies

```bash
bun install
```

### 2. Configure with Interactive CLI

```bash
bun run setup
```

The CLI provides:
1. **Setup inicial** - Initialize database and run migrations
2. **API Keys de Providers** - Configure provider API keys (Cerebras, Groq, OpenRouter, Gemini)
3. **API Keys de la Aplicacion** - Create/manage application API keys for authentication
4. **Providers Activos** - Enable/disable providers and set rotation order
5. **Estado del Sistema** - View current configuration status

### 3. Start the Server

```bash
# Development (auto-reload)
bun run dev

# Production
bun run start
```

Server runs on `http://localhost:7123` (configurable via `PORT` env var).

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

| Endpoint | Method | Auth | Format | Compatible With |
|----------|--------|------|--------|-----------------|
| `/v1/chat/completions` | POST | Required | OpenAI | Cline, Codex, LiteLLM |
| `/v1/models` | GET | Public | OpenAI | Cline, Codex |
| `/v1/messages` | POST | Required | Anthropic | Claude Code |
| `/v1/messages/count_tokens` | POST | Required | Anthropic | Claude Code |
| `/chat` | POST | Required | Legacy | Direct use |
| `/health` | GET | Public | JSON | Health checks |

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

## Environment Variables

Create a `.env` file (or use `bun run setup` to configure):

```env
# Server
PORT=7123

# Provider API Keys (configure at least one)
CEREBRAS_API_KEY=your-key
GROQ_API_KEY=your-key
OPENROUTER_API_KEY=your-key
GEMINI_API_KEY=your-key
```

## Architecture

```
aicarousel/
├── index.ts                 # HTTP server, main router
├── cli/                     # Interactive setup CLI
│   ├── index.ts             # Main menu
│   ├── setup.ts             # Initial setup
│   ├── providers.ts         # Provider API keys
│   ├── app_keys.ts          # Application API keys
│   ├── provider_toggle.ts   # Enable/disable providers
│   ├── status.ts            # System status
│   └── utils/               # CLI utilities
├── routes/
│   ├── openai.ts            # /v1/chat/completions, /v1/models
│   └── anthropic.ts         # /v1/messages
├── formatters/
│   ├── openai_formatter.ts  # OpenAI SSE format
│   └── anthropic_formatter.ts
├── services/
│   ├── chat_handler.ts      # Core chat with retry/fallback
│   ├── ai_controller.ts     # Provider management
│   ├── gemini_client.ts     # Gemini adapter
│   └── openrouter_client.ts # OpenRouter adapter
├── auth/
│   └── middleware.ts        # API key authentication
├── db/
│   ├── index.ts             # SQLite connection
│   ├── migrate.ts           # Migration runner
│   ├── api_keys.ts          # API keys repository
│   └── provider_settings.ts # Provider settings
├── defaults/
│   ├── types.ts             # TypeScript interfaces
│   ├── models.ts            # Model identifiers
│   └── providers.ts         # Provider configs
└── data/
    └── aicarousel.db        # SQLite database
```

## Adding a New Provider

1. Create adapter in `services/` implementing `chat.completions.create()` pattern
2. Add model identifier to `defaults/models.ts`
3. Add provider config to `defaults/providers.ts` (include `apiKeyName`)
4. Register client in `ClientMap` in `services/ai_controller.ts`
5. Add API key placeholder to `.env.template`

## How It Works

1. Request arrives at endpoint (OpenAI or Anthropic format)
2. Route handler converts to internal `ChatMessage[]` format
3. `handleChat()` selects next provider via round-robin
4. If provider fails, automatically retries with next provider
5. Response streams back in appropriate SSE format

Provider selection respects:
- Only providers with configured API keys
- Only providers enabled in settings
- Priority order set via CLI

## License

MIT
