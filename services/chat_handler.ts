import { getActiveProviders, createServiceWithModel } from "./ai_controller.ts";
import type { ChatMessage, AIServiceWithModel, ActiveProvider } from "@defaults/types";
import type { ProviderKey } from "@defaults/providers";

// Track current provider index for round-robin
let currentProviderIndex = 0;

export interface ChatResult {
  stream: AsyncIterable<string>;
  serviceName: string;
  model: string;
  providerKey: string;
}

/**
 * Get the count of active providers.
 */
export function getProvidersCount(): number {
  return getActiveProviders().length;
}

/**
 * Try a single model and return the result or null if failed.
 */
async function tryModel(
  service: AIServiceWithModel,
  messages: ChatMessage[]
): Promise<ChatResult | null> {
  try {
    const stream = service.chat(messages) as AsyncIterable<string>;

    // Validate by fetching first chunk
    const iterator = stream[Symbol.asyncIterator]();
    const firstResult = await iterator.next();

    if (firstResult.done) {
      console.error(`${service.name} (${service.model}) returned empty response`);
      return null;
    }

    // Create combined stream with first chunk + rest
    const combinedStream = createCombinedStream(firstResult.value, iterator);

    return {
      stream: combinedStream,
      serviceName: service.name,
      model: service.model,
      providerKey: service.providerKey,
    };
  } catch (error) {
    console.error(`${service.name} (${service.model}) failed:`, error);
    return null;
  }
}

/**
 * Try all models for a provider (with fallback if enabled).
 * Returns result on first success, or null if all models fail.
 */
async function tryProvider(
  provider: ActiveProvider,
  messages: ChatMessage[]
): Promise<{ result: ChatResult | null; lastError: Error | null }> {
  let lastError: Error | null = null;

  // Get models to try: default first, then others if fallback enabled
  const modelsToTry = provider.enableFallback
    ? getOrderedModels(provider.models, provider.defaultModel)
    : [provider.defaultModel];

  for (const model of modelsToTry) {
    console.log(`Using service: ${provider.name} (model: ${model})`);

    try {
      const service = createServiceWithModel(provider.key as ProviderKey, model);
      const result = await tryModel(service, messages);

      if (result) {
        return { result, lastError: null };
      }
    } catch (error) {
      lastError = error instanceof Error ? error : new Error(String(error));
      console.error(`Failed to create service for ${provider.name}/${model}:`, error);
    }

    // If fallback disabled, don't try other models
    if (!provider.enableFallback) {
      break;
    }

    console.log(`Fallback: trying next model for ${provider.name}...`);
  }

  return { result: null, lastError };
}

/**
 * Get models in order: default first, then rest.
 */
function getOrderedModels(models: string[], defaultModel: string): string[] {
  const ordered = [defaultModel];
  for (const model of models) {
    if (model !== defaultModel) {
      ordered.push(model);
    }
  }
  return ordered;
}

/**
 * Handles chat with automatic retry/fallback logic.
 *
 * Fallback order:
 * 1. Try current provider's default model
 * 2. If enableFallback=true, try other models in the provider
 * 3. Move to next provider and repeat
 * 4. Continue until success or all providers exhausted
 */
export async function handleChat(messages: ChatMessage[]): Promise<ChatResult> {
  const providers = getActiveProviders();

  if (providers.length === 0) {
    throw new Error("No AI providers configured. Please configure at least one provider with an API key.");
  }

  // Ensure index is within bounds
  currentProviderIndex = currentProviderIndex % providers.length;

  let lastError: Error | null = null;
  const startIndex = currentProviderIndex;

  // Try each provider in round-robin order
  for (let i = 0; i < providers.length; i++) {
    const providerIndex = (startIndex + i) % providers.length;
    const provider = providers[providerIndex];

    if (!provider) continue;

    const { result, lastError: providerError } = await tryProvider(provider, messages);

    if (result) {
      // Update index to next provider for round-robin
      currentProviderIndex = (providerIndex + 1) % providers.length;
      return result;
    }

    if (providerError) {
      lastError = providerError;
    }

    console.log(`Provider ${provider.name} exhausted, trying next provider...`);
  }

  throw lastError || new Error("All AI services failed");
}

/**
 * Create a combined async iterable from first chunk and remaining iterator.
 */
async function* createCombinedStream(
  firstChunk: string,
  iterator: AsyncIterator<string>
): AsyncIterable<string> {
  yield firstChunk;

  while (true) {
    const { done, value } = await iterator.next();
    if (done) break;
    yield value;
  }
}

/**
 * Reset the provider index (useful for testing).
 */
export function resetProviderIndex(): void {
  currentProviderIndex = 0;
}
