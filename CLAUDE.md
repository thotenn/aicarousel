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
│   ├── models.ts            # Provider models management (CRUD, fallback, reorder)
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
│   ├── models_config.ts     # Models JSON config CRUD operations
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
├── defaults/
│   ├── types.ts             # ChatMessage, AIService, AIServiceWithModel interfaces
│   ├── models.ts            # Legacy model exports (use models.json instead)
│   └── providers.ts         # Provider configs (params, apiKeyName)
└── models.json              # Model configuration per provider (default, fallback, models list)
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

interface AIServiceWithModel extends AIService {
  providerKey: string;
  model: string;
}

interface ActiveProvider {
  key: string;
  name: string;
  models: string[];
  defaultModel: string;
  enableFallback: boolean;
  priority: number;
}
```

## Models Configuration

Models are configured in `models.json` at the project root. Each provider has:
- **default**: The primary model to use
- **enableFallback**: If `true`, tries other models when default fails before moving to next provider
- **models**: Array of available models (order determines fallback priority)

### models.json Format

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

1. **Intra-provider fallback** (`enableFallback: true`): When a model fails, tries other models in the same provider before moving to the next provider
2. **Cross-provider fallback**: After exhausting all models in a provider (or if `enableFallback: false`), moves to the next provider in round-robin order
3. **All fail**: Throws error only after all providers and their models have been exhausted

### Managing Models via CLI

Run `bun run setup` and select option 5 "Gestionar Modelos de Providers":

- **View models**: See all models for a provider with default marked
- **Add model**: Add a new model to a provider
- **Edit model**: Rename an existing model
- **Delete model**: Remove a model (cannot delete the default)
- **Set default**: Change the default model
- **Toggle fallback**: Enable/disable intra-provider fallback
- **Reorder models**: Change fallback priority order

### Programmatic Access

```typescript
import {
  getModelsConfig,
  saveModelsConfig,
  getProviderModels,
  getDefaultModel,
  isProviderFallbackEnabled,
  addModel,
  removeModel,
  setDefaultModel,
  toggleFallback,
  reorderModels
} from "./services/models_config";
```

## Adding a New Provider

1. Create adapter in `services/` implementing the `chat.completions.create()` pattern (see `gemini_client.ts` for non-OpenAI APIs)
2. Add provider entry to `models.json` with default model, enableFallback, and models array
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
│   ├── chat_handler.test.ts        # Chat logic, fallback behavior tests
│   ├── ai_controller.test.ts       # Provider filtering, service creation tests
│   └── models_config.test.ts       # Models JSON validation, CRUD tests
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
  // Basic service mocks
  createMockService,
  createFailingService,
  createEmptyService,
  sampleMessages,
  collectStream,
  createMockRequest,
  // Model-aware service mocks
  createMockServiceWithModel,
  createFailingServiceWithModel,
  createActiveProvider,
  createProviderModelConfig,
  sampleModelsConfig,
  sampleActiveProviders
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
5. **Gestionar Modelos de Providers** - Add/edit/delete models, set defaults, toggle fallback, reorder
6. **Ver estado actual** - View system status (database, providers, API keys)

Provider settings (enabled/disabled, priority order) are stored in SQLite and used by `ai_controller.ts` to filter active services.
Model settings are stored in `models.json` and used by `chat_handler.ts` for fallback logic.

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
