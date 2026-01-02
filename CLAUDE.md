# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AICarousel is a multi-provider AI service router built with Bun. It routes chat requests to multiple AI providers (Cerebras, Groq, OpenRouter, Gemini) with automatic round-robin rotation and fallback on failure.

## Commands

```bash
bun install          # Install dependencies
bun run dev          # Development with auto-reload (--watch)
bun run start        # Production server

# Interactive Setup CLI (recommended)
bun run setup        # Opens interactive menu for all configuration

# API Key Management (CLI alternative)
bun run api-key create "name"   # Create new API key
bun run api-key list            # List all API keys
bun run api-key revoke <id>     # Revoke an API key
bun run api-key delete <id>     # Delete an API key

# Database
bun run db:migrate              # Run migrations
bun run db:rollback             # Rollback last migration
```

Server runs on port 7123 (or `PORT` env var).

## Architecture

```
index.ts                     # HTTP server, main router
├── cli/                     # Interactive setup CLI
│   ├── index.ts             # Main menu
│   ├── setup.ts             # Initial setup logic
│   ├── providers.ts         # Provider API keys management
│   ├── app_keys.ts          # Application API keys management
│   ├── provider_toggle.ts   # Enable/disable providers
│   ├── status.ts            # System status view
│   └── utils/               # CLI utilities (prompt, display, env)
├── routes/
│   ├── openai.ts            # /v1/chat/completions, /v1/models (Cline, Codex)
│   └── anthropic.ts         # /v1/messages (Claude Code)
├── formatters/
│   ├── openai_formatter.ts  # OpenAI SSE streaming format
│   └── anthropic_formatter.ts # Anthropic SSE streaming format
├── services/
│   ├── chat_handler.ts      # Core chat logic with retry/fallback
│   ├── ai_controller.ts     # Provider management, filtering, ordering
│   ├── gemini_client.ts     # Adapter for Google Generative AI SDK
│   └── openrouter_client.ts # Adapter for OpenRouter SDK
├── auth/
│   └── middleware.ts        # API key authentication middleware
├── db/
│   ├── index.ts             # SQLite connection (bun:sqlite)
│   ├── migrate.ts           # Migration runner
│   ├── api_keys.ts          # API keys repository
│   ├── provider_settings.ts # Provider enable/disable settings
│   └── migrations/          # Database migrations
├── scripts/
│   └── api_key.ts           # CLI for API key management
├── data/
│   └── aicarousel.db        # SQLite database (gitignored)
└── defaults/
    ├── types.ts             # ChatMessage, AIService interfaces
    ├── models.ts            # Model identifiers per provider
    └── providers.ts         # Provider configs (params, apiKeyName)
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

### Request Flow

1. Request arrives at appropriate endpoint
2. Route handler converts to internal `ChatMessage[]` format
3. `handleChat()` selects next provider (round-robin) with automatic retry
4. `StandardAIController.chat()` streams response via async generator
5. Formatter converts stream to appropriate SSE format (OpenAI or Anthropic)
6. Returns `text/event-stream` response

### Key Interfaces

```typescript
interface ChatMessage {
  role: "user" | "assistant" | "system";
  content: string;
}

interface AIService {
  name: string;
  chat(messages: ChatMessage[]): AsyncIterable<string>;
}
```

## Adding a New Provider

1. Create adapter in `services/` implementing the `chat.completions.create()` pattern (see `gemini_client.ts` for non-OpenAI APIs)
2. Add model identifier to `defaults/models.ts`
3. Add provider config to `defaults/providers.ts`
4. Register client in `ClientMap` in `services/ai_controller.ts`
5. Add API key to `.env.template`
6. **Create corresponding tests** in `tests/services/`

## Testing

**IMPORTANT: Every new controller, function, route, or module MUST have corresponding tests.**

```bash
bun test              # Run all tests
bun test:watch        # Watch mode (re-run on file changes)
bun test:coverage     # Run with coverage report
```

### Test Structure

```
tests/
├── utils/
│   └── mocks.ts                    # Mock services, helpers, test utilities
├── services/
│   └── chat_handler.test.ts        # Service logic tests
├── auth/
│   └── middleware.test.ts          # Authentication tests
├── formatters/
│   ├── openai_formatter.test.ts    # OpenAI SSE format tests
│   └── anthropic_formatter.test.ts # Anthropic event format tests
├── routes/
│   ├── openai.test.ts              # OpenAI route tests
│   └── anthropic.test.ts           # Anthropic route tests
└── db/
    ├── api_keys.test.ts            # API keys CRUD tests
    └── provider_settings.test.ts   # Provider settings tests
```

### Writing Tests

Use Bun's built-in test runner:

```typescript
import { describe, test, expect, beforeEach, afterEach } from "bun:test";

describe("MyModule", () => {
  beforeEach(() => {
    // Setup
  });

  test("should do something", () => {
    expect(result).toBe(expected);
  });
});
```

### Test Requirements

When adding new code, create tests that cover:
- **Happy path**: Normal expected behavior
- **Edge cases**: Empty inputs, null values, boundaries
- **Error handling**: Invalid inputs, failures, exceptions
- **Integration**: How components work together

### Using Mocks

Import from `tests/utils/mocks.ts`:

```typescript
import {
  createMockService,
  createFailingService,
  sampleMessages,
  collectStream,
  createMockRequest
} from "../utils/mocks";
```

## Environment Variables

Copy `.env.template` to `.env` and configure:
- `GROQ_API_KEY`, `CEREBRAS_API_KEY`, `OPENROUTER_API_KEY`, `GEMINI_API_KEY`

Bun automatically loads `.env` - no dotenv needed.

## TypeScript Path Aliases

- `@services/*` → `services/*`
- `@defaults/*` → `defaults/*`
- `@routes/*` → `routes/*`
- `@formatters/*` → `formatters/*`
- `@db/*` → `db/*`
- `@auth/*` → `auth/*`

## Interactive Setup CLI

Run `bun run setup` to access the interactive configuration menu:

1. **Setup inicial** - Initialize database and run migrations
2. **Gestionar API Keys de Providers** - Configure provider API keys (reads/writes to .env)
3. **Gestionar API Keys de la Aplicación** - Create, list, revoke application API keys
4. **Seleccionar/Deseleccionar Providers** - Enable/disable providers, change rotation order
5. **Ver estado actual** - View system status (database, providers, API keys)

Provider settings (enabled/disabled, priority order) are stored in SQLite and used by `ai_controller.ts` to filter active services.

## Client Configuration

First, generate an API key:
```bash
bun run api-key create "my-client"
# Save the returned key (sk-xxx...) - it's only shown once!
```

### Cline (VS Code)

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

## Example Requests

```bash
# OpenAI format (Cline, Codex)
curl -X POST http://localhost:7123/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-api-key" \
  -d '{"model": "aicarousel", "messages": [{"role": "user", "content": "Hello"}], "stream": true}'

# Anthropic format (Claude Code)
curl -X POST http://localhost:7123/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-your-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model": "aicarousel", "max_tokens": 1024, "messages": [{"role": "user", "content": "Hello"}], "stream": true}'

# Legacy format
curl -X POST http://localhost:7123/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-api-key" \
  -d '[{"role": "user", "content": "Hello"}]'
```
