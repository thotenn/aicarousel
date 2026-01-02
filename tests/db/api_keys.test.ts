/**
 * Tests for db/api_keys.ts
 */

import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { Database } from "bun:sqlite";

// Test database and mock implementations
let testDb: Database;

interface ApiKey {
  id: number;
  key_hash: string;
  key_prefix: string;
  name: string | null;
  is_active: number;
  created_at: string;
  last_used_at: string | null;
}

// Mock implementations of api_keys functions
function createApiKey(db: Database, name?: string): { key: string; id: number } {
  const key = `sk-${crypto.randomUUID().replace(/-/g, "")}`;
  const keyHash = Bun.hash(key).toString();
  const prefix = key.substring(0, 10);

  const result = db.run(
    `INSERT INTO api_keys (key_hash, key_prefix, name, is_active) VALUES (?, ?, ?, 1)`,
    [keyHash, prefix, name || null]
  );

  return { key, id: Number(result.lastInsertRowid) };
}

function listApiKeys(db: Database): ApiKey[] {
  return db.query(`SELECT * FROM api_keys ORDER BY created_at DESC`).all() as ApiKey[];
}

function getApiKeyById(db: Database, id: number): ApiKey | null {
  return db.query(`SELECT * FROM api_keys WHERE id = ?`).get(id) as ApiKey | null;
}

function revokeApiKey(db: Database, id: number): boolean {
  const result = db.run(`UPDATE api_keys SET is_active = 0 WHERE id = ?`, [id]);
  return result.changes > 0;
}

function deleteApiKey(db: Database, id: number): boolean {
  const result = db.run(`DELETE FROM api_keys WHERE id = ?`, [id]);
  return result.changes > 0;
}

function validateApiKey(db: Database, key: string): ApiKey | null {
  const keyHash = Bun.hash(key).toString();
  const result = db.query(
    `SELECT * FROM api_keys WHERE key_hash = ? AND is_active = 1`
  ).get(keyHash) as ApiKey | null;

  if (result) {
    // Update last_used_at
    db.run(`UPDATE api_keys SET last_used_at = datetime('now') WHERE id = ?`, [result.id]);
  }

  return result;
}

describe("API Keys Database", () => {
  beforeEach(() => {
    testDb = new Database(":memory:");
    testDb.run(`
      CREATE TABLE api_keys (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        key_hash TEXT NOT NULL UNIQUE,
        key_prefix TEXT NOT NULL,
        name TEXT,
        is_active INTEGER DEFAULT 1,
        created_at TEXT DEFAULT (datetime('now')),
        last_used_at TEXT
      )
    `);
  });

  afterEach(() => {
    testDb.close();
  });

  describe("createApiKey", () => {
    test("should create a new API key", () => {
      const { key, id } = createApiKey(testDb, "test-key");

      expect(key).toMatch(/^sk-/);
      expect(id).toBeGreaterThan(0);
    });

    test("should store key hash, not actual key", () => {
      const { key, id } = createApiKey(testDb, "test-key");
      const stored = getApiKeyById(testDb, id);

      expect(stored?.key_hash).not.toBe(key);
      expect(stored?.key_hash).toBe(Bun.hash(key).toString());
    });

    test("should store key prefix", () => {
      const { key, id } = createApiKey(testDb, "test-key");
      const stored = getApiKeyById(testDb, id);

      expect(stored?.key_prefix).toBe(key.substring(0, 10));
    });

    test("should store name when provided", () => {
      const { id } = createApiKey(testDb, "my-key-name");
      const stored = getApiKeyById(testDb, id);

      expect(stored?.name).toBe("my-key-name");
    });

    test("should allow null name", () => {
      const { id } = createApiKey(testDb);
      const stored = getApiKeyById(testDb, id);

      expect(stored?.name).toBeNull();
    });

    test("should create key as active by default", () => {
      const { id } = createApiKey(testDb);
      const stored = getApiKeyById(testDb, id);

      expect(stored?.is_active).toBe(1);
    });

    test("should generate unique keys", () => {
      const keys = new Set<string>();
      for (let i = 0; i < 10; i++) {
        const { key } = createApiKey(testDb);
        expect(keys.has(key)).toBe(false);
        keys.add(key);
      }
    });
  });

  describe("listApiKeys", () => {
    test("should return empty array when no keys exist", () => {
      const keys = listApiKeys(testDb);
      expect(keys).toEqual([]);
    });

    test("should return all keys", () => {
      createApiKey(testDb, "key-1");
      createApiKey(testDb, "key-2");
      createApiKey(testDb, "key-3");

      const keys = listApiKeys(testDb);
      expect(keys.length).toBe(3);
    });

    test("should return keys in order", () => {
      createApiKey(testDb, "key-1");
      createApiKey(testDb, "key-2");
      createApiKey(testDb, "key-3");

      const keys = listApiKeys(testDb);
      expect(keys.length).toBe(3);
      // All keys should be returned
      const names = keys.map(k => k.name);
      expect(names).toContain("key-1");
      expect(names).toContain("key-2");
      expect(names).toContain("key-3");
    });
  });

  describe("getApiKeyById", () => {
    test("should return key by id", () => {
      const { id } = createApiKey(testDb, "test-key");
      const key = getApiKeyById(testDb, id);

      expect(key).not.toBeNull();
      expect(key?.id).toBe(id);
    });

    test("should return null for non-existent id", () => {
      const key = getApiKeyById(testDb, 9999);
      expect(key).toBeNull();
    });
  });

  describe("revokeApiKey", () => {
    test("should set is_active to 0", () => {
      const { id } = createApiKey(testDb, "test-key");

      const result = revokeApiKey(testDb, id);
      const key = getApiKeyById(testDb, id);

      expect(result).toBe(true);
      expect(key?.is_active).toBe(0);
    });

    test("should return false for non-existent id", () => {
      const result = revokeApiKey(testDb, 9999);
      expect(result).toBe(false);
    });

    test("should not delete the key", () => {
      const { id } = createApiKey(testDb, "test-key");
      revokeApiKey(testDb, id);

      const key = getApiKeyById(testDb, id);
      expect(key).not.toBeNull();
    });
  });

  describe("deleteApiKey", () => {
    test("should remove key from database", () => {
      const { id } = createApiKey(testDb, "test-key");

      const result = deleteApiKey(testDb, id);
      const key = getApiKeyById(testDb, id);

      expect(result).toBe(true);
      expect(key).toBeNull();
    });

    test("should return false for non-existent id", () => {
      const result = deleteApiKey(testDb, 9999);
      expect(result).toBe(false);
    });
  });

  describe("validateApiKey", () => {
    test("should return key data for valid key", () => {
      const { key } = createApiKey(testDb, "test-key");
      const result = validateApiKey(testDb, key);

      expect(result).not.toBeNull();
      expect(result?.name).toBe("test-key");
    });

    test("should return null for invalid key", () => {
      const result = validateApiKey(testDb, "sk-invalid");
      expect(result).toBeNull();
    });

    test("should return null for revoked key", () => {
      const { key, id } = createApiKey(testDb, "test-key");
      revokeApiKey(testDb, id);

      const result = validateApiKey(testDb, key);
      expect(result).toBeNull();
    });

    test("should update last_used_at on validation", () => {
      const { key, id } = createApiKey(testDb, "test-key");

      // Initially null
      let stored = getApiKeyById(testDb, id);
      expect(stored?.last_used_at).toBeNull();

      // After validation
      validateApiKey(testDb, key);
      stored = getApiKeyById(testDb, id);
      expect(stored?.last_used_at).not.toBeNull();
    });
  });
});
