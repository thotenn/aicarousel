import { GoogleGenerativeAI } from "@google/generative-ai";

export class GeminiClient {
  private client: GoogleGenerativeAI;

  constructor() {
    const apiKey = process.env.GEMINI_API_KEY || "";
    if (!apiKey) {
      console.warn("GEMINI_API_KEY is not set");
    }
    this.client = new GoogleGenerativeAI(apiKey);
  }

  chat = {
    completions: {
      create: async (params: any) => {
        const model = this.client.getGenerativeModel({
          model: params.model,
          systemInstruction: this.getSystemInstruction(params.messages),
        });

        const history = this.formatHistory(params.messages);

        // Gemini SDK doesn't support "system" role in history, it must be separate
        // So we filter out system messages from history
        const chat = model.startChat({
          history: history,
        });

        // The last message is the new user prompt
        const lastMessage = params.messages[params.messages.length - 1];
        const prompt = lastMessage.content;

        // Streaming check
        if (params.stream) {
          const result = await chat.sendMessageStream(prompt);
          return this.streamResponse(result);
        } else {
          const result = await chat.sendMessage(prompt);
          return {
            choices: [
              {
                message: {
                  content: result.response.text(),
                  role: "assistant",
                },
              },
            ],
          };
        }
      },
    },
  };

  private getSystemInstruction(messages: any[]) {
    const systemMsg = messages.find((m) => m.role === "system");
    return systemMsg ? systemMsg.content : undefined;
  }

  private formatHistory(messages: any[]) {
    // Filter out system messages (handled separately) and the last message (which is the prompt)
    // Gemini expects history to be previous turns.
    return messages
      .slice(0, -1)
      .filter((m) => m.role !== "system")
      .map((m) => ({
        role: m.role === "assistant" ? "model" : "user",
        parts: [{ text: m.content }],
      }));
  }

  async *streamResponse(result: any) {
    for await (const chunk of result.stream) {
      const text = chunk.text();
      yield {
        choices: [
          {
            delta: {
              content: text,
            },
          },
        ],
      };
    }
  }
}
