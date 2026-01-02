/**
 * Models configuration service.
 * Reads and writes models.json for dynamic model management.
 */

import { join } from "path";

const MODELS_FILE = join(import.meta.dir, "..", "models.json");

/**
 * Configuration for a single provider's models.
 */
export interface ProviderModelConfig {
  default: string;
  enableFallback: boolean;
  models: string[];
}

/**
 * Full models configuration object.
 */
export interface ModelsConfig {
  [providerKey: string]: ProviderModelConfig;
}

/**
 * Validation error for models config.
 */
export class ModelsConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ModelsConfigError";
  }
}

// Cache for models config to avoid reading file on every call
let configCache: ModelsConfig | null = null;
let configCacheTime = 0;
const CACHE_TTL_MS = 1000; // 1 second cache

/**
 * Clear the config cache.
 * Call this after saving changes.
 */
export function clearConfigCache(): void {
  configCache = null;
  configCacheTime = 0;
}

/**
 * Read and parse models.json synchronously.
 * Uses a short-lived cache to avoid file reads on every call.
 */
export function getModelsConfig(): ModelsConfig {
  const now = Date.now();

  // Return cached if valid
  if (configCache && now - configCacheTime < CACHE_TTL_MS) {
    return configCache;
  }

  try {
    // Use require for synchronous read (Bun caches this)
    // Delete from cache first to get fresh content
    delete require.cache[MODELS_FILE];
    const content = require(MODELS_FILE) as ModelsConfig;
    configCache = content;
    configCacheTime = now;
    return content;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "MODULE_NOT_FOUND") {
      throw new ModelsConfigError(`models.json not found at ${MODELS_FILE}`);
    }
    throw new ModelsConfigError(`Failed to read models.json: ${error}`);
  }
}

/**
 * Alias for getModelsConfig for backwards compatibility.
 */
export function getModelsConfigSync(): ModelsConfig {
  return getModelsConfig();
}

/**
 * Save models configuration to models.json.
 */
export async function saveModelsConfig(config: ModelsConfig): Promise<void> {
  // Validate before saving
  validateModelsConfig(config);

  const content = JSON.stringify(config, null, 2) + "\n";
  await Bun.write(MODELS_FILE, content);

  // Clear cache to pick up new values
  clearConfigCache();
}

/**
 * Get models configuration for a specific provider.
 */
export function getProviderConfig(providerKey: string): ProviderModelConfig | null {
  const config = getModelsConfigSync();
  return config[providerKey] ?? null;
}

/**
 * Get all models for a specific provider.
 */
export function getProviderModels(providerKey: string): string[] {
  const providerConfig = getProviderConfig(providerKey);
  return providerConfig?.models ?? [];
}

/**
 * Get the default model for a specific provider.
 */
export function getDefaultModel(providerKey: string): string | null {
  const providerConfig = getProviderConfig(providerKey);
  return providerConfig?.default ?? null;
}

/**
 * Check if fallback is enabled for a provider.
 */
export function isProviderFallbackEnabled(providerKey: string): boolean {
  const providerConfig = getProviderConfig(providerKey);
  return providerConfig?.enableFallback ?? false;
}

/**
 * Validate the entire models configuration.
 * Throws ModelsConfigError if validation fails.
 */
export function validateModelsConfig(config: ModelsConfig): void {
  if (!config || typeof config !== "object") {
    throw new ModelsConfigError("Config must be an object");
  }

  const providers = Object.keys(config);
  if (providers.length === 0) {
    throw new ModelsConfigError("Config must have at least one provider");
  }

  for (const [providerKey, providerConfig] of Object.entries(config)) {
    validateProviderConfig(providerKey, providerConfig);
  }
}

/**
 * Validate a single provider's configuration.
 */
export function validateProviderConfig(providerKey: string, config: ProviderModelConfig): void {
  if (!config || typeof config !== "object") {
    throw new ModelsConfigError(`Provider "${providerKey}": config must be an object`);
  }

  // Check required fields
  if (typeof config.default !== "string" || config.default.trim() === "") {
    throw new ModelsConfigError(`Provider "${providerKey}": "default" must be a non-empty string`);
  }

  if (typeof config.enableFallback !== "boolean") {
    throw new ModelsConfigError(`Provider "${providerKey}": "enableFallback" must be a boolean`);
  }

  if (!Array.isArray(config.models)) {
    throw new ModelsConfigError(`Provider "${providerKey}": "models" must be an array`);
  }

  if (config.models.length === 0) {
    throw new ModelsConfigError(`Provider "${providerKey}": "models" must have at least one model`);
  }

  // Check all models are strings
  for (const model of config.models) {
    if (typeof model !== "string" || model.trim() === "") {
      throw new ModelsConfigError(`Provider "${providerKey}": all models must be non-empty strings`);
    }
  }

  // Check default is in models list
  if (!config.models.includes(config.default)) {
    throw new ModelsConfigError(
      `Provider "${providerKey}": default model "${config.default}" must be in models list`
    );
  }
}

