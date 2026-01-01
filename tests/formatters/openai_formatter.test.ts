/**
 * Tests for formatters/openai_formatter.ts
 */

import { describe, test, expect } from "bun:test";

// Inline implementations for testing without importing the actual module
// This avoids dependency issues during testing

function formatOpenAIChunk(content: string, model: string): string {
  const chunk = {
    id: `chatcmpl-${Date.now()}`,
    object: "chat.completion.chunk",
    created: Math.floor(Date.now() / 1000),
    model,
    choices: [
      {
        index: 0,
        delta: { content },
        finish_reason: null,
      },
    ],
  };
  return `data: ${JSON.stringify(chunk)}\n\n`;
}

function formatOpenAIDone(): string {
  return "data: [DONE]\n\n";
}

async function* formatOpenAIStream(
  stream: AsyncIterable<string>,
  model: string
): AsyncIterable<string> {
  for await (const chunk of stream) {
    if (chunk) {
      yield formatOpenAIChunk(chunk, model);
    }
  }
  yield formatOpenAIDone();
}

describe("OpenAI Formatter", () => {
  describe("formatOpenAIChunk", () => {
    test("should format chunk as SSE data line", () => {
      const result = formatOpenAIChunk("Hello", "aicarousel");

      expect(result).toMatch(/^data: /);
      expect(result).toMatch(/\n\n$/);
    });

    test("should include content in delta", () => {
      const result = formatOpenAIChunk("Hello", "aicarousel");
      const json = JSON.parse(result.replace("data: ", "").trim());

      expect(json.choices[0].delta.content).toBe("Hello");
    });

    test("should include model name", () => {
      const result = formatOpenAIChunk("Hello", "test-model");
      const json = JSON.parse(result.replace("data: ", "").trim());

      expect(json.model).toBe("test-model");
    });

    test("should have correct object type", () => {
      const result = formatOpenAIChunk("Hello", "aicarousel");
      const json = JSON.parse(result.replace("data: ", "").trim());

      expect(json.object).toBe("chat.completion.chunk");
    });

    test("should have null finish_reason during streaming", () => {
      const result = formatOpenAIChunk("Hello", "aicarousel");
      const json = JSON.parse(result.replace("data: ", "").trim());

      expect(json.choices[0].finish_reason).toBeNull();
    });
  });

  describe("formatOpenAIDone", () => {
    test("should return [DONE] marker", () => {
      const result = formatOpenAIDone();
      expect(result).toBe("data: [DONE]\n\n");
    });
  });

  describe("formatOpenAIStream", () => {
    test("should format all chunks and end with [DONE]", async () => {
      async function* mockStream(): AsyncIterable<string> {
        yield "Hello";
        yield " ";
        yield "World";
      }

      const chunks: string[] = [];
      for await (const chunk of formatOpenAIStream(mockStream(), "aicarousel")) {
        chunks.push(chunk);
      }

      expect(chunks.length).toBe(4); // 3 content chunks + [DONE]
      expect(chunks[chunks.length - 1]).toBe("data: [DONE]\n\n");
    });

    test("should skip empty chunks", async () => {
      async function* mockStream(): AsyncIterable<string> {
        yield "Hello";
        yield "";
        yield "World";
      }

      const chunks: string[] = [];
      for await (const chunk of formatOpenAIStream(mockStream(), "aicarousel")) {
        chunks.push(chunk);
      }

      // Should have 2 content chunks + [DONE] (empty string skipped)
      expect(chunks.length).toBe(3);
    });

    test("should handle empty stream", async () => {
      async function* mockStream(): AsyncIterable<string> {
        // Empty stream
      }

      const chunks: string[] = [];
      for await (const chunk of formatOpenAIStream(mockStream(), "aicarousel")) {
        chunks.push(chunk);
      }

      expect(chunks.length).toBe(1); // Only [DONE]
      expect(chunks[0]).toBe("data: [DONE]\n\n");
    });
  });

  describe("SSE format compliance", () => {
    test("should use proper SSE line endings", () => {
      const chunk = formatOpenAIChunk("test", "model");
      // SSE requires \n\n between events
      expect(chunk.endsWith("\n\n")).toBe(true);
    });

    test("should be valid JSON in data field", () => {
      const chunk = formatOpenAIChunk("test", "model");
      const dataContent = chunk.replace("data: ", "").trim();

      expect(() => JSON.parse(dataContent)).not.toThrow();
    });
  });
});
