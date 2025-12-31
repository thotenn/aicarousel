#!/usr/bin/env bun
/**
 * API Key Management CLI
 *
 * Usage:
 *   bun run scripts/api_key.ts create [name]   - Create a new API key
 *   bun run scripts/api_key.ts list            - List all API keys
 *   bun run scripts/api_key.ts revoke <id>     - Revoke an API key
 *   bun run scripts/api_key.ts delete <id>     - Delete an API key
 */

import { migrate } from "../db/migrate.ts";
import {
  createApiKey,
  listApiKeys,
  revokeApiKey,
  deleteApiKey,
} from "../db/api_keys.ts";

const [command, ...args] = process.argv.slice(2);

// Ensure migrations are run
await migrate();

async function main() {
  switch (command) {
    case "create": {
      const name = args[0];
      const { key, record } = await createApiKey(name);

      console.log("\n╔════════════════════════════════════════════════════════════════════╗");
      console.log("║                    🔑 NEW API KEY CREATED                          ║");
      console.log("╠════════════════════════════════════════════════════════════════════╣");
      console.log("║                                                                    ║");
      console.log(`║  ${key}  ║`);
      console.log("║                                                                    ║");
      console.log("╠════════════════════════════════════════════════════════════════════╣");
      console.log("║  ⚠️  Save this key now. You won't be able to see it again!         ║");
      console.log("╚════════════════════════════════════════════════════════════════════╝\n");

      console.log("Details:");
      console.log(`  ID:      ${record.id}`);
      console.log(`  Name:    ${record.name || "(none)"}`);
      console.log(`  Prefix:  ${record.key_prefix}`);
      console.log(`  Created: ${record.created_at}`);
      console.log();
      console.log("Usage:");
      console.log(`  curl -H "Authorization: Bearer ${key}" http://localhost:7123/v1/chat/completions ...`);
      console.log(`  curl -H "x-api-key: ${key}" http://localhost:7123/v1/messages ...`);
      console.log();
      break;
    }

    case "list": {
      const keys = listApiKeys();

      if (keys.length === 0) {
        console.log("\nNo API keys found. Create one with: bun run scripts/api_key.ts create [name]\n");
        break;
      }

      console.log("\n┌────┬─────────────┬──────────────────┬─────────────────────┬─────────────────────┬────────┬───────────┐");
      console.log("│ ID │ Prefix      │ Name             │ Created             │ Last Used           │ Active │ Usage     │");
      console.log("├────┼─────────────┼──────────────────┼─────────────────────┼─────────────────────┼────────┼───────────┤");

      for (const key of keys) {
        const id = String(key.id).padEnd(2);
        const prefix = key.key_prefix.padEnd(11);
        const name = (key.name || "-").slice(0, 16).padEnd(16);
        const created = key.created_at.slice(0, 19);
        const lastUsed = key.last_used_at?.slice(0, 19) || "-".padEnd(19);
        const active = key.is_active ? "✓".padEnd(6) : "✗".padEnd(6);
        const usage = String(key.usage_count).padEnd(9);

        console.log(`│ ${id} │ ${prefix} │ ${name} │ ${created} │ ${lastUsed} │ ${active} │ ${usage} │`);
      }

      console.log("└────┴─────────────┴──────────────────┴─────────────────────┴─────────────────────┴────────┴───────────┘\n");
      break;
    }

    case "revoke": {
      const id = parseInt(args[0]);
      if (isNaN(id)) {
        console.error("Error: Please provide a valid API key ID");
        process.exit(1);
      }

      const success = revokeApiKey(id);
      if (success) {
        console.log(`\n✓ API key ${id} has been revoked\n`);
      } else {
        console.error(`\n✗ API key ${id} not found\n`);
        process.exit(1);
      }
      break;
    }

    case "delete": {
      const id = parseInt(args[0]);
      if (isNaN(id)) {
        console.error("Error: Please provide a valid API key ID");
        process.exit(1);
      }

      const success = deleteApiKey(id);
      if (success) {
        console.log(`\n✓ API key ${id} has been deleted\n`);
      } else {
        console.error(`\n✗ API key ${id} not found\n`);
        process.exit(1);
      }
      break;
    }

    default: {
      console.log(`
API Key Management CLI

Usage:
  bun run scripts/api_key.ts create [name]   Create a new API key
  bun run scripts/api_key.ts list            List all API keys
  bun run scripts/api_key.ts revoke <id>     Revoke an API key (disable)
  bun run scripts/api_key.ts delete <id>     Delete an API key permanently

Examples:
  bun run scripts/api_key.ts create "Production"
  bun run scripts/api_key.ts create "Development"
  bun run scripts/api_key.ts list
  bun run scripts/api_key.ts revoke 1
`);
      break;
    }
  }
}

main().catch(console.error);
