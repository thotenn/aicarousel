/**
 * Test mocks and utilities for AICarousel tests.
 */

import type { AIService, ChatMessage } from "../../defaults/types";

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
