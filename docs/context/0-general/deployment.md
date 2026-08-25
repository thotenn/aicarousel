# Deployment and Configuration

## Environment variables

Copy `.env.template` to `.env` and fill in:

```env
PORT=7123                         # HTTP listen port (default: 7123)

# Provider API keys
GROQ_API_KEY=
CEREBRAS_API_KEY=
OPENROUTER_API_KEY=
GEMINI_API_KEY=
ZAI_API_KEY=
ZAI_BASE_URL=                     # Optional. Default: https://api.z.ai/api/anthropic

# Ollama (local models)
OLLAMA_ENABLED=true               # Set to true to enable
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_NUM_CTX=8192               # Optional. Context window in tokens
OLLAMA_KEEP_ALIVE=30m             # Optional. Keeps the model in RAM between requests

# Database
DB_PATH=data/aicarousel.db        # Optional. Default: data/aicarousel.db

# Models config override (replaces models.json entirely)
MODELS_CONFIG=                    # JSON string, optional

# Timeouts and behavior
FIRST_CHUNK_TIMEOUT_MS=3000       # Provider probe timeout in ms (dial included)
FIRST_CHUNK_TIMEOUT_MS_OLLAMA=30000  # Optional. Per-provider override; a local
                                     # model may need to load before answering

# Logging
LOG_LEVEL=info                    # debug | info | warn | error
LOG_FORMAT=text                   # text | json
```

## Docker Compose (recommended for production)

```yaml
services:
  aicarousel:
    build: .
    ports:
      - "${PORT:-7123}:7123"
    volumes:
      - aicarousel-data:/app/data
    env_file:
      - .env
    environment:
      - DB_PATH=/app/data/aicarousel.db
    networks:
      - shared-net
    restart: unless-stopped

volumes:
  aicarousel-data:

networks:
  shared-net:
    external: true
```

```bash
# Create the external network once
docker network create shared-net

# Start
docker compose up -d --build

# Logs
docker compose logs -f

# Stop (DB persists in the volume)
docker compose down
```

The `aicarousel-data` volume persists the database across restarts and re-deployments.
Other containers on `shared-net` can reach the service at `aicarousel:7123`.

## Coolify

1. Point to the `go` branch of the repository.
2. Coolify detects `docker-compose.yaml` automatically.
3. Add provider API keys in **Environment Variables**.
4. Optionally add `APP_VERSION=go-v0.1.0` in **Build Variables** for version in logs.
5. Create `shared-net` on the server: `docker network create shared-net`.
6. Deploy.

## Manual build

```bash
# Build all binaries into bin/
make build

# Or individually
go build -o bin/aicarousel-server ./cmd/server
go build -o bin/aicarousel-setup  ./cmd/setup
go build -o bin/aicarousel-apikey ./cmd/apikey

# With version injected
go build -ldflags="-X main.version=go-v0.1.0" -o bin/aicarousel-server ./cmd/server
```

## Upgrading from TypeScript/Bun

The SQLite database is fully compatible. The Go migration runner uses `IF NOT EXISTS` on all
`CREATE TABLE` statements, so starting on an existing TS database does not fail.

On first startup, Go creates the `_migrations` tracking table and marks existing migrations as
already applied — no data is touched (API keys, provider settings all preserved).

## Client configuration

### Cline (VS Code)
```
API Provider: OpenAI Compatible
Base URL: http://localhost:7123/v1    (or https://ai.yourdomain.com/v1)
API Key: sk-xxxx
Model ID: aicarousel
```

### Claude Code
```bash
export ANTHROPIC_BASE_URL=http://localhost:7123
export ANTHROPIC_API_KEY=sk-xxxx
```

### Codex CLI
```bash
export OPENAI_API_BASE=http://localhost:7123/v1
export OPENAI_API_KEY=sk-xxxx
```

### Direct curl
```bash
# OpenAI format
curl https://ai.yourdomain.com/v1/chat/completions \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"Hello"}],"stream":true}'

# Anthropic format
curl https://ai.yourdomain.com/v1/messages \
  -H "x-api-key: sk-xxxx" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"aicarousel","max_tokens":1024,"messages":[{"role":"user","content":"Hello"}]}'
```
