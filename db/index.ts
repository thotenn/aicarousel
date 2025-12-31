/**
 * SQLite database connection using Bun's native SQLite.
 */

import { Database } from "bun:sqlite";
import { join } from "path";

const DB_PATH = process.env.DB_PATH || join(import.meta.dir, "..", "data", "aicarousel.db");

// Ensure data directory exists
const dataDir = join(import.meta.dir, "..", "data");
await Bun.write(join(dataDir, ".gitkeep"), "");

// Create database connection
export const db = new Database(DB_PATH, { create: true });

// Enable WAL mode for better concurrent performance
db.exec("PRAGMA journal_mode = WAL;");
db.exec("PRAGMA foreign_keys = ON;");

export { DB_PATH };
