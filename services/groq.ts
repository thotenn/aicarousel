import type { AIService, ChatMessage } from "@defaults/types";
import { Groq } from "groq-sdk";

const client = new Groq();

export const groqService: AIService = {
  name: "Groq",
  async *chat(messages: ChatMessage[]) {
    const stream = await client.chat.completions.create({
      model: "moonshotai/kimi-k2-instruct-0905",
      messages,
      temperature: 0.6,
      max_completion_tokens: 4096,
      top_p: 1,
      stream: true,
      stop: null,
    });
    for await (const chunk of stream) {
      yield chunk.choices[0]?.delta?.content || "";
    }
  },
};
