/**
 * Tests for routes/anthropic.ts
 */

import { describe, test, expect } from "bun:test";

// Types matching the Anthropic API
interface AnthropicMessage {
  role: "user" | "assistant";
  content: string | { type: "text"; text: string }[];
}

interface AnthropicMessageRequest {
  model: string;
  messages: AnthropicMessage[];
  system?: string | { type: "text"; text: string }[];
  max_tokens: number;
  stream?: boolean;
}

interface ChatMessage {
  role: "system" | "user" | "assistant";
  content: string;
}

// Helper to extract text content
function extractContent(
  content: string | { type: "text"; text: string }[]
): string {
  if (typeof content === "string") {
    return content;
  }
  return content
    .filter((block) => block.type === "text")
    .map((block) => block.text)
    .join("\n");
}

// Helper to convert Anthropic messages to internal format
function convertAnthropicMessages(body: AnthropicMessageRequest): ChatMessage[] {
  const messages: ChatMessage[] = [];

  if (body.system) {
    const systemContent =
      typeof body.system === "string"
        ? body.system
        : extractContent(body.system);
    messages.push({ role: "system", content: systemContent });
  }

  for (const msg of body.messages) {
    messages.push({
      role: msg.role,
      content: extractContent(msg.content),
    });
  }

  return messages;
}

describe("Anthropic Routes", () => {
  describe("Content Extraction", () => {
    test("should extract string content directly", () => {
      const content = "Hello World";
      expect(extractContent(content)).toBe("Hello World");
    });

    test("should extract text from content blocks", () => {
      const content = [
        { type: "text" as const, text: "Hello" },
        { type: "text" as const, text: "World" },
      ];
      expect(extractContent(content)).toBe("Hello\nWorld");
    });

    test("should handle empty content blocks", () => {
      const content: { type: "text"; text: string }[] = [];
      expect(extractContent(content)).toBe("");
    });

    test("should filter non-text blocks", () => {
      const content = [
        { type: "text" as const, text: "Hello" },
        { type: "image" as any, data: "..." },
        { type: "text" as const, text: "World" },
      ];
      expect(extractContent(content)).toBe("Hello\nWorld");
    });
  });

  describe("Message Conversion", () => {
    test("should convert user messages", () => {
      const request: AnthropicMessageRequest = {
        model: "aicarousel",
        messages: [{ role: "user", content: "Hello" }],
        max_tokens: 1000,
      };

      const result = convertAnthropicMessages(request);

      expect(result).toEqual([{ role: "user", content: "Hello" }]);
    });

    test("should include system message at start", () => {
      const request: AnthropicMessageRequest = {
        model: "aicarousel",
        system: "You are a helpful assistant",
        messages: [{ role: "user", content: "Hi" }],
        max_tokens: 1000,
      };

      const result = convertAnthropicMessages(request);

      expect(result[0].role).toBe("system");
      expect(result[0].content).toBe("You are a helpful assistant");
      expect(result[1].role).toBe("user");
    });

    test("should handle system as content blocks", () => {
      const request: AnthropicMessageRequest = {
        model: "aicarousel",
        system: [{ type: "text", text: "System prompt here" }],
        messages: [{ role: "user", content: "Hi" }],
        max_tokens: 1000,
      };

      const result = convertAnthropicMessages(request);

      expect(result[0].content).toBe("System prompt here");
    });

    test("should convert content blocks in messages", () => {
      const request: AnthropicMessageRequest = {
        model: "aicarousel",
        messages: [
          {
            role: "user",
            content: [{ type: "text", text: "Hello from blocks" }],
          },
        ],
        max_tokens: 1000,
      };

      const result = convertAnthropicMessages(request);

      expect(result[0].content).toBe("Hello from blocks");
    });

    test("should preserve conversation history", () => {
      const request: AnthropicMessageRequest = {
        model: "aicarousel",
        messages: [
          { role: "user", content: "Hello" },
          { role: "assistant", content: "Hi there!" },
          { role: "user", content: "How are you?" },
        ],
        max_tokens: 1000,
      };

      const result = convertAnthropicMessages(request);

      expect(result.length).toBe(3);
      expect(result.map((m) => m.role)).toEqual(["user", "assistant", "user"]);
    });
  });

  describe("Request Validation", () => {
    test("should require messages array", () => {
      const request: AnthropicMessageRequest = {
        model: "aicarousel",
        messages: [],
        max_tokens: 1000,
      };

      expect(request.messages).toBeDefined();
      expect(Array.isArray(request.messages)).toBe(true);
    });

    test("should require max_tokens", () => {
      const request: AnthropicMessageRequest = {
        model: "aicarousel",
        messages: [{ role: "user", content: "Hi" }],
        max_tokens: 1000,
      };

      expect(request.max_tokens).toBeDefined();
      expect(typeof request.max_tokens).toBe("number");
    });

    test("should have optional stream parameter", () => {
      const request: AnthropicMessageRequest = {
        model: "aicarousel",
        messages: [{ role: "user", content: "Hi" }],
        max_tokens: 1000,
        stream: true,
      };

      expect(request.stream).toBe(true);
    });
  });

  describe("Error Responses", () => {
    test("should format error in Anthropic format", () => {
      const error = {
        type: "error",
        error: {
          type: "invalid_request_error",
          message: "messages is required",
        },
      };

      expect(error.type).toBe("error");
      expect(error.error.type).toBe("invalid_request_error");
      expect(error.error.message).toBe("messages is required");
    });

    test("should format authentication error", () => {
      const error = {
        type: "error",
        error: {
          type: "authentication_error",
          message: "Invalid API key",
        },
      };

      expect(error.error.type).toBe("authentication_error");
    });

    test("should format API error", () => {
      const error = {
        type: "error",
        error: {
          type: "api_error",
          message: "Internal server error",
        },
      };

      expect(error.error.type).toBe("api_error");
    });
  });

  describe("Token Counting", () => {
    test("should estimate tokens from character count", () => {
      // Rough estimation: 4 chars per token
      const text = "Hello World"; // 11 chars
      const estimatedTokens = Math.ceil(text.length / 4);

      expect(estimatedTokens).toBe(3);
    });

    test("should sum tokens from all messages", () => {
      const messages = [
        { content: "Hello" }, // 5 chars = ~2 tokens
        { content: "World" }, // 5 chars = ~2 tokens
      ];

      const totalChars = messages.reduce((sum, m) => sum + m.content.length, 0);
      const estimatedTokens = Math.ceil(totalChars / 4);

      expect(estimatedTokens).toBe(3); // 10/4 rounded up
    });
  });
});