/**
 * Add a model to a provider.
 */
export async function addModel(providerKey: string, model: string): Promise<void> {
  const config = getModelsConfigSync();

  if (!config[providerKey]) {
    throw new ModelsConfigError(`Provider "${providerKey}" not found`);
  }

  if (config[providerKey].models.includes(model)) {
    throw new ModelsConfigError(`Model "${model}" already exists for provider "${providerKey}"`);
  }

  config[providerKey].models.push(model);
  await saveModelsConfig(config);
}

/**
 * Remove a model from a provider.
 */
export async function removeModel(providerKey: string, model: string): Promise<void> {
  const config = getModelsConfigSync();

  if (!config[providerKey]) {
    throw new ModelsConfigError(`Provider "${providerKey}" not found`);
  }

  const models = config[providerKey].models;
  const index = models.indexOf(model);

  if (index === -1) {
    throw new ModelsConfigError(`Model "${model}" not found for provider "${providerKey}"`);
  }

  if (models.length === 1) {
    throw new ModelsConfigError(`Cannot remove the only model for provider "${providerKey}"`);
  }

  if (config[providerKey].default === model) {
    throw new ModelsConfigError(
      `Cannot remove default model "${model}". Set a different default first.`
    );
  }

  models.splice(index, 1);
  await saveModelsConfig(config);
}

/**
 * Set the default model for a provider.
 */
export async function setDefaultModel(providerKey: string, model: string): Promise<void> {
  const config = getModelsConfigSync();

  if (!config[providerKey]) {
    throw new ModelsConfigError(`Provider "${providerKey}" not found`);
  }

  if (!config[providerKey].models.includes(model)) {
    throw new ModelsConfigError(
      `Model "${model}" not in models list for provider "${providerKey}"`
    );
  }

  config[providerKey].default = model;
  await saveModelsConfig(config);
}

/**
 * Toggle fallback for a provider.
 */
export async function toggleFallback(providerKey: string, enabled?: boolean): Promise<boolean> {
  const config = getModelsConfigSync();

  if (!config[providerKey]) {
    throw new ModelsConfigError(`Provider "${providerKey}" not found`);
  }

  const newValue = enabled ?? !config[providerKey].enableFallback;
  config[providerKey].enableFallback = newValue;
  await saveModelsConfig(config);

  return newValue;
}

/**
 * Reorder models for a provider.
 * The first model in the new order becomes the fallback priority.
 */
export async function reorderModels(providerKey: string, newOrder: string[]): Promise<void> {
  const config = getModelsConfigSync();

  if (!config[providerKey]) {
    throw new ModelsConfigError(`Provider "${providerKey}" not found`);
  }

  const currentModels = config[providerKey].models;

  // Validate new order contains same models
  if (newOrder.length !== currentModels.length) {
    throw new ModelsConfigError("New order must contain same number of models");
  }

  for (const model of newOrder) {
    if (!currentModels.includes(model)) {
      throw new ModelsConfigError(`Model "${model}" not found in current models`);
    }
  }

  config[providerKey].models = newOrder;
  await saveModelsConfig(config);
}

/**
 * Update a model name (rename).
 */
export async function updateModel(
  providerKey: string,
  oldModel: string,
  newModel: string
): Promise<void> {
  const config = getModelsConfigSync();

  if (!config[providerKey]) {
    throw new ModelsConfigError(`Provider "${providerKey}" not found`);
  }

  const models = config[providerKey].models;
  const index = models.indexOf(oldModel);

  if (index === -1) {
    throw new ModelsConfigError(`Model "${oldModel}" not found for provider "${providerKey}"`);
  }

  if (models.includes(newModel)) {
    throw new ModelsConfigError(`Model "${newModel}" already exists for provider "${providerKey}"`);
  }

  models[index] = newModel;

  // Update default if it was the renamed model
  if (config[providerKey].default === oldModel) {
    config[providerKey].default = newModel;
  }

  await saveModelsConfig(config);
}

/**
 * Ensure a provider exists in the config.
 * Creates it with defaults if missing.
 */
export async function ensureProvider(providerKey: string, defaultModel: string): Promise<void> {
  const config = getModelsConfigSync();

  if (!config[providerKey]) {
    config[providerKey] = {
      default: defaultModel,
      enableFallback: true,
      models: [defaultModel],
    };
    await saveModelsConfig(config);
  }
}

/**
 * Get all provider keys from config.
 */
export function getConfiguredProviders(): string[] {
  const config = getModelsConfigSync();
  return Object.keys(config);
}
