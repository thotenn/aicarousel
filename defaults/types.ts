export interface ChatMessage {
  role: "user" | "assistant" | "system";
  content: string;
}

export interface AIService {
  name: string;
  chat(messages: ChatMessage[]): AsyncIterable<string>;
}

/**
 * Extended AIService with provider and model information.
 * Used for fallback logic.
 */
export interface AIServiceWithModel extends AIService {
  providerKey: string;
  model: string;
}

/**
 * Active provider information with fallback configuration.
 */
export interface ActiveProvider {
  key: string;
  name: string;
  models: string[];
  defaultModel: string;
  enableFallback: boolean;
  priority: number;
}
