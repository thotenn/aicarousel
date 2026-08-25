CREATE TABLE IF NOT EXISTS provider_settings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  provider_key TEXT NOT NULL UNIQUE,
  is_enabled INTEGER DEFAULT 1,
  priority INTEGER DEFAULT 0,
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_provider_settings_key ON provider_settings(provider_key);
CREATE INDEX IF NOT EXISTS idx_provider_settings_enabled ON provider_settings(is_enabled);

INSERT OR IGNORE INTO provider_settings (provider_key, is_enabled, priority) VALUES
  ('cerebras', 1, 1),
  ('groq', 1, 2),
  ('openrouter', 1, 3),
  ('gemini', 1, 4);
