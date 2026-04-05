# AI Providers

## Summary

| Provider   | Protocol             | Free tier  | Speed     | Notes                            |
|------------|----------------------|------------|-----------|----------------------------------|
| Cerebras   | OpenAI-compatible    | Generous   | Very fast | Dedicated wafer-scale hardware   |
| Groq       | OpenAI-compatible    | 12k TPM    | Very fast | Strict tokens/min limit          |
| OpenRouter | OpenAI-compatible    | Free models| Variable  | Access to hundreds of models     |
| Gemini     | Native REST/SSE      | 2M TPM     | Fast      | Flash models very efficient      |
| Z.ai       | Anthropic-compatible | —          | —         | GLM models (Zhipu AI)            |
| Ollama     | OpenAI-compatible    | Local      | HW-dependent | No network latency            |

## Cerebras

- **Endpoint**: `https://api.cerebras.ai/v1/chat/completions`
- **Auth**: `Authorization: Bearer <key>`
- **Recommended models**: `qwen-3-32b`, `llama-3.3-70b`
- **Free tier**: Very generous compared to others — ideal as primary provider.
- **Console**: https://cloud.cerebras.ai/

## Groq

- **Endpoint**: `https://api.groq.com/openai/v1/chat/completions`
- **Auth**: `Authorization: Bearer <key>`
- **Recommended models**: `llama-3.3-70b-versatile`, `llama-3.1-8b-instant`
- **Free tier limit**: ~12,000 tokens/minute on-demand.
- **Note**: 429 errors mean TPM rate limit. The system automatically falls back to the next provider.
- **Console**: https://console.groq.com/

## OpenRouter

- **Endpoint**: `https://openrouter.ai/api/v1/chat/completions`
- **Auth**: `Authorization: Bearer <key>`
- **Free models**: `qwen/qwen3-coder:free`, `google/gemma-3-27b-it:free`, and many others.
- **Advantage**: Access to models from multiple labs (Anthropic, Google, Meta, Mistral) with a single key.
- **Console**: https://openrouter.ai/

## Gemini (Google)

- **Endpoint**: `https://generativelanguage.googleapis.com/v1beta/models/{model}:streamGenerateContent`
- **Auth**: API key as query param (`?key=xxx`)
- **Protocol**: Google native SSE (different from OpenAI)
- **Recommended models**: `gemini-1.5-flash`, `gemini-2.0-flash-exp`
- **Free tier**: 2,000,000 tokens/minute with Flash models.
- **Special note**: `system` messages are sent as `systemInstruction`, not as part of the conversation history.
- **Console**: https://aistudio.google.com/app/api-keys

## Z.ai

- **Endpoint**: `https://api.z.ai/api/anthropic/v1/messages` (or `ZAI_BASE_URL`)
- **Auth**: `x-api-key: <key>` + `anthropic-version: 2023-06-01`
- **Protocol**: Anthropic-compatible SSE (events: message_start, content_block_delta, etc.)
- **Models**: `glm-4.7` and variants (Zhipu AI)
- **Note**: The adapter filters only `content_block_delta` events of type `text_delta`.

## Ollama (local)

- **Endpoint**: `http://localhost:11434/v1/chat/completions` (configurable via `OLLAMA_BASE_URL`)
- **Auth**: None (local)
- **Activation**: `OLLAMA_ENABLED=true` in `.env`
- **Technical note**: Ollama uses `max_tokens` instead of `max_completion_tokens`. The adapter handles this automatically.
- **Use case**: Ideal for local development or air-gapped environments.
- **Website**: https://ollama.com/

## Adding a new provider

1. Create `internal/providers/<name>/client.go` implementing `chat.Provider`.
2. Create `internal/providers/<name>/client_test.go` with a full test suite.
3. Register in `internal/providers/registry.go`.
4. Add an entry in `models.json`.
5. Add the env variable to `.env.template`.
6. Register in `providerMeta` in `internal/cli/menu.go` so it appears in the CLI.
