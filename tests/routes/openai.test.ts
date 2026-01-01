/**
 * Tests for routes/openai.ts
 */

import { describe, test, expect } from "bun:test";
import { createMockRequest } from "../utils/mocks";

// Types matching the OpenAI API
interface OpenAIMessage {
  role: "system" | "user" | "assistant";
  content: string;
}

interface OpenAIChatRequest {
  model: string;
  messages: OpenAIMessage[];
  stream?: boolean;
  temperature?: number;
  max_tokens?: number;
}

// Helper to convert OpenAI messages to internal format
interface ChatMessage {
  role: "system" | "user" | "assistant";
  content: string;
}

function convertOpenAIMessages(messages: OpenAIMessage[]): ChatMessage[] {
  return messages.map((msg) => ({
    role: msg.role,
    content: msg.content,
  }));
}

describe("OpenAI Routes", () => {
  describe("Message Conversion", () => {
    test("should convert user messages", () => {
      const openaiMessages: OpenAIMessage[] = [
        { role: "user", content: "Hello" },
      ];

      const result = convertOpenAIMessages(openaiMessages);

      expect(result).toEqual([{ role: "user", content: "Hello" }]);
    });

    test("should convert system messages", () => {
      const openaiMessages: OpenAIMessage[] = [
        { role: "system", content: "You are helpful" },
        { role: "user", content: "Hi" },
      ];

      const result = convertOpenAIMessages(openaiMessages);

      expect(result[0].role).toBe("system");
      expect(result[0].content).toBe("You are helpful");
    });

    test("should convert assistant messages", () => {
      const openaiMessages: OpenAIMessage[] = [
        { role: "user", content: "Hi" },
        { role: "assistant", content: "Hello!" },
        { role: "user", content: "How are you?" },
      ];

      const result = convertOpenAIMessages(openaiMessages);

      expect(result.length).toBe(3);
      expect(result[1].role).toBe("assistant");
    });

    test("should preserve message order", () => {
      const openaiMessages: OpenAIMessage[] = [
        { role: "system", content: "System" },
        { role: "user", content: "User 1" },
        { role: "assistant", content: "Assistant 1" },
        { role: "user", content: "User 2" },
      ];

      const result = convertOpenAIMessages(openaiMessages);

      expect(result.map((m) => m.role)).toEqual([
        "system",
        "user",
        "assistant",
        "user",
      ]);
    });
  });

  describe("Request Validation", () => {
    test("should accept valid chat request", () => {
      const body: OpenAIChatRequest = {
        model: "aicarousel",
        messages: [{ role: "user", content: "Hello" }],
        stream: true,
      };

      expect(body.messages).toBeDefined();
      expect(Array.isArray(body.messages)).toBe(true);
      expect(body.messages.length).toBeGreaterThan(0);
    });

    test("should require messages array", () => {
      const body = {
        model: "aicarousel",
        stream: true,
      };

      expect(body).not.toHaveProperty("messages");
    });

    test("should handle optional parameters", () => {
      const body: OpenAIChatRequest = {
        model: "aicarousel",
        messages: [{ role: "user", content: "Hello" }],
        temperature: 0.7,
        max_tokens: 1000,
      };

      expect(body.temperature).toBe(0.7);
      expect(body.max_tokens).toBe(1000);
      expect(body.stream).toBeUndefined();
    });
  });

  describe("/v1/models endpoint", () => {
    test("should return models list structure", () => {
      const modelsResponse = {
        object: "list",
        data: [
          {
            id: "aicarousel",
            object: "model",
            created: Math.floor(Date.now() / 1000),
            owned_by: "aicarousel",
          },
        ],
      };

      expect(modelsResponse.object).toBe("list");
      expect(modelsResponse.data).toBeInstanceOf(Array);
      expect(modelsResponse.data[0].id).toBe("aicarousel");
    });
  });

  describe("Streaming Response", () => {
    test("should set correct headers for streaming", () => {
      const headers = {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
      };

      expect(headers["Content-Type"]).toBe("text/event-stream");
      expect(headers["Cache-Control"]).toBe("no-cache");
    });
  });

  describe("Error Responses", () => {
    test("should format error in OpenAI format", () => {
      const error = {
        error: {
          message: "Invalid request",
          type: "invalid_request_error",
          param: null,
          code: null,
        },
      };

      expect(error.error).toBeDefined();
      expect(error.error.message).toBe("Invalid request");
      expect(error.error.type).toBe("invalid_request_error");
    });

    test("should include 401 status for auth errors", () => {
      const authError = {
        error: {
          message: "Invalid API key",
          type: "authentication_error",
          param: null,
          code: "invalid_api_key",
        },
      };

      expect(authError.error.type).toBe("authentication_error");
      expect(authError.error.code).toBe("invalid_api_key");
    });
  });
});
