/**
 * Z.ai client adapter.
 * Z.ai exposes an Anthropic-compatible API.
 * This adapter converts between the internal OpenAI-like format
 * and the Anthropic Messages API format.
 */

export class ZaiClient {
  private baseUrl: string;
  private apiKey: string;

  constructor() {
    this.baseUrl = process.env.ZAI_BASE_URL || "https://api.z.ai/api/anthropic";
    this.apiKey = process.env.ZAI_API_KEY || "";
  }

  chat = {
    completions: {
      create: async (params: any) => {
        const { systemPrompt, messages } = this.extractSystemMessage(params.messages);

        const body: Record<string, any> = {
          model: params.model,
          max_tokens: params.max_completion_tokens ?? 4096,
          messages,
          stream: params.stream ?? true,
        };

        if (systemPrompt) {
          body.system = systemPrompt;
        }

        if (params.temperature !== undefined) {
          body.temperature = params.temperature;
        }
        if (params.top_p !== undefined) {
          body.top_p = params.top_p;
        }

        const response = await fetch(`${this.baseUrl}/v1/messages`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "x-api-key": this.apiKey,
            "anthropic-version": "2023-06-01",
          },
          body: JSON.stringify(body),
        });

        if (!response.ok) {
          const error = await response.text();
          throw new Error(`Z.ai API error: ${response.status} - ${error}`);
        }

        if (params.stream) {
          return this.streamResponse(response);
        } else {
          const data: any = await response.json();
          const text = data.content
            ?.filter((b: any) => b.type === "text")
            .map((b: any) => b.text)
            .join("") || "";
          return {
            choices: [{ message: { content: text, role: "assistant" } }],
          };
        }
      },
    },
  };

  /**
   * Extract system messages from the messages array.
   * Anthropic API requires system as a separate top-level field.
   */
  private extractSystemMessage(messages: any[]): {
    systemPrompt: string | null;
    messages: any[];
  } {
    const systemMessages = messages.filter((m) => m.role === "system");
    const nonSystemMessages = messages.filter((m) => m.role !== "system");

    const systemPrompt =
      systemMessages.length > 0
        ? systemMessages.map((m) => m.content).join("\n")
        : null;

    return { systemPrompt, messages: nonSystemMessages };
  }

  /**
   * Parse Anthropic SSE stream and yield OpenAI-compatible chunks.
   */
  async *streamResponse(response: Response) {
    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error("No response body");
    }

    const decoder = new TextDecoder();
    let buffer = "";

    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed || !trimmed.startsWith("data:")) continue;

          const data = trimmed.slice(5).trim();
          if (data === "[DONE]") continue;

          try {
            const parsed = JSON.parse(data);

            // Only extract text from content_block_delta events
            if (parsed.type === "content_block_delta" && parsed.delta?.type === "text_delta") {
              yield {
                choices: [
                  {
                    delta: {
                      content: parsed.delta.text || "",
                    },
                  },
                ],
              };
            }
          } catch {
            // Skip invalid JSON
          }
        }
      }
    } finally {
      reader.releaseLock();
    }
  }
}
