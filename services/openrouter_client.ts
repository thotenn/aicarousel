import { OpenRouter } from "@openrouter/sdk";

export class OpenRouterClient {
  private client: OpenRouter;

  constructor() {
    this.client = new OpenRouter({
      apiKey: process.env.OPENROUTER_API_KEY,
    });
  }

  chat = {
    completions: {
      create: async (params: any) => {
        const stream = await this.client.chat.send({
          ...params,
        });

        return stream;
      },
    },
  };
}
