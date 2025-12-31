/**
 * Migration: Create api_keys table
 */

import { db } from "../index.ts";

export function up(): void {
  db.exec(`
    CREATE TABLE api_keys (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      key_hash TEXT NOT NULL UNIQUE,
      key_prefix TEXT NOT NULL,
      name TEXT,
      created_at TEXT DEFAULT (datetime('now')),
      last_used_at TEXT,
      is_active INTEGER DEFAULT 1,
      usage_count INTEGER DEFAULT 0
    );

    CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
    CREATE INDEX idx_api_keys_active ON api_keys(is_active);
  `);
}

export function down(): void {
  db.exec(`
    DROP INDEX IF EXISTS idx_api_keys_active;
    DROP INDEX IF EXISTS idx_api_keys_hash;
    DROP TABLE IF EXISTS api_keys;
  `);
}
