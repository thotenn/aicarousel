import { services } from "@services/ai_controller";
import type { ChatMessage } from "@defaults/types";

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
      let service = getNextService();

      const MAX_RETRIES = services.length;
      let attempts = 0;

      while (attempts < MAX_RETRIES) {
        if (!service) {
          console.error("No service available");
          break;
        }
        console.log("Using service:", service.name);
        try {
          const stream = service.chat(messages) as AsyncIterable<string>;

          let firstChunk: string | undefined;
          let iterator: AsyncIterator<string>;

          try {
            iterator = stream[Symbol.asyncIterator]();
            const result = await iterator.next();
            if (!result.done) {
              firstChunk = result.value;
            }
          } catch (error) {
            console.error(`Service ${service.name} failed with error:`, error);
            attempts++;
            service = getNextService();
            continue;
          }

          async function* combinedStream() {
            if (firstChunk !== undefined) yield firstChunk;
            while (true) {
              const { done, value } = await iterator.next();
              if (done) break;
              yield value;
            }
          }

          return new Response(combinedStream(), {
            headers: {
              "Content-Type": "text/event-stream",
              "Cache-Control": "no-cache",
              Connection: "keep-alive",
            },
          });
        } catch (e) {
          console.error(`Unexpected error with ${service?.name}:`, e);
          attempts++;
          service = getNextService();
        }
      }

      return new Response("All AI services failed", { status: 503 });
    }
    return new Response("Not found", { status: 404 });
  },
});

/**
EXAMPLE REQUEST:
curl -X POST http://localhost:7123/chat -H "Content-Type: application/json" -d '[{"role": "user", "content": "Resolve Fibonacci with C"}]'
 */
