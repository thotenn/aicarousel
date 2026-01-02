/**
 * Models configuration.
 *
 * This file now reads from models.json for dynamic configuration.
 * The static exports are kept for backwards compatibility.
 *
 * @see models.json for the source of truth
 * @see services/models_config.ts for management functions
 */

import { getDefaultModel } from "../services/models_config";

// Re-export all functions from models_config for convenience
export {
  getModelsConfig,
  getModelsConfigSync,
  saveModelsConfig,
  clearConfigCache,
  getProviderConfig,
  getProviderModels,
  getDefaultModel,
  isProviderFallbackEnabled,
  validateModelsConfig,
  addModel,
  removeModel,
  setDefaultModel,
  toggleFallback,
  reorderModels,
  updateModel,
  type ModelsConfig,
  type ProviderModelConfig,
} from "../services/models_config";

/**
 * @deprecated Use getDefaultModel(providerKey) instead.
 * Static model constants kept for backwards compatibility.
 */
export const models = {
  // These are now dynamic - reading from models.json
  get GLM46ZAI() {
    return getDefaultModel("cerebras") ?? "zai-glm-4.6";
  },
  get KIMIK2I0905() {
    return getDefaultModel("groq") ?? "moonshotai/kimi-k2-instruct-0905";
  },
  get QWEN3CODERFREE() {
    return getDefaultModel("openrouter") ?? "qwen/qwen3-coder:free";
  },
  get GEMINI25FLASH() {
    return getDefaultModel("gemini") ?? "gemini-2.5-flash";
  },
};
