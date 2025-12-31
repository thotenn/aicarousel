/**
 * OpenAI-compatible API routes.
 * Provides /v1/chat/completions and /v1/models endpoints for Cline, Codex, etc.
 */

import { handleChat } from "@services/chat_handler.ts";
import {
  formatOpenAIStream,
  formatOpenAIComplete,
  formatOpenAIError,
} from "../formatters/openai_formatter.ts";
import type { ChatMessage } from "@defaults/types";

export interface OpenAIChatRequest {
  model?: string;
  messages: {
    role: "system" | "user" | "assistant";
    content: string;
  }[];
  stream?: boolean;
  temperature?: number;
  max_tokens?: number;
  top_p?: number;
  frequency_penalty?: number;
  presence_penalty?: number;
  stop?: string | string[];
}

/**
 * POST /v1/chat/completions
 * OpenAI-compatible chat completions endpoint.
 */
export async function handleChatCompletions(req: Request): Promise<Response> {
  try {
    const body = (await req.json()) as OpenAIChatRequest;

    // Validate request
    if (!body.messages || !Array.isArray(body.messages)) {
      return Response.json(
        formatOpenAIError("messages is required and must be an array", "invalid_request_error"),
        { status: 400 }
      );
    }

    // Convert to internal format
    const messages: ChatMessage[] = body.messages.map((m) => ({
      role: m.role,
      content: m.content,
    }));

    const shouldStream = body.stream !== false; // Default to streaming
    const model = body.model || "aicarousel";

    // Get chat stream with retry logic
    const result = await handleChat(messages);

    if (shouldStream) {
      // Streaming response
      const sseStream = formatOpenAIStream(result.stream, model);

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
            "X-Accel-Buffering": "no", // Disable nginx buffering
          },
        }
      );
    } else {
      // Non-streaming response
      const completion = await formatOpenAIComplete(result.stream, model);
      return Response.json(completion);
    }
  } catch (error) {
    console.error("Chat completions error:", error);
    const message = error instanceof Error ? error.message : "Internal server error";
    return Response.json(
      formatOpenAIError(message, "server_error"),
      { status: 503 }
    );
  }
}

/**
 * GET /v1/models
 * Returns available models list.
 */
export function handleModels(): Response {
  const models = [
    {
      id: "aicarousel",
      object: "model",
      created: Math.floor(Date.now() / 1000),
      owned_by: "aicarousel",
      permission: [],
      root: "aicarousel",
      parent: null,
    },
    {
      id: "gpt-4",
      object: "model",
      created: Math.floor(Date.now() / 1000),
      owned_by: "aicarousel",
      permission: [],
      root: "gpt-4",
      parent: null,
    },
    {
      id: "gpt-3.5-turbo",
      object: "model",
      created: Math.floor(Date.now() / 1000),
      owned_by: "aicarousel",
      permission: [],
      root: "gpt-3.5-turbo",
      parent: null,
    },
    {
      id: "claude-3-5-sonnet-20241022",
      object: "model",
      created: Math.floor(Date.now() / 1000),
      owned_by: "aicarousel",
      permission: [],
      root: "claude-3-5-sonnet-20241022",
      parent: null,
    },
  ];

  return Response.json({
    object: "list",
    data: models,
  });
}

/**
 * GET /v1/models/:model
 * Returns a specific model info.
 */
export function handleModelInfo(modelId: string): Response {
  return Response.json({
    id: modelId,
    object: "model",
    created: Math.floor(Date.now() / 1000),
    owned_by: "aicarousel",
    permission: [],
    root: modelId,
    parent: null,
  });
}
