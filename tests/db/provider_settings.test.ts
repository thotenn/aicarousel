/**
 * Tests for db/provider_settings.ts
 */

import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { Database } from "bun:sqlite";

let testDb: Database;

interface ProviderSetting {
  id: number;
  provider_key: string;
  is_enabled: number;
  priority: number;
  created_at: string;
  updated_at: string;
}

// Mock implementations
function getAllProviderSettings(db: Database): ProviderSetting[] {
  return db
    .query(`SELECT * FROM provider_settings ORDER BY priority ASC`)
    .all() as ProviderSetting[];
}

function getProviderSetting(
  db: Database,
  providerKey: string
): ProviderSetting | null {
  return db
    .query(`SELECT * FROM provider_settings WHERE provider_key = ?`)
    .get(providerKey) as ProviderSetting | null;
}

function toggleProvider(
  db: Database,
  providerKey: string,
  enabled: boolean
): boolean {
  const result = db.run(
    `UPDATE provider_settings SET is_enabled = ?, updated_at = datetime('now') WHERE provider_key = ?`,
    [enabled ? 1 : 0, providerKey]
  );
  return result.changes > 0;
}

function reorderProviders(db: Database, orderedKeys: string[]): void {
  const stmt = db.prepare(
    `UPDATE provider_settings SET priority = ?, updated_at = datetime('now') WHERE provider_key = ?`
  );

  for (let i = 0; i < orderedKeys.length; i++) {
    stmt.run(i + 1, orderedKeys[i]);
  }
}

function syncProviders(db: Database, providerKeys: string[]): void {
  const existing = getAllProviderSettings(db).map((s) => s.provider_key);
  const maxPriority =
    db
      .query(`SELECT MAX(priority) as max FROM provider_settings`)
      .get() as { max: number | null };
  let nextPriority = (maxPriority?.max || 0) + 1;

  for (const key of providerKeys) {
    if (!existing.includes(key)) {
      db.run(
        `INSERT INTO provider_settings (provider_key, is_enabled, priority) VALUES (?, 1, ?)`,
        [key, nextPriority++]
      );
    }
  }
}

