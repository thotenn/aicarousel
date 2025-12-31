import { services } from "./ai_controller.ts";
import type { ChatMessage, AIService } from "@defaults/types";

let currentServiceIndex = 0;

export function getNextService(): AIService {
  const service = services[currentServiceIndex];
  currentServiceIndex = (currentServiceIndex + 1) % services.length;
  return service!;
}

export function getServicesCount(): number {
  return services.length;
}

export interface ChatResult {
  stream: AsyncIterable<string>;
  serviceName: string;
}

/**
 * Handles chat with automatic retry/fallback logic.
 * Returns a validated stream (first chunk already fetched) or throws if all services fail.
 */
export async function handleChat(messages: ChatMessage[]): Promise<ChatResult> {
  const MAX_RETRIES = services.length;
  let attempts = 0;
  let lastError: Error | null = null;

  while (attempts < MAX_RETRIES) {
    const service = getNextService();

    if (!service) {
      console.error("No service available");
      break;
    }

    console.log("Using service:", service.name);

    try {
      const stream = service.chat(messages) as AsyncIterable<string>;

      // Validate by fetching first chunk
      const iterator = stream[Symbol.asyncIterator]();
      const firstResult = await iterator.next();

      if (firstResult.done) {
        // Empty response, try next service
        console.error(`Service ${service.name} returned empty response`);
        attempts++;
        continue;
      }

      // Create combined stream with first chunk + rest
      const combinedStream = createCombinedStream(firstResult.value, iterator);

      return {
        stream: combinedStream,
        serviceName: service.name,
      };
    } catch (error) {
      console.error(`Service ${service.name} failed with error:`, error);
      lastError = error instanceof Error ? error : new Error(String(error));
      attempts++;
    }
  }

  throw lastError || new Error("All AI services failed");
}

async function* createCombinedStream(
  firstChunk: string,
  iterator: AsyncIterator<string>
): AsyncIterable<string> {
  yield firstChunk;

  while (true) {
    const { done, value } = await iterator.next();
    if (done) break;
    yield value;
  }
}
