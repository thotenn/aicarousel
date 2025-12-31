import { models } from "./models";

const defaults = {
  max_completion_tokens: 4096,
  stream: true,
  temperature: 0.6,
  top_p: 1,
};

export const providers = {
  cerebras: {
    name: "Cerebras",
    params: {
      model: models.GLM46ZAI,
      max_completion_tokens: defaults.max_completion_tokens,
      stream: defaults.stream,
      temperature: defaults.temperature,
      top_p: defaults.top_p,
    },
  },
  groq: {
    name: "Groq",
    params: {
      model: models.KIMIK2I0905,
      max_completion_tokens: defaults.max_completion_tokens,
      stream: defaults.stream,
      stop: null,
      temperature: defaults.temperature,
      top_p: defaults.top_p,
    },
  },
  openrouter: {
    name: "OpenRouter",
    params: {
      model: models.QWEN3CODERFREE,
      stream: defaults.stream,
    },
  },
};
