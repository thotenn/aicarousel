/**
 * Ollama client adapter.
 * Ollama runs locally and exposes an OpenAI-compatible API.
 */

export class OllamaClient {
  private baseUrl: string;

  constructor() {
    this.baseUrl = process.env.OLLAMA_BASE_URL || "http://localhost:11434";
  }

  chat = {
    completions: {
      create: async (params: any) => {
        const response = await fetch(`${this.baseUrl}/v1/chat/completions`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            model: params.model,
            messages: params.messages,
            stream: params.stream ?? true,
            temperature: params.temperature,
            top_p: params.top_p,
            max_tokens: params.max_completion_tokens,
          }),
        });

        if (!response.ok) {
          const error = await response.text();
          throw new Error(`Ollama API error: ${response.status} - ${error}`);
        }

        if (params.stream) {
          return this.streamResponse(response);
        } else {
          const data = await response.json();
          return {
            choices: [
              {
                message: {
                  content: data.choices[0]?.message?.content || "",
                  role: "assistant",
                },
              },
            ],
          };
        }
      },
    },
  };

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
            yield {
              choices: [
                {
                  delta: {
                    content: parsed.choices[0]?.delta?.content || "",
                  },
                },
              ],
            };
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
