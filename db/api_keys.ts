/**
 * API Keys repository.
 * Handles all database operations for API keys.
 */

import { db } from "./index.ts";

export interface ApiKey {
  id: number;
  key_hash: string;
  key_prefix: string;
  name: string | null;
  created_at: string;
  last_used_at: string | null;
  is_active: number;
  usage_count: number;
}

/**
 * Generate a secure random API key.
 * Format: sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx (40 chars total)
 */
export function generateApiKey(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(32));
  const key = Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
  return `sk-${key}`;
}

/**
 * Hash an API key using SHA-256.
 */
export async function hashApiKey(key: string): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(key);
  const hashBuffer = await crypto.subtle.digest("SHA-256", data);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  return hashArray.map((b) => b.toString(16).padStart(2, "0")).join("");
}

/**
 * Create a new API key.
 * Returns the plain key (only shown once) and the database record.
 */
export async function createApiKey(name?: string): Promise<{ key: string; record: ApiKey }> {
  const key = generateApiKey();
  const keyHash = await hashApiKey(key);
  const keyPrefix = key.slice(0, 7) + "..."; // sk-xxx...

  const stmt = db.prepare(`
    INSERT INTO api_keys (key_hash, key_prefix, name)
    VALUES (?, ?, ?)
    RETURNING *
  `);

  const record = stmt.get(keyHash, keyPrefix, name || null) as ApiKey;

  return { key, record };
}

/**
 * Validate an API key.
 * Returns the API key record if valid, null otherwise.
 */
export async function validateApiKey(key: string): Promise<ApiKey | null> {
  if (!key || !key.startsWith("sk-")) {
    return null;
  }

  const keyHash = await hashApiKey(key);

  const stmt = db.prepare(`
    SELECT * FROM api_keys
    WHERE key_hash = ? AND is_active = 1
  `);

  const record = stmt.get(keyHash) as ApiKey | null;

  if (record) {
    // Update last_used_at and usage_count
    db.run(`
      UPDATE api_keys
      SET last_used_at = datetime('now'), usage_count = usage_count + 1
      WHERE id = ?
    `, [record.id]);
  }

  return record;
}

/**
 * List all API keys (without hashes).
 */
export function listApiKeys(): Omit<ApiKey, "key_hash">[] {
  const stmt = db.prepare(`
    SELECT id, key_prefix, name, created_at, last_used_at, is_active, usage_count
    FROM api_keys
    ORDER BY created_at DESC
  `);

  return stmt.all() as Omit<ApiKey, "key_hash">[];
}

/**
 * Revoke an API key by ID.
 */
export function revokeApiKey(id: number): boolean {
  const result = db.run(`
    UPDATE api_keys SET is_active = 0 WHERE id = ?
  `, [id]);

  return result.changes > 0;
}

/**
 * Delete an API key by ID.
 */
export function deleteApiKey(id: number): boolean {
  const result = db.run(`
    DELETE FROM api_keys WHERE id = ?
  `, [id]);

  return result.changes > 0;
}

/**
 * Get API key by ID.
 */
export function getApiKeyById(id: number): ApiKey | null {
  const stmt = db.prepare("SELECT * FROM api_keys WHERE id = ?");
  return stmt.get(id) as ApiKey | null;
}