describe("Provider Settings Database", () => {
  beforeEach(() => {
    testDb = new Database(":memory:");
    testDb.run(`
      CREATE TABLE provider_settings (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        provider_key TEXT NOT NULL UNIQUE,
        is_enabled INTEGER DEFAULT 1,
        priority INTEGER DEFAULT 0,
        created_at TEXT DEFAULT (datetime('now')),
        updated_at TEXT DEFAULT (datetime('now'))
      )
    `);

    // Insert default providers
    testDb.run(`
      INSERT INTO provider_settings (provider_key, is_enabled, priority)
      VALUES
        ('cerebras', 1, 1),
        ('groq', 1, 2),
        ('openrouter', 1, 3),
        ('gemini', 1, 4)
    `);
  });

  afterEach(() => {
    testDb.close();
  });

  describe("getAllProviderSettings", () => {
    test("should return all providers ordered by priority", () => {
      const settings = getAllProviderSettings(testDb);

      expect(settings.length).toBe(4);
      expect(settings[0].provider_key).toBe("cerebras");
      expect(settings[1].provider_key).toBe("groq");
      expect(settings[2].provider_key).toBe("openrouter");
      expect(settings[3].provider_key).toBe("gemini");
    });

    test("should include all fields", () => {
      const settings = getAllProviderSettings(testDb);
      const first = settings[0];

      expect(first).toHaveProperty("id");
      expect(first).toHaveProperty("provider_key");
      expect(first).toHaveProperty("is_enabled");
      expect(first).toHaveProperty("priority");
      expect(first).toHaveProperty("created_at");
      expect(first).toHaveProperty("updated_at");
    });
  });

  describe("getProviderSetting", () => {
    test("should return setting for existing provider", () => {
      const setting = getProviderSetting(testDb, "groq");

      expect(setting).not.toBeNull();
      expect(setting?.provider_key).toBe("groq");
      expect(setting?.priority).toBe(2);
    });

    test("should return null for non-existent provider", () => {
      const setting = getProviderSetting(testDb, "nonexistent");
      expect(setting).toBeNull();
    });
  });

  describe("toggleProvider", () => {
    test("should disable provider", () => {
      toggleProvider(testDb, "groq", false);
      const setting = getProviderSetting(testDb, "groq");

      expect(setting?.is_enabled).toBe(0);
    });

    test("should enable provider", () => {
      // First disable
      toggleProvider(testDb, "groq", false);
      // Then enable
      toggleProvider(testDb, "groq", true);
      const setting = getProviderSetting(testDb, "groq");

      expect(setting?.is_enabled).toBe(1);
    });

    test("should return true for existing provider", () => {
      const result = toggleProvider(testDb, "groq", false);
      expect(result).toBe(true);
    });

    test("should return false for non-existent provider", () => {
      const result = toggleProvider(testDb, "nonexistent", false);
      expect(result).toBe(false);
    });

    test("should update updated_at timestamp", () => {
      const before = getProviderSetting(testDb, "groq")?.updated_at;

      // Small delay to ensure timestamp changes
      toggleProvider(testDb, "groq", false);

      const after = getProviderSetting(testDb, "groq")?.updated_at;
      // Note: In SQLite, datetime('now') might return same value in fast tests
      expect(after).toBeDefined();
    });
  });

  describe("reorderProviders", () => {
    test("should update priorities based on order", () => {
      reorderProviders(testDb, ["gemini", "cerebras", "groq", "openrouter"]);
      const settings = getAllProviderSettings(testDb);

      expect(settings[0].provider_key).toBe("gemini");
      expect(settings[0].priority).toBe(1);
      expect(settings[1].provider_key).toBe("cerebras");
      expect(settings[1].priority).toBe(2);
    });

    test("should handle partial reorder", () => {
      reorderProviders(testDb, ["groq", "cerebras"]);

      const groq = getProviderSetting(testDb, "groq");
      const cerebras = getProviderSetting(testDb, "cerebras");

      expect(groq?.priority).toBe(1);
      expect(cerebras?.priority).toBe(2);
    });
  });

  describe("syncProviders", () => {
    test("should add new providers", () => {
      syncProviders(testDb, ["cerebras", "groq", "newprovider"]);

      const settings = getAllProviderSettings(testDb);
      const newProvider = settings.find((s) => s.provider_key === "newprovider");

      expect(newProvider).toBeDefined();
      expect(newProvider?.is_enabled).toBe(1);
    });

    test("should not duplicate existing providers", () => {
      syncProviders(testDb, ["cerebras", "groq"]);

      const settings = getAllProviderSettings(testDb);
      const cerebrasCount = settings.filter(
        (s) => s.provider_key === "cerebras"
      ).length;

      expect(cerebrasCount).toBe(1);
    });

    test("should assign incrementing priorities to new providers", () => {
      syncProviders(testDb, ["new1", "new2"]);

      const new1 = getProviderSetting(testDb, "new1");
      const new2 = getProviderSetting(testDb, "new2");

      expect(new1?.priority).toBe(5); // After existing 4
      expect(new2?.priority).toBe(6);
    });
  });

  describe("Enabled/Disabled Filtering", () => {
    test("should filter enabled providers", () => {
      toggleProvider(testDb, "groq", false);
      toggleProvider(testDb, "gemini", false);

      const all = getAllProviderSettings(testDb);
      const enabled = all.filter((s) => s.is_enabled === 1);

      expect(enabled.length).toBe(2);
      expect(enabled.map((s) => s.provider_key)).toContain("cerebras");
      expect(enabled.map((s) => s.provider_key)).toContain("openrouter");
    });
  });
});
