import { handleChat } from "@services/chat_handler.ts";
import { handleChatCompletions, handleModels, handleModelInfo } from "./routes/openai.ts";
import { handleMessages, handleCountTokens } from "./routes/anthropic.ts";
import type { ChatMessage } from "@defaults/types";

const PORT = process.env.PORT ?? 7123;

Bun.serve({
  port: PORT,
  async fetch(req) {
    const url = new URL(req.url);
    const { pathname } = url;

    // CORS headers for browser-based clients
    const corsHeaders = {
      "Access-Control-Allow-Origin": "*",
      "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type, Authorization, x-api-key, anthropic-version, anthropic-beta",
    };

    // Handle CORS preflight
    if (req.method === "OPTIONS") {
      return new Response(null, { headers: corsHeaders });
    }

    try {
      let response: Response;

      // OpenAI-compatible endpoints (Cline, Codex, etc.)
      if (pathname === "/v1/chat/completions" && req.method === "POST") {
        response = await handleChatCompletions(req);
      }
      else if (pathname === "/v1/models" && req.method === "GET") {
        response = handleModels();
      }
      else if (pathname.startsWith("/v1/models/") && req.method === "GET") {
        const modelId = pathname.replace("/v1/models/", "");
        response = handleModelInfo(modelId);
      }
      // Anthropic-compatible endpoints (Claude Code)
      else if (pathname === "/v1/messages" && req.method === "POST") {
        response = await handleMessages(req);
      }
      else if (pathname === "/v1/messages/count_tokens" && req.method === "POST") {
        response = await handleCountTokens(req);
      }
      // Legacy endpoint (backward compatibility)
      else if (pathname === "/chat" && req.method === "POST") {
        response = await handleLegacyChat(req);
      }
      // Health check
      else if (pathname === "/health" && req.method === "GET") {
        response = Response.json({ status: "ok", service: "aicarousel" });
      }
      else {
        response = new Response("Not found", { status: 404 });
      }

      // Add CORS headers to response
      for (const [key, value] of Object.entries(corsHeaders)) {
        response.headers.set(key, value);
      }

      return response;
    } catch (error) {
      console.error("Unhandled error:", error);
      return new Response("Internal server error", { status: 500 });
    }
  },
});

/**
 * Legacy /chat endpoint handler for backward compatibility.
 */
async function handleLegacyChat(req: Request): Promise<Response> {
  try {
    const messages = (await req.json()) as ChatMessage[];
    const result = await handleChat(messages);

    return new Response(
      new ReadableStream({
        async start(controller) {
          const encoder = new TextEncoder();
          try {
            for await (const chunk of result.stream) {
              controller.enqueue(encoder.encode(chunk));
            }
          } catch (error) {
            console.error("Legacy stream error:", error);
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
        },
      }
    );
  } catch (error) {
    console.error("Legacy chat error:", error);
    return new Response("All AI services failed", { status: 503 });
  }
}

console.log(`🚀 AICarousel running on http://localhost:${PORT}`);
console.log(`
Available endpoints:
  POST /v1/chat/completions  - OpenAI compatible (Cline, Codex)
  GET  /v1/models            - OpenAI models list
  POST /v1/messages          - Anthropic compatible (Claude Code)
  POST /chat                 - Legacy endpoint
  GET  /health               - Health check
`);

/**
EXAMPLE REQUESTS:

# OpenAI format (Cline, Codex):
curl -X POST http://localhost:7123/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer dummy" \\
  -d '{"model": "aicarousel", "messages": [{"role": "user", "content": "Hello"}], "stream": true}'

# Anthropic format (Claude Code):
curl -X POST http://localhost:7123/v1/messages \\
  -H "Content-Type: application/json" \\
  -H "x-api-key: dummy" \\
  -H "anthropic-version: 2023-06-01" \\
  -d '{"model": "aicarousel", "max_tokens": 1024, "messages": [{"role": "user", "content": "Hello"}], "stream": true}'

# Legacy format:
curl -X POST http://localhost:7123/chat \\
  -H "Content-Type: application/json" \\
  -d '[{"role": "user", "content": "Resolve Fibonacci with C"}]'
*/
