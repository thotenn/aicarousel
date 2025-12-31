/**
 * Migration: Create provider_settings table
 */

import { db } from "../index.ts";

export function up(): void {
  db.exec(`
    CREATE TABLE provider_settings (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      provider_key TEXT NOT NULL UNIQUE,
      is_enabled INTEGER DEFAULT 1,
      priority INTEGER DEFAULT 0,
      created_at TEXT DEFAULT (datetime('now')),
      updated_at TEXT DEFAULT (datetime('now'))
    );

    CREATE INDEX idx_provider_settings_key ON provider_settings(provider_key);
    CREATE INDEX idx_provider_settings_enabled ON provider_settings(is_enabled);

    -- Initialize with all known providers
    INSERT INTO provider_settings (provider_key, is_enabled, priority)
    VALUES
      ('cerebras', 1, 1),
      ('groq', 1, 2),
      ('openrouter', 1, 3),
      ('gemini', 1, 4);
  `);
}

export function down(): void {
  db.exec(`
    DROP INDEX IF EXISTS idx_provider_settings_enabled;
    DROP INDEX IF EXISTS idx_provider_settings_key;
    DROP TABLE IF EXISTS provider_settings;
  `);
}
