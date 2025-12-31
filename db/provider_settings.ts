/**
 * Provider settings repository.
 * Manages provider enable/disable status and priority order.
 */

import { db } from "./index.ts";

export interface ProviderSetting {
  id: number;
  provider_key: string;
  is_enabled: number;
  priority: number;
  created_at: string;
  updated_at: string;
}

/**
 * Get all provider settings.
 */
export function getAllProviderSettings(): ProviderSetting[] {
  const stmt = db.prepare(`
    SELECT * FROM provider_settings
    ORDER BY priority ASC
  `);
  return stmt.all() as ProviderSetting[];
}

/**
 * Get settings for a specific provider.
 */
export function getProviderSetting(providerKey: string): ProviderSetting | null {
  const stmt = db.prepare(`
    SELECT * FROM provider_settings
    WHERE provider_key = ?
  `);
  return stmt.get(providerKey) as ProviderSetting | null;
}

/**
 * Get only enabled provider settings, ordered by priority.
 */
export function getEnabledProviderSettings(): ProviderSetting[] {
  const stmt = db.prepare(`
    SELECT * FROM provider_settings
    WHERE is_enabled = 1
    ORDER BY priority ASC
  `);
  return stmt.all() as ProviderSetting[];
}

/**
 * Toggle a provider's enabled status.
 */
export function toggleProvider(providerKey: string, enabled: boolean): boolean {
  const result = db.run(`
    UPDATE provider_settings
    SET is_enabled = ?, updated_at = datetime('now')
    WHERE provider_key = ?
  `, [enabled ? 1 : 0, providerKey]);

  return result.changes > 0;
}

/**
 * Enable a provider.
 */
export function enableProvider(providerKey: string): boolean {
  return toggleProvider(providerKey, true);
}

/**
 * Disable a provider.
 */
export function disableProvider(providerKey: string): boolean {
  return toggleProvider(providerKey, false);
}

/**
 * Update a provider's priority (order in rotation).
 */
export function updateProviderPriority(providerKey: string, priority: number): boolean {
  const result = db.run(`
    UPDATE provider_settings
    SET priority = ?, updated_at = datetime('now')
    WHERE provider_key = ?
  `, [priority, providerKey]);

  return result.changes > 0;
}

/**
 * Reorder providers by setting new priorities.
 * Accepts an array of provider keys in desired order.
 */
export function reorderProviders(orderedKeys: string[]): void {
  const stmt = db.prepare(`
    UPDATE provider_settings
    SET priority = ?, updated_at = datetime('now')
    WHERE provider_key = ?
  `);

  for (let i = 0; i < orderedKeys.length; i++) {
    stmt.run(i + 1, orderedKeys[i]);
  }
}

/**
 * Ensure a provider exists in the settings table.
 * Used when new providers are added to the codebase.
 */
export function ensureProviderExists(providerKey: string, priority?: number): void {
  const existing = getProviderSetting(providerKey);

  if (!existing) {
    // Get max priority if not specified
    const maxPriority = priority ??
      ((db.query("SELECT MAX(priority) as max FROM provider_settings").get() as { max: number | null })?.max ?? 0) + 1;

    db.run(`
      INSERT INTO provider_settings (provider_key, is_enabled, priority)
      VALUES (?, 1, ?)
    `, [providerKey, maxPriority]);
  }
}

/**
 * Sync providers from code with database.
 * Adds new providers and removes deleted ones.
 */
export function syncProviders(providerKeys: string[]): void {
  // Add any missing providers
  for (const key of providerKeys) {
    ensureProviderExists(key);
  }

  // Remove providers that no longer exist in code
  const existing = getAllProviderSettings();
  for (const setting of existing) {
    if (!providerKeys.includes(setting.provider_key)) {
      db.run("DELETE FROM provider_settings WHERE provider_key = ?", [setting.provider_key]);
    }
  }
}

/**
 * Get enabled provider keys only (for filtering).
 */
export function getEnabledProviderKeys(): string[] {
  return getEnabledProviderSettings().map((s) => s.provider_key);
}

/**
 * Check if a provider is enabled.
 */
export function isProviderEnabled(providerKey: string): boolean {
  const setting = getProviderSetting(providerKey);
  return setting?.is_enabled === 1;
}
