import type { AIService, ChatMessage } from "@defaults/types";
import { Cerebras } from "@cerebras/cerebras_cloud_sdk";

const cerebras = new Cerebras();

export const cerebrasService: AIService = {
  name: "Cerebras",
  async *chat(messages: ChatMessage[]) {
    const stream = await cerebras.chat.completions.create({
      messages: messages as any,
      model: "zai-glm-4.6",
      stream: true,
      max_completion_tokens: 40960,
      temperature: 0.6,
      top_p: 0.95,
    });

    for await (const chunk of stream as any) {
      yield chunk.choices[0]?.delta?.content || "";
    }
  },
};
