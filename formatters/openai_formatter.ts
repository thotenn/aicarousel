/**
 * OpenAI-compatible SSE formatter.
 * Transforms internal chat stream to OpenAI chat/completions streaming format.
 */

export interface OpenAIChatCompletionChunk {
  id: string;
  object: "chat.completion.chunk";
  created: number;
  model: string;
  choices: {
    index: number;
    delta: {
      role?: "assistant";
      content?: string;
    };
    finish_reason: string | null;
  }[];
}

export interface OpenAIChatCompletion {
  id: string;
  object: "chat.completion";
  created: number;
  model: string;
  choices: {
    index: number;
    message: {
      role: "assistant";
      content: string;
    };
    finish_reason: string;
  }[];
  usage: {
    prompt_tokens: number;
    completion_tokens: number;
    total_tokens: number;
  };
}

function generateId(): string {
  return `chatcmpl-${crypto.randomUUID().replace(/-/g, "").slice(0, 24)}`;
}

/**
 * Formats an async stream of text chunks into OpenAI SSE format.
 */
export async function* formatOpenAIStream(
  stream: AsyncIterable<string>,
  model: string = "aicarousel"
): AsyncGenerator<string> {
  const id = generateId();
  const created = Math.floor(Date.now() / 1000);

  // First chunk includes role
  let isFirst = true;

  for await (const content of stream) {
    if (content) {
      const chunk: OpenAIChatCompletionChunk = {
        id,
        object: "chat.completion.chunk",
        created,
        model,
        choices: [
          {
            index: 0,
            delta: isFirst
              ? { role: "assistant", content }
              : { content },
            finish_reason: null,
          },
        ],
      };
      isFirst = false;
      yield `data: ${JSON.stringify(chunk)}\n\n`;
    }
  }

  // Final chunk with finish_reason
  const finalChunk: OpenAIChatCompletionChunk = {
    id,
    object: "chat.completion.chunk",
    created,
    model,
    choices: [
      {
        index: 0,
        delta: {},
        finish_reason: "stop",
      },
    ],
  };
  yield `data: ${JSON.stringify(finalChunk)}\n\n`;
  yield "data: [DONE]\n\n";
}

/**
 * Collects stream and formats as non-streaming OpenAI response.
 */
export async function formatOpenAIComplete(
  stream: AsyncIterable<string>,
  model: string = "aicarousel"
): Promise<OpenAIChatCompletion> {
  const id = generateId();
  const created = Math.floor(Date.now() / 1000);

  let content = "";
  for await (const chunk of stream) {
    content += chunk;
  }

  // Rough token estimation (chars / 4)
  const completionTokens = Math.ceil(content.length / 4);

  return {
    id,
    object: "chat.completion",
    created,
    model,
    choices: [
      {
        index: 0,
        message: {
          role: "assistant",
          content,
        },
        finish_reason: "stop",
      },
    ],
    usage: {
      prompt_tokens: 0, // We don't have this info
      completion_tokens: completionTokens,
      total_tokens: completionTokens,
    },
  };
}

/**
 * Formats an error into OpenAI error response format.
 */
export function formatOpenAIError(
  message: string,
  type: string = "server_error",
  code: string | null = null
): object {
  return {
    error: {
      message,
      type,
      param: null,
      code,
    },
  };
}
