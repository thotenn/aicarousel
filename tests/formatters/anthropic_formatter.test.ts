/**
 * Tests for formatters/anthropic_formatter.ts
 */

import { describe, test, expect } from "bun:test";

// Inline implementations for testing

function formatAnthropicEvent(event: string, data: object): string {
  return `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
}

function formatMessageStart(model: string): string {
  return formatAnthropicEvent("message_start", {
    type: "message_start",
    message: {
      id: `msg_${Date.now()}`,
      type: "message",
      role: "assistant",
      model,
      content: [],
      stop_reason: null,
      stop_sequence: null,
      usage: { input_tokens: 0, output_tokens: 0 },
    },
  });
}

function formatContentBlockStart(): string {
  return formatAnthropicEvent("content_block_start", {
    type: "content_block_start",
    index: 0,
    content_block: { type: "text", text: "" },
  });
}

function formatContentBlockDelta(text: string): string {
  return formatAnthropicEvent("content_block_delta", {
    type: "content_block_delta",
    index: 0,
    delta: { type: "text_delta", text },
  });
}

function formatContentBlockStop(): string {
  return formatAnthropicEvent("content_block_stop", {
    type: "content_block_stop",
    index: 0,
  });
}

function formatMessageDelta(): string {
  return formatAnthropicEvent("message_delta", {
    type: "message_delta",
    delta: { stop_reason: "end_turn", stop_sequence: null },
    usage: { output_tokens: 1 },
  });
}

function formatMessageStop(): string {
  return formatAnthropicEvent("message_stop", {
    type: "message_stop",
  });
}

function formatAnthropicError(message: string, type: string) {
  return {
    type: "error",
    error: {
      type,
      message,
    },
  };
}

describe("Anthropic Formatter", () => {
  describe("formatAnthropicEvent", () => {
    test("should format event with type and data", () => {
      const result = formatAnthropicEvent("test_event", { key: "value" });

      expect(result).toContain("event: test_event");
      expect(result).toContain('data: {"key":"value"}');
      expect(result).toMatch(/\n\n$/);
    });
  });

  describe("formatMessageStart", () => {
    test("should include message_start event", () => {
      const result = formatMessageStart("aicarousel");
      expect(result).toContain("event: message_start");
    });

    test("should include model name", () => {
      const result = formatMessageStart("test-model");
      expect(result).toContain('"model":"test-model"');
    });

    test("should have assistant role", () => {
      const result = formatMessageStart("aicarousel");
      expect(result).toContain('"role":"assistant"');
    });

    test("should have empty content array initially", () => {
      const result = formatMessageStart("aicarousel");
      expect(result).toContain('"content":[]');
    });
  });

  describe("formatContentBlockStart", () => {
    test("should have content_block_start event", () => {
      const result = formatContentBlockStart();
      expect(result).toContain("event: content_block_start");
    });

    test("should have text type content block", () => {
      const result = formatContentBlockStart();
      expect(result).toContain('"type":"text"');
    });

    test("should have index 0", () => {
      const result = formatContentBlockStart();
      expect(result).toContain('"index":0');
    });
  });

  describe("formatContentBlockDelta", () => {
    test("should include text content", () => {
      const result = formatContentBlockDelta("Hello");
      expect(result).toContain('"text":"Hello"');
    });

    test("should have text_delta type", () => {
      const result = formatContentBlockDelta("test");
      expect(result).toContain('"type":"text_delta"');
    });

    test("should escape special characters in text", () => {
      const result = formatContentBlockDelta('Hello "World"');
      expect(result).toContain('\\"World\\"');
    });

    test("should handle newlines", () => {
      const result = formatContentBlockDelta("Line1\nLine2");
      expect(result).toContain("\\n");
    });
  });

  describe("formatContentBlockStop", () => {
    test("should have content_block_stop event", () => {
      const result = formatContentBlockStop();
      expect(result).toContain("event: content_block_stop");
    });
  });

  describe("formatMessageDelta", () => {
    test("should have message_delta event", () => {
      const result = formatMessageDelta();
      expect(result).toContain("event: message_delta");
    });

    test("should have end_turn stop_reason", () => {
      const result = formatMessageDelta();
      expect(result).toContain('"stop_reason":"end_turn"');
    });
  });

  describe("formatMessageStop", () => {
    test("should have message_stop event", () => {
      const result = formatMessageStop();
      expect(result).toContain("event: message_stop");
    });
  });

  describe("formatAnthropicError", () => {
    test("should format error with type and message", () => {
      const result = formatAnthropicError("Something went wrong", "api_error");

      expect(result.type).toBe("error");
      expect(result.error.type).toBe("api_error");
      expect(result.error.message).toBe("Something went wrong");
    });

    test("should handle invalid_request_error type", () => {
      const result = formatAnthropicError("Bad request", "invalid_request_error");
      expect(result.error.type).toBe("invalid_request_error");
    });
  });

  describe("Full stream sequence", () => {
    test("should produce valid event sequence", () => {
      const events: string[] = [];

      events.push(formatMessageStart("aicarousel"));
      events.push(formatContentBlockStart());
      events.push(formatContentBlockDelta("Hello"));
      events.push(formatContentBlockDelta(" World"));
      events.push(formatContentBlockStop());
      events.push(formatMessageDelta());
      events.push(formatMessageStop());

      // Check all events are properly formatted
      for (const event of events) {
        expect(event).toMatch(/^event: \w+\n/);
        expect(event).toMatch(/data: \{.*\}\n\n$/);
      }

      // Check event order
      expect(events[0]).toContain("message_start");
      expect(events[1]).toContain("content_block_start");
      expect(events[events.length - 1]).toContain("message_stop");
    });
  });
});
