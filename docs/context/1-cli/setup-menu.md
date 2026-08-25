# aicarousel-setup — Interactive Menu

## Access

```bash
./bin/aicarousel-setup
# or in Docker:
docker exec -it <container> aicarousel-setup
```

Requires the database to be accessible (`DB_PATH` or `data/aicarousel.db`).

---

## Option 1 — Initial Setup

**When to use**: First time installing the system, or if the database was deleted.

**What it does**:
1. Creates the `data/` directory if it does not exist.
2. Creates `.env` from `.env.template` if it does not exist.
3. Runs database migrations (`CREATE TABLE IF NOT EXISTS`).
4. Syncs providers: inserts all known providers (cerebras, groq, openrouter, gemini, ollama, zai)
   into `provider_settings` if they are not already there.

**Output**:
```
Initial Setup
  ✓ Data directory ready
  ✓ .env file exists
  ✓ Migrations completed
  ✓ 6 providers synced

Setup completed successfully!

Next steps:
  1. Configure provider API Keys (menu option 2)
  2. Create an Application API Key (menu option 3)
  3. Start the server with: aicarousel-server
```

**Note**: Safe to run on already-configured systems — all operations are idempotent.

---

## Option 2 — Manage Provider API Keys

**When to use**: Configure, update, or remove the API keys of AI providers.

**Shows**: Table of all providers with their current status.

```
#   Provider       Variable              Status
──────────────────────────────────────────────────────
1   Cerebras       CEREBRAS_API_KEY      ✓ sk-abc...
2   Groq           GROQ_API_KEY          ✓ gsk-xyz...
3   OpenRouter     OPENROUTER_API_KEY    ✗ Not configured
4   Gemini         GEMINI_API_KEY        ✗ Not configured
5   Ollama         OLLAMA_ENABLED        ✓ true
6   Z.ai           ZAI_API_KEY           ✗ Not configured
```

**Sub-menu for a provider that has a key**:
- `1. Update API Key` — overwrites the value in `.env`
- `2. Remove API Key` — deletes the variable from `.env`

**Sub-menu for a provider without a key**:
- `1. Add API Key` — writes the value to `.env`

**Important**: Changes are written directly to the `.env` file.
The server must be restarted to pick them up. In Docker, restart or redeploy the container.

---

## Option 3 — Manage Application API Keys

**When to use**: Create keys for clients (Cline, Claude Code, etc.) that consume the service.

**Shows**: Table of all created keys.

```
ID   Prefix        Name           Last Used          Status       Usage
──────────────────────────────────────────────────────────────────────
1    sk-abc123...  Production     2026-04-05 17:31   ✓ Active    142
2    sk-def456...  Development    -                  ✓ Active    0
3    sk-ghi789...  Old client     2026-04-01 09:12   ✗ Revoked   23
```

### Create new API Key

```
1. Create new API Key
Name for the new API Key (optional): Production

╔══════════════════════════════════════════════════════════╗
║  Your new API Key (copy it now, it won't be shown again)  ║
║                                                           ║
║  sk-a1b2c3d4e5f6...                                       ║
╚══════════════════════════════════════════════════════════╝

ID:       4
Name:     Production
Created:  2026-04-05 17:30:00
```

- Name is optional but recommended to identify the client.
- **The key is shown only once.** Copy it immediately.
- Only the SHA-256 hash is stored in the DB — never the plain text.

### Revoke API Key

Disables the key (`is_active = 0`). Requests using that key return 401.
The key remains in the DB and still appears in the list.

```
ID of the API Key to revoke: 3
Revoke API Key #3? (y/n): y
✓ API Key #3 revoked
```

### Delete API Key

Permanently removes the row from the DB. **Irreversible.**

```
ID of the API Key to delete: 3
PERMANENTLY DELETE API Key #3? (y/n): y
✓ API Key #3 permanently deleted
```

---

## Option 4 — Enable/Disable Providers

**When to use**: Activate or deactivate providers from the round-robin, or change rotation order.

**Shows**:

```
#   Provider      API Key        Enabled          Order
────────────────────────────────────────────────────────
1   Groq          ✓ Configured   ☑ Active         1
2   Cerebras      ✓ Configured   ☑ Active         2
3   Z.ai          ✓ Configured   ☐ Inactive       -
4   OpenRouter    ✗ Missing      - N/A             -
5   Gemini        ✗ Missing      - N/A             -
6   Ollama        ✗ Missing      - N/A             -
```

