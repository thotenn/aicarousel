/**
 * Anthropic Messages API SSE formatter.
 * Transforms internal chat stream to Anthropic streaming format.
 */

export interface AnthropicMessage {
  id: string;
  type: "message";
  role: "assistant";
  content: { type: "text"; text: string }[];
  model: string;
  stop_reason: string | null;
  stop_sequence: string | null;
  usage: {
    input_tokens: number;
    output_tokens: number;
  };
}

function generateMessageId(): string {
  return `msg_${crypto.randomUUID().replace(/-/g, "").slice(0, 24)}`;
}

/**
 * Formats an async stream of text chunks into Anthropic SSE format.
 */
export async function* formatAnthropicStream(
  stream: AsyncIterable<string>,
  model: string = "aicarousel"
): AsyncGenerator<string> {
  const msgId = generateMessageId();

  // message_start event
  yield formatSSE("message_start", {
    type: "message_start",
    message: {
      id: msgId,
      type: "message",
      role: "assistant",
      content: [],
      model,
      stop_reason: null,
      stop_sequence: null,
      usage: {
        input_tokens: 0,
        output_tokens: 0,
      },
    },
  });

  // content_block_start event
  yield formatSSE("content_block_start", {
    type: "content_block_start",
    index: 0,
    content_block: {
      type: "text",
      text: "",
    },
  });

  let outputTokens = 0;

  // Stream content deltas
  for await (const content of stream) {
    if (content) {
      // Rough token estimation
      outputTokens += Math.ceil(content.length / 4);

      yield formatSSE("content_block_delta", {
        type: "content_block_delta",
        index: 0,
        delta: {
          type: "text_delta",
          text: content,
        },
      });
    }
  }

  // content_block_stop event
  yield formatSSE("content_block_stop", {
    type: "content_block_stop",
    index: 0,
  });

  // message_delta event (final usage and stop reason)
  yield formatSSE("message_delta", {
    type: "message_delta",
    delta: {
      stop_reason: "end_turn",
      stop_sequence: null,
    },
    usage: {
      output_tokens: outputTokens,
    },
  });

  // message_stop event
  yield formatSSE("message_stop", {
    type: "message_stop",
  });
}

/**
 * Collects stream and formats as non-streaming Anthropic response.
 */
export async function formatAnthropicComplete(
  stream: AsyncIterable<string>,
  model: string = "aicarousel"
): Promise<AnthropicMessage> {
  const msgId = generateMessageId();

  let content = "";
  for await (const chunk of stream) {
    content += chunk;
  }

  const outputTokens = Math.ceil(content.length / 4);

  return {
    id: msgId,
    type: "message",
    role: "assistant",
    content: [
      {
        type: "text",
        text: content,
      },
    ],
    model,
    stop_reason: "end_turn",
    stop_sequence: null,
    usage: {
      input_tokens: 0,
      output_tokens: outputTokens,
    },
  };
}

/**
 * Format a single SSE event with event type and data.
 */
function formatSSE(event: string, data: object): string {
  return `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
}

/**
 * Formats an error into Anthropic error response format.
 */
export function formatAnthropicError(
  message: string,
  type: string = "api_error"
): object {
  return {
    type: "error",
    error: {
      type,
      message,
    },
  };
}
