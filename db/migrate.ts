/**
 * Database migration system.
 * Runs all pending migrations in order.
 */

import { db } from "./index.ts";
import { readdirSync } from "fs";
import { join } from "path";

// Create migrations table if not exists
db.exec(`
  CREATE TABLE IF NOT EXISTS _migrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    executed_at TEXT DEFAULT (datetime('now'))
  );
`);

interface Migration {
  name: string;
  up: () => void;
  down: () => void;
}

/**
 * Get list of executed migrations from database.
 */
function getExecutedMigrations(): string[] {
  const rows = db.query("SELECT name FROM _migrations ORDER BY id").all() as { name: string }[];
  return rows.map((r) => r.name);
}

/**
 * Load all migration files from the migrations directory.
 */
async function loadMigrations(): Promise<Migration[]> {
  const migrationsDir = join(import.meta.dir, "migrations");
  const files = readdirSync(migrationsDir)
    .filter((f) => f.endsWith(".ts"))
    .sort();

  const migrations: Migration[] = [];

  for (const file of files) {
    const module = await import(join(migrationsDir, file));
    migrations.push({
      name: file.replace(".ts", ""),
      up: module.up,
      down: module.down,
    });
  }

  return migrations;
}

/**
 * Run all pending migrations.
 */
export async function migrate(): Promise<void> {
  const executed = getExecutedMigrations();
  const migrations = await loadMigrations();

  const pending = migrations.filter((m) => !executed.includes(m.name));

  if (pending.length === 0) {
    console.log("✓ No pending migrations");
    return;
  }

  console.log(`Running ${pending.length} migration(s)...`);

  for (const migration of pending) {
    console.log(`  → ${migration.name}`);
    try {
      migration.up();
      db.run("INSERT INTO _migrations (name) VALUES (?)", [migration.name]);
      console.log(`    ✓ Done`);
    } catch (error) {
      console.error(`    ✗ Failed:`, error);
      throw error;
    }
  }

  console.log("✓ All migrations completed");
}

/**
 * Rollback the last migration.
 */
export async function rollback(): Promise<void> {
  const executed = getExecutedMigrations();
  if (executed.length === 0) {
    console.log("✓ No migrations to rollback");
    return;
  }

  const lastMigration = executed[executed.length - 1];
  const migrations = await loadMigrations();
  const migration = migrations.find((m) => m.name === lastMigration);

  if (!migration) {
    throw new Error(`Migration ${lastMigration} not found`);
  }

  console.log(`Rolling back: ${migration.name}`);
  try {
    migration.down();
    db.run("DELETE FROM _migrations WHERE name = ?", [migration.name]);
    console.log("✓ Rollback completed");
  } catch (error) {
    console.error("✗ Rollback failed:", error);
    throw error;
  }
}

// Run migrations if this file is executed directly
if (import.meta.main) {
  const command = process.argv[2];

  if (command === "rollback") {
    await rollback();
  } else {
    await migrate();
  }
}
