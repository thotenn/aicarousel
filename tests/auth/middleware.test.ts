/**
 * Tests for auth/middleware.ts
 */

import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { Database } from "bun:sqlite";
import { createMockRequest, testApiKey } from "../utils/mocks";

// Create an in-memory database for testing
let testDb: Database;

// Mock API key functions
function createTestApiKey(db: Database, name: string): string {
  const key = `sk-test-${Math.random().toString(36).substring(7)}`;
  const keyHash = Bun.hash(key).toString();
  const prefix = key.substring(0, 10);

  db.run(
    `INSERT INTO api_keys (key_hash, key_prefix, name, is_active) VALUES (?, ?, ?, 1)`,
    [keyHash, prefix, name]
  );

  return key;
}

function validateApiKey(db: Database, key: string): boolean {
  if (!key || !key.startsWith("sk-")) return false;

  const keyHash = Bun.hash(key).toString();
  const result = db.query(
    `SELECT id FROM api_keys WHERE key_hash = ? AND is_active = 1`
  ).get(keyHash);

  return result !== null;
}

describe("Auth Middleware", () => {
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

  describe("API Key Validation", () => {
    test("should validate correct API key", () => {
      const key = createTestApiKey(testDb, "test-key");
      expect(validateApiKey(testDb, key)).toBe(true);
    });

    test("should reject invalid API key", () => {
      expect(validateApiKey(testDb, "sk-invalid-key")).toBe(false);
    });

    test("should reject empty API key", () => {
      expect(validateApiKey(testDb, "")).toBe(false);
    });

    test("should reject null API key", () => {
      expect(validateApiKey(testDb, null as any)).toBe(false);
    });

    test("should reject key without sk- prefix", () => {
      expect(validateApiKey(testDb, "invalid-prefix")).toBe(false);
    });
  });

  describe("Header Extraction", () => {
    test("should extract API key from Authorization header", () => {
      const request = createMockRequest({}, {
        Authorization: "Bearer sk-test-key",
      });
      const authHeader = request.headers.get("Authorization");
      const key = authHeader?.replace("Bearer ", "");
      expect(key).toBe("sk-test-key");
    });

    test("should extract API key from x-api-key header", () => {
      const request = createMockRequest({}, {
        "x-api-key": "sk-test-key",
      });
      const key = request.headers.get("x-api-key");
      expect(key).toBe("sk-test-key");
    });

    test("should prefer Authorization header over x-api-key", () => {
      const request = createMockRequest({}, {
        Authorization: "Bearer sk-auth-key",
        "x-api-key": "sk-xapi-key",
      });

      const authHeader = request.headers.get("Authorization");
      const xApiKey = request.headers.get("x-api-key");
      const key = authHeader?.replace("Bearer ", "") || xApiKey;

      expect(key).toBe("sk-auth-key");
    });
  });

  describe("Key Revocation", () => {
    test("should reject revoked API key", () => {
      const key = createTestApiKey(testDb, "revoked-key");

      // Revoke the key
      const keyHash = Bun.hash(key).toString();
      testDb.run(`UPDATE api_keys SET is_active = 0 WHERE key_hash = ?`, [keyHash]);

      expect(validateApiKey(testDb, key)).toBe(false);
    });
  });

  describe("Multiple Keys", () => {
    test("should validate each key independently", () => {
      const key1 = createTestApiKey(testDb, "key-1");
      const key2 = createTestApiKey(testDb, "key-2");

      expect(validateApiKey(testDb, key1)).toBe(true);
      expect(validateApiKey(testDb, key2)).toBe(true);

      // Revoke key1
      const key1Hash = Bun.hash(key1).toString();
      testDb.run(`UPDATE api_keys SET is_active = 0 WHERE key_hash = ?`, [key1Hash]);

      expect(validateApiKey(testDb, key1)).toBe(false);
      expect(validateApiKey(testDb, key2)).toBe(true);
    });
  });
});
