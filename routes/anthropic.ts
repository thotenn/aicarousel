/**
 * Anthropic Messages API routes.
 * Provides /v1/messages endpoint for Claude Code.
 */

import { handleChat } from "@services/chat_handler.ts";
import {
  formatAnthropicStream,
  formatAnthropicComplete,
  formatAnthropicError,
} from "../formatters/anthropic_formatter.ts";
import type { ChatMessage } from "@defaults/types";

export interface AnthropicMessageRequest {
  model: string;
  messages: {
    role: "user" | "assistant";
    content: string | { type: "text"; text: string }[];
  }[];
  system?: string | { type: "text"; text: string }[];
  max_tokens: number;
  stream?: boolean;
  temperature?: number;
  top_p?: number;
  top_k?: number;
  stop_sequences?: string[];
  metadata?: {
    user_id?: string;
  };
}

/**
 * Extract text content from Anthropic message content format.
 * Content can be a string or array of content blocks.
 */
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

/**
 * Convert Anthropic messages to internal ChatMessage format.
 */
function convertMessages(body: AnthropicMessageRequest): ChatMessage[] {
  const messages: ChatMessage[] = [];

  // Add system message if present
  if (body.system) {
    const systemContent =
      typeof body.system === "string"
        ? body.system
        : extractContent(body.system);
    messages.push({
      role: "system",
      content: systemContent,
    });
  }

  // Add conversation messages
  for (const msg of body.messages) {
    messages.push({
      role: msg.role,
      content: extractContent(msg.content),
    });
  }

  return messages;
}

/**
 * POST /v1/messages
 * Anthropic-compatible messages endpoint.
 */
export async function handleMessages(req: Request): Promise<Response> {
  try {
    const body = (await req.json()) as AnthropicMessageRequest;

    // Validate request
    if (!body.messages || !Array.isArray(body.messages)) {
      return Response.json(
        formatAnthropicError("messages is required and must be an array", "invalid_request_error"),
        { status: 400 }
      );
    }

    if (!body.max_tokens) {
      return Response.json(
        formatAnthropicError("max_tokens is required", "invalid_request_error"),
        { status: 400 }
      );
    }

    // Convert to internal format
    const messages = convertMessages(body);
    const shouldStream = body.stream === true; // Default to non-streaming for Anthropic
    const model = body.model || "aicarousel";

    // Get chat stream with retry logic
    const result = await handleChat(messages);

    if (shouldStream) {
      // Streaming response
      const sseStream = formatAnthropicStream(result.stream, model);

      return new Response(
        new ReadableStream({
          async start(controller) {
            const encoder = new TextEncoder();
            try {
              for await (const chunk of sseStream) {
                controller.enqueue(encoder.encode(chunk));
              }
            } catch (error) {
              console.error("Stream error:", error);
            } finally {
              controller.close();
            }
          },
        }),
        {
          headers: {
            "Content-Type": "text/event-stream",
            "Cache-Control": "no-cache",
            Connection: "keep-alive",
            "X-Accel-Buffering": "no",
          },
        }
      );
    } else {
      // Non-streaming response
      const message = await formatAnthropicComplete(result.stream, model);
      return Response.json(message);
    }
  } catch (error) {
    console.error("Messages error:", error);
    const message = error instanceof Error ? error.message : "Internal server error";
    return Response.json(
      formatAnthropicError(message, "api_error"),
      { status: 503 }
    );
  }
}

/**
 * POST /v1/messages/count_tokens
 * Token counting endpoint (stub implementation).
 */
export async function handleCountTokens(req: Request): Promise<Response> {
  try {
    const body = (await req.json()) as AnthropicMessageRequest;

    // Rough estimation: 4 chars per token
    let totalChars = 0;

    if (body.system) {
      const systemContent =
        typeof body.system === "string"
          ? body.system
          : extractContent(body.system);
      totalChars += systemContent.length;
    }

    for (const msg of body.messages) {
      totalChars += extractContent(msg.content).length;
    }

    const inputTokens = Math.ceil(totalChars / 4);

    return Response.json({
      input_tokens: inputTokens,
    });
  } catch (error) {
    console.error("Count tokens error:", error);
    return Response.json(
      formatAnthropicError("Failed to count tokens", "api_error"),
      { status: 500 }
    );
  }
}