Providers without an API key cannot be enabled (shown as `N/A`).

### Toggle provider

```
1. Toggle provider (enable/disable)
Provider number to toggle: 3
✓ Z.ai enabled
```

Enables an inactive provider or disables an active one. The change takes effect immediately
in `provider_settings`. The running server picks it up on the next request.

### Change rotation order

```
2. Change rotation order

Current rotation order:
  1. Groq
  2. Cerebras
  3. Z.ai

Enter the new order separated by commas.
Example: 2,1,3 to put the second one first.

New order: 2,3,1
```

This updates the `priority` field in the `provider_settings` table:
```
✓ Order updated:
  1. Cerebras
  2. Z.ai
  3. Groq
```

---

## Option 5 — Manage Provider Models

**When to use**: Add, edit, or remove models; change the default model; configure fallback.

**Main view**: Table of all providers with their default model, fallback status, and model count.

```
#   Provider      Default                    Fallback   Models
────────────────────────────────────────────────────────────────
1   Cerebras      qwen-3-32b                 ☑          2
2   Groq          llama-3.3-70b-versatile    ☑          2
3   OpenRouter    No config.                 -          0
```

Selecting a provider shows the model list:

```
Models — Groq
──────────────────────────────────────────
#   Model                           Default
1   llama-3.3-70b-versatile         ★ Default
2   llama-3.1-8b-instant

Intra-provider fallback: ✓ Enabled
If the default model fails, others will be tried in order.

  1. Add model
  2. Edit model
  3. Delete model
  4. Change default
  5. Toggle fallback
  6. Reorder models
```

### Add model

```
1. Add model
Enter the model identifier (e.g. llama-3.3-70b-versatile)

Model: mixtral-8x7b-32768
✓ Model "mixtral-8x7b-32768" added.
```

### Edit model

Renames an existing model (useful when a provider changes a model slug).

```
2. Edit model
Model number to edit: 2
Current model: llama-3.1-8b-instant
New name: llama-3.1-8b-instant-128k
✓ Model updated to "llama-3.1-8b-instant-128k".
```

### Delete model

Cannot delete if it is the only model or if it is the default model.

```
3. Delete model
Model number to delete: 3
Delete "mixtral-8x7b-32768"? Type 'yes' to confirm: yes
✓ Model "mixtral-8x7b-32768" deleted.
```

### Change default

```
4. Change default
Available models:
  1. llama-3.3-70b-versatile  ★
  2. llama-3.1-8b-instant

Number of new default: 2
✓ Default changed to "llama-3.1-8b-instant".
```

### Toggle fallback

```
5. Toggle fallback
Fallback currently: enabled
If you disable fallback, only the default model will be used.

Disable fallback? (yes/no): yes
✓ Fallback disabled.
```

With fallback disabled, if the default model fails the router moves directly to the next
provider without trying other models in the same one.

### Reorder models

The order defines fallback priority when `enableFallback: true`.

```
6. Reorder models
Current order (for fallback):
  1. llama-3.3-70b-versatile  ★
  2. llama-3.1-8b-instant

Enter the new order separated by commas.
Example: 2,1 to put the second one first.

New order: 2,1
✓ Order updated:
  1. llama-3.1-8b-instant
  2. llama-3.3-70b-versatile
```

**Note**: Reordering does not change the default model. If you want the first fallback model
to also be the default, use option 4 (Change default) as well.

---

## Option 6 — View Current Status

**When to use**: Quick system diagnostics.

**Output**:

```
System Status

Database
  Location:   data/aicarousel.db
  Status:     ✓ Connected
  Migrations: ✓ Up to date

.env File
  Status: ✓ Exists
  Port:   7123

Providers
  Cerebras     ✓ Active (order: 1)
  Groq         ✓ Active (order: 2)
  OpenRouter   ✗ No API Key
  Gemini       ✗ No API Key
  Ollama       ⊘ Disabled
  Z.ai         ✓ Active (order: 3)

Application API Keys
  Total:    3
  Active:   2
  Revoked:  1

Server
  Port:    7123
  URL:     http://localhost:7123
  Start:   aicarousel-server

Quick Commands
  aicarousel-setup       - This menu
  aicarousel-server      - Start server
  aicarousel-apikey list - List API Keys
```
