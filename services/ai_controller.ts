import type { AIService, ChatMessage } from "@defaults/types";
import { providers } from "@defaults/providers";
import { Cerebras } from "@cerebras/cerebras_cloud_sdk";
import { Groq } from "groq-sdk";
import { OpenRouterClient } from "./openrouter_client";
import { GeminiClient } from "./gemini_client";

export class StandardAIController implements AIService {
  name: string;
  private client: any;
  private params: any;

  constructor(name: string, client: any, params: any) {
    this.name = name;
    this.client = client;
    this.params = params;
  }

  async *chat(messages: ChatMessage[]) {
    const stream = await this.client.chat.completions.create({
      messages: messages as any,
      ...this.params,
    });
    for await (const chunk of stream as any) {
      yield chunk.choices[0]?.delta?.content || "";
    }
  }
}

const ClientMap: Record<string, any> = {
  cerebras: Cerebras,
  groq: Groq,
  openrouter: OpenRouterClient,
  gemini: GeminiClient,
};

export const services: AIService[] = Object.entries(providers).map(
  ([key, provider]) => {
    const Client = ClientMap[key];
    if (!Client) {
      throw new Error(`Client class not found for provider key: ${key}`);
    }
    return new StandardAIController(
      provider.name,
      new Client(),
      provider.params
    );
  }
);
