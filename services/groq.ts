import type { AIService, ChatMessage } from "@defaults/types";
import { Groq } from "groq-sdk";
import { providers } from "@defaults/providers";

const client = new Groq();

export const groqService: AIService = {
  name: providers.groq.name,
  async *chat(messages: ChatMessage[]) {
    const stream = await client.chat.completions.create({
      messages: messages as any,
      ...providers.groq.params,
    });
    for await (const chunk of stream as any) {
      yield chunk.choices[0]?.delta?.content || "";
    }
  },
};
