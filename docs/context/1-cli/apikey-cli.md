# aicarousel-apikey — Non-Interactive CLI

CLI for API Key management from terminal, scripts, or CI/CD pipelines.
Requires no human interaction — designed for automation.

## Usage

```bash
aicarousel-apikey <subcommand> [arguments]
```

If no subcommand is provided, prints help and exits with code 1.

## Subcommands

---

### create

Creates a new API Key.

```bash
aicarousel-apikey create [name]
```

- `name` is optional. If omitted, the key is created without a name.
- Names with spaces must be quoted.

**Output**:
```
╔══════════════════════════════════════════════════════════════╗
║  Your new API Key (copy it now, it won't be shown again)     ║
║                                                              ║
║  sk-a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6  ║
╚══════════════════════════════════════════════════════════════╝

ID:       4
Name:     Production
Created:  2026-04-05 17:30:00
```

**The key is shown only once.** Save it immediately.

**Examples**:
```bash
aicarousel-apikey create "Production"
aicarousel-apikey create "Claude Code dev"
aicarousel-apikey create          # no name
```

---

### list

Lists all API Keys (active and revoked).

```bash
aicarousel-apikey list
```

**Output**:
```
ID   Prefix          Name              Created              Last Used            Active   Usage
─────────────────────────────────────────────────────────────────────────────────────────────
1    sk-a1b2c3d4...  Production        2026-04-01 10:00     2026-04-05 17:31     true     142
2    sk-e5f6g7h8...  Development       2026-04-02 09:00     -                    true     0
3    sk-i9j0k1l2...  Old client        2026-04-01 11:00     2026-04-01 15:00     false    23
```

- **Prefix**: First 10 characters + `...` — lets you identify the key without exposing the full value.
- **Active**: `true` = works, `false` = revoked (returns 401).
- **Usage**: Number of authenticated requests made with this key.

---

### revoke

Disables an API Key. Requests using that key return 401.
The key remains in the DB and still appears in `list`.

```bash
aicarousel-apikey revoke <id>
```

**Example**:
```bash
aicarousel-apikey revoke 3
```

**Output**:
```
API Key #3 revoked. It can no longer be used to authenticate.
```

If the key was already revoked or the ID does not exist:
```
API Key #3 not found or already revoked.
```

---

### delete

Permanently removes an API Key from the DB. **Irreversible.**

```bash
aicarousel-apikey delete <id>
```

**Example**:
```bash
aicarousel-apikey delete 3
```

**Output**:
```
API Key #3 deleted.
```

---

## Exit codes

| Code | Meaning                                                   |
|------|-----------------------------------------------------------|
| `0`  | Success                                                   |
| `1`  | Usage error: unknown subcommand, invalid argument         |
| `2`  | Runtime error: DB not accessible, operation failed        |

Useful in scripts:
```bash
aicarousel-apikey revoke 5
if [ $? -ne 0 ]; then
  echo "Failed to revoke key"
fi
```

## Prerequisites

The CLI needs the database to exist and have migrations applied.
If the DB is not initialized, it outputs:

```
error: cannot open database: ...
Run 'aicarousel-setup' first to initialize the database.
```

Solution: run `aicarousel-setup` → option 1 first.

## Usage in Docker

```bash
# Create a key from outside the container
docker exec <container_name> aicarousel-apikey create "Production"

# List keys
docker exec <container_name> aicarousel-apikey list

# Revoke
docker exec <container_name> aicarousel-apikey revoke 2
```
