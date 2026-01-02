/**
 * Test mocks and utilities for AICarousel tests.
 */

import type { AIService, AIServiceWithModel, ActiveProvider, ChatMessage } from "../../defaults/types";
import type { ModelsConfig, ProviderModelConfig } from "../../services/models_config";

/**
 * Create a mock AI service that returns predefined responses.
 */
export function createMockService(
  name: string,
  responses: string[] = ["Hello", " world", "!"]
): AIService {
  return {
    name,
    async *chat(_messages: ChatMessage[]) {
      for (const chunk of responses) {
        yield chunk;
      }
    },
  };
}

/**
 * Create a mock AI service that fails with an error.
 */
export function createFailingService(name: string, error: Error): AIService {
  return {
    name,
    async *chat(_messages: ChatMessage[]) {
      throw error;
    },
  };
}

/**
 * Create a mock AI service that returns empty response.
 */
export function createEmptyService(name: string): AIService {
  return {
    name,
    async *chat(_messages: ChatMessage[]) {
      // Empty generator - returns done immediately
    },
  };
}

/**
 * Create a mock AI service that simulates rate limiting.
 */
export function createRateLimitedService(name: string): AIService {
  const error = new Error("429 Too Many Requests");
  (error as any).status = 429;
  return createFailingService(name, error);
}

/**
 * Sample chat messages for testing.
 */
export const sampleMessages: ChatMessage[] = [
  { role: "system", content: "You are a helpful assistant." },
  { role: "user", content: "Hello!" },
];

/**
 * Sample API key for testing.
 */
export const testApiKey = "sk-test-1234567890abcdef";

/**
 * Collect all chunks from an async iterable into a single string.
 */
export async function collectStream(
  stream: AsyncIterable<string>
): Promise<string> {
  let result = "";
  for await (const chunk of stream) {
    result += chunk;
  }
  return result;
}

/**
 * Create a mock request with JSON body.
 */
export function createMockRequest(
  body: object,
  headers: Record<string, string> = {}
): Request {
  return new Request("http://localhost:7123/test", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...headers,
    },
    body: JSON.stringify(body),
  });
}

/**
 * Create a mock request with API key authentication.
 */
export function createAuthenticatedRequest(
  body: object,
  apiKey: string = testApiKey
): Request {
  return createMockRequest(body, {
    Authorization: `Bearer ${apiKey}`,
  });
}

// ============================================
// Model configuration mocks
// ============================================

/**
 * Create a mock AI service with model information.
 */
export function createMockServiceWithModel(
  name: string,
  providerKey: string,
  model: string,
  responses: string[] = ["Hello", " world", "!"]
): AIServiceWithModel {
  return {
    name,
    providerKey,
    model,
    async *chat(_messages: ChatMessage[]) {
      for (const chunk of responses) {
        yield chunk;
      }
    },
  };
}

/**
 * Create a failing mock service with model information.
 */
export function createFailingServiceWithModel(
  name: string,
  providerKey: string,
  model: string,
  error: Error
): AIServiceWithModel {
  return {
    name,
    providerKey,
    model,
    async *chat(_messages: ChatMessage[]) {
      throw error;
    },
  };
}

/**
 * Sample models configuration for testing.
 */
export const sampleModelsConfig: ModelsConfig = {
  testProvider1: {
    default: "model-a",
    enableFallback: true,
    models: ["model-a", "model-b", "model-c"],
  },
  testProvider2: {
    default: "model-x",
    enableFallback: false,
    models: ["model-x", "model-y"],
  },
  testProvider3: {
    default: "model-only",
    enableFallback: true,
    models: ["model-only"],
  },
};

/**
 * Sample active providers for testing.
 */
export const sampleActiveProviders: ActiveProvider[] = [
  {
    key: "testProvider1",
    name: "Test Provider 1",
    models: ["model-a", "model-b", "model-c"],
    defaultModel: "model-a",
    enableFallback: true,
    priority: 1,
  },
  {
    key: "testProvider2",
    name: "Test Provider 2",
    models: ["model-x", "model-y"],
    defaultModel: "model-x",
    enableFallback: false,
    priority: 2,
  },
];

/**
 * Create a sample provider model config.
 */
export function createProviderModelConfig(
  defaultModel: string,
  models: string[],
  enableFallback: boolean = true
): ProviderModelConfig {
  return {
    default: defaultModel,
    enableFallback,
    models,
  };
}

/**
 * Create a sample active provider.
 */
export function createActiveProvider(
  key: string,
  name: string,
  models: string[],
  defaultModel: string,
  enableFallback: boolean = true,
  priority: number = 1
): ActiveProvider {
  return {
    key,
    name,
    models,
    defaultModel,
    enableFallback,
    priority,
  };
}
