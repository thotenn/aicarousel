import type { AIService, ChatMessage } from "@defaults/types";
import { Cerebras } from "@cerebras/cerebras_cloud_sdk";
import { providers } from "@defaults/providers";

const cerebras = new Cerebras();

export const cerebrasService: AIService = {
  name: providers.cerebras.name,
  async *chat(messages: ChatMessage[]) {
    const stream = await cerebras.chat.completions.create({
      messages: messages as any,
      ...providers.cerebras.params,
    });
    for await (const chunk of stream as any) {
      yield chunk.choices[0]?.delta?.content || "";
    }
  },
};
