/**
 * .env file management utilities.
 */

import { join } from "path";

const ENV_PATH = join(import.meta.dir, "..", "..", ".env");
const ENV_TEMPLATE_PATH = join(import.meta.dir, "..", "..", ".env.template");

/**
 * Read .env file and parse into key-value pairs.
 */
export async function readEnvFile(): Promise<Record<string, string>> {
  const env: Record<string, string> = {};

  try {
    const file = Bun.file(ENV_PATH);
    if (!(await file.exists())) {
      return env;
    }

    const content = await file.text();
    const lines = content.split("\n");

    for (const line of lines) {
      const trimmed = line.trim();

      // Skip empty lines and comments
      if (!trimmed || trimmed.startsWith("#")) {
        continue;
      }

      const eqIndex = trimmed.indexOf("=");
      if (eqIndex > 0) {
        const key = trimmed.slice(0, eqIndex).trim();
        let value = trimmed.slice(eqIndex + 1).trim();

        // Remove quotes if present
        if ((value.startsWith('"') && value.endsWith('"')) ||
            (value.startsWith("'") && value.endsWith("'"))) {
          value = value.slice(1, -1);
        }

        env[key] = value;
      }
    }
  } catch (error) {
    // File doesn't exist or can't be read
  }

  return env;
}

/**
 * Write a value to .env file.
 * If the key exists, update it. Otherwise, append it.
 */
export async function writeEnvValue(key: string, value: string): Promise<void> {
  const file = Bun.file(ENV_PATH);
  let content = "";

  if (await file.exists()) {
    content = await file.text();
  }

  const lines = content.split("\n");
  let found = false;
  const newLines: string[] = [];

  for (const line of lines) {
    const trimmed = line.trim();

    // Check if this line has our key
    if (trimmed.startsWith(`${key}=`) || trimmed.startsWith(`${key} =`)) {
      newLines.push(`${key}=${value}`);
      found = true;
    } else {
      newLines.push(line);
    }
  }

  // If key wasn't found, append it
  if (!found) {
    // Add newline before if file doesn't end with one
    if (newLines.length > 0 && newLines[newLines.length - 1] !== "") {
      newLines.push("");
    }
    newLines.push(`${key}=${value}`);
  }

  // Write back
  await Bun.write(ENV_PATH, newLines.join("\n"));

  // Also update process.env for current session
  process.env[key] = value;
}

/**
 * Remove a value from .env file.
 */
export async function removeEnvValue(key: string): Promise<void> {
  const file = Bun.file(ENV_PATH);

  if (!(await file.exists())) {
    return;
  }

  const content = await file.text();
  const lines = content.split("\n");
  const newLines = lines.filter((line) => {
    const trimmed = line.trim();
    return !trimmed.startsWith(`${key}=`) && !trimmed.startsWith(`${key} =`);
  });

  await Bun.write(ENV_PATH, newLines.join("\n"));

  // Also remove from process.env
  delete process.env[key];
}

/**
 * Check if .env file exists.
 */
export async function envFileExists(): Promise<boolean> {
  return await Bun.file(ENV_PATH).exists();
}

/**
 * Check if .env.template file exists.
 */
export async function envTemplateExists(): Promise<boolean> {
  return await Bun.file(ENV_TEMPLATE_PATH).exists();
}

/**
 * Create .env from .env.template if it doesn't exist.
 */
export async function createEnvFromTemplate(): Promise<boolean> {
  if (await envFileExists()) {
    return false; // Already exists
  }

  if (!(await envTemplateExists())) {
    // Create a basic .env file
    await Bun.write(ENV_PATH, `# AICarousel Environment Configuration
PORT=7123

# Provider API Keys
CEREBRAS_API_KEY=
GROQ_API_KEY=
OPENROUTER_API_KEY=
GEMINI_API_KEY=
`);
    return true;
  }

  // Copy template
  const template = await Bun.file(ENV_TEMPLATE_PATH).text();
  await Bun.write(ENV_PATH, template);
  return true;
}

/**
 * Get a value from the current environment (process.env or .env file).
 */
export function getEnvValue(key: string): string | undefined {
  return process.env[key];
}

/**
 * Check if a key has a value set.
 */
export function hasEnvValue(key: string): boolean {
  const value = process.env[key];
  return !!value && value.trim() !== "";
}

/**
 * Mask an API key for display (show first 7 and last 4 chars).
 */
export function maskKey(key: string | undefined): string {
  if (!key || key.trim() === "") return "";
  if (key.length <= 11) return "***";
  return `${key.slice(0, 7)}...${key.slice(-4)}`;
}

/**
 * Get path to .env file.
 */
export function getEnvPath(): string {
  return ENV_PATH;
}
