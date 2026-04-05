# CLI — Overview

AICarousel ships three CLI binaries:

| Binary                 | Purpose                                               |
|------------------------|-------------------------------------------------------|
| `aicarousel-server`    | Main HTTP server                                      |
| `aicarousel-setup`     | Interactive configuration menu (TUI)                  |
| `aicarousel-apikey`    | API Key management from terminal (non-interactive)    |

---

## aicarousel-server

```bash
aicarousel-server           # Start the server on the configured port
aicarousel-server migrate   # Run migrations only and exit
```

On startup the server:
- Loads `.env` automatically.
- Runs database migrations.
- Syncs `provider_settings` with `MODELS_CONFIG` / `models.json`.
- Starts listening for HTTP requests.

---

## aicarousel-setup

Interactive menu with 6 options. Navigate with numbers + Enter.

```bash
aicarousel-setup
```

```
  1. Initial setup (database and migrations)
  2. Manage Provider API Keys
  3. Manage Application API Keys
  4. Enable/Disable Providers
  5. Manage Provider Models
  6. View current status
  0. Exit
```

See detailed documentation for each option in the other files in this folder.

---

## aicarousel-apikey

Non-interactive CLI for API Key management from scripts or CI.

```bash
aicarousel-apikey create [name]   # Create a key (name optional)
aicarousel-apikey list            # List all keys
aicarousel-apikey revoke <id>     # Revoke (disable)
aicarousel-apikey delete <id>     # Delete permanently
```

**Exit codes**:
- `0` — success
- `1` — usage error (invalid argument, missing subcommand)
- `2` — runtime error (DB not accessible, operation failed)

See detailed documentation in `apikey-cli.md`.
