/**
 * Tests for zai_client.ts
 */

import { describe, test, expect, beforeEach, afterEach, mock } from "bun:test";
import { ZaiClient } from "../../services/zai_client";

/**
 * Create a mock SSE stream from Anthropic-format events.
 */
function createAnthropicSSE(textChunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  const events = [
    `event: message_start\ndata: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[]}}\n\n`,
    `event: content_block_start\ndata: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}\n\n`,
    ...textChunks.map(
      (text) =>
        `event: content_block_delta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":${JSON.stringify(text)}}}\n\n`
    ),
    `event: message_delta\ndata: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}\n\n`,
    `event: message_stop\ndata: {"type":"message_stop"}\n\n`,
  ];

  return new ReadableStream({
    start(controller) {
      for (const event of events) {
        controller.enqueue(encoder.encode(event));
      }
      controller.close();
    },
  });
}

describe("ZaiClient", () => {
  let originalFetch: typeof globalThis.fetch;
  let originalApiKey: string | undefined;
  let originalBaseUrl: string | undefined;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
    originalApiKey = process.env.ZAI_API_KEY;
    originalBaseUrl = process.env.ZAI_BASE_URL;
    process.env.ZAI_API_KEY = "test-zai-key";
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    if (originalApiKey === undefined) delete process.env.ZAI_API_KEY;
    else process.env.ZAI_API_KEY = originalApiKey;
    if (originalBaseUrl === undefined) delete process.env.ZAI_BASE_URL;
    else process.env.ZAI_BASE_URL = originalBaseUrl;
  });

  test("should send correct headers and body format", async () => {
    let capturedRequest: { url: string; headers: Headers; body: any } | null = null;

    globalThis.fetch = mock(async (url: any, init: any) => {
      capturedRequest = {
        url: url.toString(),
        headers: new Headers(init.headers),
        body: JSON.parse(init.body),
      };
      return new Response(JSON.stringify({
        content: [{ type: "text", text: "Hello" }],
      }), { status: 200 });
    }) as any;

    const client = new ZaiClient();
    await client.chat.completions.create({
      model: "glm-4.5-0111",
      messages: [{ role: "user", content: "Hi" }],
      stream: false,
      max_completion_tokens: 1024,
      temperature: 0.7,
    });

    expect(capturedRequest).not.toBeNull();
    expect(capturedRequest!.url).toBe("https://api.z.ai/api/anthropic/v1/messages");
    expect(capturedRequest!.headers.get("x-api-key")).toBe("test-zai-key");
    expect(capturedRequest!.headers.get("anthropic-version")).toBe("2023-06-01");
    expect(capturedRequest!.body.model).toBe("glm-4.5-0111");
    expect(capturedRequest!.body.max_tokens).toBe(1024);
    expect(capturedRequest!.body.temperature).toBe(0.7);
    expect(capturedRequest!.body.stream).toBe(false);
  });

  test("should extract system messages to top-level field", async () => {
    let capturedBody: any = null;

    globalThis.fetch = mock(async (_url: any, init: any) => {
      capturedBody = JSON.parse(init.body);
      return new Response(JSON.stringify({
        content: [{ type: "text", text: "OK" }],
      }), { status: 200 });
    }) as any;

    const client = new ZaiClient();
    await client.chat.completions.create({
      model: "glm-4.5-0111",
      messages: [
        { role: "system", content: "You are helpful." },
        { role: "user", content: "Hi" },
      ],
      stream: false,
    });

    expect(capturedBody.system).toBe("You are helpful.");
    expect(capturedBody.messages).toEqual([{ role: "user", content: "Hi" }]);
  });

  test("should not include system field when no system messages", async () => {
    let capturedBody: any = null;

    globalThis.fetch = mock(async (_url: any, init: any) => {
      capturedBody = JSON.parse(init.body);
      return new Response(JSON.stringify({
        content: [{ type: "text", text: "OK" }],
      }), { status: 200 });
    }) as any;

    const client = new ZaiClient();
    await client.chat.completions.create({
      model: "glm-4.5-0111",
      messages: [{ role: "user", content: "Hi" }],
      stream: false,
    });

    expect(capturedBody.system).toBeUndefined();
  });

  test("should handle non-streaming response", async () => {
    globalThis.fetch = mock(async () => {
      return new Response(JSON.stringify({
        content: [
          { type: "text", text: "Hello " },
          { type: "text", text: "world!" },
        ],
      }), { status: 200 });
    }) as any;

    const client = new ZaiClient();
    const result = await client.chat.completions.create({
      model: "glm-4.5-0111",
      messages: [{ role: "user", content: "Hi" }],
      stream: false,
    });

    expect(result.choices[0].message.content).toBe("Hello world!");
  });

  test("should stream Anthropic SSE and yield OpenAI-format chunks", async () => {
    globalThis.fetch = mock(async () => {
      return new Response(createAnthropicSSE(["Hello", " world", "!"]), {
        status: 200,
        headers: { "Content-Type": "text/event-stream" },
      });
    }) as any;

    const client = new ZaiClient();
    const stream = await client.chat.completions.create({
      model: "glm-4.5-0111",
      messages: [{ role: "user", content: "Hi" }],
      stream: true,
    });

    const chunks: string[] = [];
    for await (const chunk of stream as any) {
      const content = chunk.choices[0]?.delta?.content;
      if (content) chunks.push(content);
    }

    expect(chunks).toEqual(["Hello", " world", "!"]);
  });

  test("should throw on API error", async () => {
    globalThis.fetch = mock(async () => {
      return new Response("Unauthorized", { status: 401 });
    }) as any;

    const client = new ZaiClient();

    expect(
      client.chat.completions.create({
        model: "glm-4.5-0111",
        messages: [{ role: "user", content: "Hi" }],
        stream: false,
      })
    ).rejects.toThrow("Z.ai API error: 401");
  });

  test("should use custom base URL from env", async () => {
    process.env.ZAI_BASE_URL = "https://custom.z.ai/api";
    let capturedUrl = "";

    globalThis.fetch = mock(async (url: any) => {
      capturedUrl = url.toString();
      return new Response(JSON.stringify({
        content: [{ type: "text", text: "OK" }],
      }), { status: 200 });
    }) as any;

    const client = new ZaiClient();
    await client.chat.completions.create({
      model: "glm-4.5-0111",
      messages: [{ role: "user", content: "Hi" }],
      stream: false,
    });

    expect(capturedUrl).toBe("https://custom.z.ai/api/v1/messages");
  });
});
