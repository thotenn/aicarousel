import { groqService } from "@services/groq";
import { cerebrasService } from "@services/cerebras";
import type { AIService, ChatMessage } from "@defaults/types";

const services: AIService[] = [groqService, cerebrasService];

let currentServiceIndex = 0;

function getNextService() {
  const service = services[currentServiceIndex];
  currentServiceIndex = (currentServiceIndex + 1) % services.length;
  return service;
}

Bun.serve({
  port: process.env.PORT ?? 7123,
  async fetch(req) {
    const { pathname } = new URL(req.url);

    if (req.method === "POST" && pathname === "/chat") {
      const messages = (await req.json()) as ChatMessage[];
      const service = getNextService();

      console.log("Using service:", service?.name);

      const stream = service?.chat(messages);

      return new Response(stream, {
        headers: {
          "Content-Type": "text/event-stream",
          "Cache-Control": "no-cache",
          Connection: "keep-alive",
        },
      });
    }
    return new Response("Not found", { status: 404 });
  },
});

/**
EXAMPLE REQUEST:
curl -X POST http://localhost:7123/chat -H "Content-Type: application/json" -d '[{"role": "user", "content": "Resolve Fibonacci with C"}]'
 */
