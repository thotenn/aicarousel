/**
 * Provider configurations.
 *
 * Models are now loaded dynamically from models.json.
 * Use getProviderParams() to get params with the current default model.
 */

import { getDefaultModel } from "./models";

const defaults = {
  max_completion_tokens: 4096,
  stream: true,
  temperature: 0.6,
  top_p: 1,
};

/**
 * Base provider params without model (model is added dynamically).
 */
const providerBaseParams = {
  cerebras: {
    max_completion_tokens: defaults.max_completion_tokens,
    stream: defaults.stream,
    temperature: defaults.temperature,
    top_p: defaults.top_p,
  },
  groq: {
    max_completion_tokens: defaults.max_completion_tokens,
    stream: defaults.stream,
    stop: null,
    temperature: defaults.temperature,
    top_p: defaults.top_p,
  },
  openrouter: {
    stream: defaults.stream,
  },
  gemini: {
    stream: defaults.stream,
  },
} as const;

export type ProviderKey = keyof typeof providerBaseParams;

/**
 * Provider definitions.
 */
export const providers: Record<ProviderKey, {
  name: string;
  apiKeyName: string;
}> = {
  cerebras: {
    name: "Cerebras",
    apiKeyName: "CEREBRAS_API_KEY",
  },
  groq: {
    name: "Groq",
    apiKeyName: "GROQ_API_KEY",
  },
  openrouter: {
    name: "OpenRouter",
    apiKeyName: "OPENROUTER_API_KEY",
  },
  gemini: {
    name: "Gemini",
    apiKeyName: "GEMINI_API_KEY",
  },
};

/**
 * Get provider params with the specified model (or default model from config).
 */
export function getProviderParams(providerKey: ProviderKey, model?: string): Record<string, any> {
  const baseParams = providerBaseParams[providerKey];
  const modelToUse = model ?? getDefaultModel(providerKey);

  if (!modelToUse) {
    throw new Error(`No model configured for provider: ${providerKey}`);
  }

  return {
    model: modelToUse,
    ...baseParams,
  };
}

/**
 * Get all provider keys.
 */
export function getProviderKeys(): ProviderKey[] {
  return Object.keys(providers) as ProviderKey[];
}
