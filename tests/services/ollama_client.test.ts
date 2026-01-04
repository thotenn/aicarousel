/**
 * Tests for OllamaClient
 */

import { describe, test, expect, beforeEach, afterEach, mock, spyOn } from "bun:test";
import { OllamaClient } from "../../services/ollama_client";

describe("OllamaClient", () => {
  let originalEnv: NodeJS.ProcessEnv;

  beforeEach(() => {
    originalEnv = { ...process.env };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  describe("constructor", () => {
    test("should use default base URL when OLLAMA_BASE_URL is not set", () => {
      delete process.env.OLLAMA_BASE_URL;
      const client = new OllamaClient();
      // Access private property for testing
      expect((client as any).baseUrl).toBe("http://localhost:11434");
    });

    test("should use custom base URL when OLLAMA_BASE_URL is set", () => {
      process.env.OLLAMA_BASE_URL = "http://custom:8080";
      const client = new OllamaClient();
      expect((client as any).baseUrl).toBe("http://custom:8080");
    });
  });

  describe("chat.completions.create", () => {
    test("should make correct API call with params", async () => {
      const client = new OllamaClient();
      const mockResponse = {
        ok: true,
        json: async () => ({
          choices: [
            {
              message: { content: "Hello!", role: "assistant" },
            },
          ],
        }),
      };

      const originalFetch = globalThis.fetch;
      globalThis.fetch = mock(() => Promise.resolve(mockResponse as Response));

      try {
        const result = await client.chat.completions.create({
          model: "llama3:8b",
          messages: [{ role: "user", content: "Hi" }],
          stream: false,
          temperature: 0.7,
          top_p: 0.9,
          max_completion_tokens: 1024,
        });

        expect(globalThis.fetch).toHaveBeenCalledTimes(1);
        const fetchCall = (globalThis.fetch as any).mock.calls[0];
        expect(fetchCall[0]).toBe("http://localhost:11434/v1/chat/completions");

        const requestBody = JSON.parse(fetchCall[1].body);
        expect(requestBody.model).toBe("llama3:8b");
        expect(requestBody.messages).toEqual([{ role: "user", content: "Hi" }]);
        expect(requestBody.stream).toBe(false);
        expect(requestBody.temperature).toBe(0.7);
        expect(requestBody.top_p).toBe(0.9);
        expect(requestBody.max_tokens).toBe(1024);

        expect(result.choices[0].message.content).toBe("Hello!");
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    test("should handle non-streaming response", async () => {
      const client = new OllamaClient();
      const mockResponse = {
        ok: true,
        json: async () => ({
          choices: [
            {
              message: { content: "Test response", role: "assistant" },
            },
          ],
        }),
      };

      const originalFetch = globalThis.fetch;
      globalThis.fetch = mock(() => Promise.resolve(mockResponse as Response));

      try {
        const result = await client.chat.completions.create({
          model: "llama3:8b",
          messages: [{ role: "user", content: "Test" }],
          stream: false,
        });

        expect(result.choices[0].message.content).toBe("Test response");
        expect(result.choices[0].message.role).toBe("assistant");
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    test("should throw error on API failure", async () => {
      const client = new OllamaClient();
      const mockResponse = {
        ok: false,
        status: 500,
        text: async () => "Internal Server Error",
      };

      const originalFetch = globalThis.fetch;
      globalThis.fetch = mock(() => Promise.resolve(mockResponse as Response));

      try {
        await expect(
          client.chat.completions.create({
            model: "llama3:8b",
            messages: [{ role: "user", content: "Test" }],
            stream: false,
          })
        ).rejects.toThrow("Ollama API error: 500 - Internal Server Error");
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    test("should handle streaming response", async () => {
      const client = new OllamaClient();

      // Create a mock readable stream
      const chunks = [
        'data: {"choices":[{"delta":{"content":"Hello"}}]}\n\n',
        'data: {"choices":[{"delta":{"content":" world"}}]}\n\n',
        'data: {"choices":[{"delta":{"content":"!"}}]}\n\n',
        'data: [DONE]\n\n',
      ];

      let chunkIndex = 0;
      const mockReader = {
        read: async () => {
          if (chunkIndex < chunks.length) {
            const chunk = chunks[chunkIndex++];
            return {
              done: false,
              value: new TextEncoder().encode(chunk),
            };
          }
          return { done: true, value: undefined };
        },
        releaseLock: () => {},
      };

      const mockResponse = {
        ok: true,
        body: {
          getReader: () => mockReader,
        },
      };

      const originalFetch = globalThis.fetch;
      globalThis.fetch = mock(() => Promise.resolve(mockResponse as unknown as Response));

      try {
        const stream = await client.chat.completions.create({
          model: "llama3:8b",
          messages: [{ role: "user", content: "Test" }],
          stream: true,
        });

        const results: string[] = [];
        for await (const chunk of stream as AsyncIterable<any>) {
          if (chunk.choices[0]?.delta?.content) {
            results.push(chunk.choices[0].delta.content);
          }
        }

        expect(results).toEqual(["Hello", " world", "!"]);
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    test("should handle empty choices in non-streaming response", async () => {
      const client = new OllamaClient();
      const mockResponse = {
        ok: true,
        json: async () => ({
          choices: [],
        }),
      };

      const originalFetch = globalThis.fetch;
      globalThis.fetch = mock(() => Promise.resolve(mockResponse as Response));

      try {
        const result = await client.chat.completions.create({
          model: "llama3:8b",
          messages: [{ role: "user", content: "Test" }],
          stream: false,
        });

        expect(result.choices[0].message.content).toBe("");
      } finally {
        globalThis.fetch = originalFetch;
      }
    });
  });

  describe("streamResponse", () => {
    test("should skip invalid JSON in stream", async () => {
      const client = new OllamaClient();

      const chunks = [
        'data: {"choices":[{"delta":{"content":"Valid"}}]}\n\n',
        'data: invalid-json\n\n',
        'data: {"choices":[{"delta":{"content":" content"}}]}\n\n',
      ];

      let chunkIndex = 0;
      const mockReader = {
        read: async () => {
          if (chunkIndex < chunks.length) {
            const chunk = chunks[chunkIndex++];
            return {
              done: false,
              value: new TextEncoder().encode(chunk),
            };
          }
          return { done: true, value: undefined };
        },
        releaseLock: () => {},
      };

      const mockResponse = {
        body: {
          getReader: () => mockReader,
        },
      } as unknown as Response;

      const results: string[] = [];
      for await (const chunk of client.streamResponse(mockResponse)) {
        if (chunk.choices[0]?.delta?.content) {
          results.push(chunk.choices[0].delta.content);
        }
      }

      expect(results).toEqual(["Valid", " content"]);
    });

    test("should throw error when response body is null", async () => {
      const client = new OllamaClient();
      const mockResponse = {
        body: null,
      } as Response;

      const generator = client.streamResponse(mockResponse);
      await expect(generator.next()).rejects.toThrow("No response body");
    });
  });
});
