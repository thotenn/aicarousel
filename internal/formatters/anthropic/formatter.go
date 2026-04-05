package anthropic

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/thotenn/aicarousel/internal/chat"
)

// ── message_start structs ────────────────────────────────────────────────────

type messageStartEvent struct {
	Type    string         `json:"type"`
	Message startMessage   `json:"message"`
}

type startMessage struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Role         string   `json:"role"`
	Content      []any    `json:"content"`
	Model        string   `json:"model"`
	StopReason   *string  `json:"stop_reason"`
	StopSequence *string  `json:"stop_sequence"`
	Usage        usage    `json:"usage"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ── content_block_start structs ──────────────────────────────────────────────

type contentBlockStartEvent struct {
	Type         string       `json:"type"`
	Index        int          `json:"index"`
	ContentBlock contentBlock `json:"content_block"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ── content_block_delta structs ──────────────────────────────────────────────

type contentBlockDeltaEvent struct {
	Type  string     `json:"type"`
	Index int        `json:"index"`
	Delta textDelta  `json:"delta"`
}

type textDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ── content_block_stop struct ────────────────────────────────────────────────

type contentBlockStopEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// ── message_delta structs ────────────────────────────────────────────────────

type messageDeltaEvent struct {
	Type  string        `json:"type"`
	Delta stopDelta     `json:"delta"`
	Usage deltaUsage    `json:"usage"`
}

type stopDelta struct {
	StopReason   string  `json:"stop_reason"`
	StopSequence *string `json:"stop_sequence"`
}

type deltaUsage struct {
	OutputTokens int `json:"output_tokens"`
}

// ── message_stop struct ───────────────────────────────────────────────────────

type messageStopEvent struct {
	Type string `json:"type"`
}

// ── complete (non-streaming) structs ─────────────────────────────────────────

// Message is the non-streaming response returned by Complete.
type Message struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []contentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        usage          `json:"usage"`
}

// ── error structs ─────────────────────────────────────────────────────────────

type errorResponse struct {
	Type  string    `json:"type"`
	Error errorBody `json:"error"`
}

type errorBody struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ── public API ────────────────────────────────────────────────────────────────

// Stream writes the 6-event Anthropic SSE sequence to w.
// flush is called after each event write; pass nil for tests (no-op).
func Stream(w io.Writer, flush func(), id string, model string, ch <-chan chat.StreamChunk) error {
	if flush == nil {
		flush = func() {}
	}

	// 1. message_start
	ev := messageStartEvent{
		Type: "message_start",
		Message: startMessage{
			ID:           id,
			Type:         "message",
			Role:         "assistant",
			Content:      []any{},
			Model:        model,
			StopReason:   nil,
			StopSequence: nil,
			Usage:        usage{InputTokens: 0, OutputTokens: 0},
		},
	}
	if err := writeEvent(w, "message_start", ev); err != nil {
		return err
	}
	flush()

	// 2. content_block_start
	cbs := contentBlockStartEvent{
		Type:         "content_block_start",
		Index:        0,
		ContentBlock: contentBlock{Type: "text", Text: ""},
	}
	if err := writeEvent(w, "content_block_start", cbs); err != nil {
		return err
	}
	flush()

	// 3. content_block_delta (N×)
	outputTokens := 0
	for chunk := range ch {
		if chunk.Err != nil {
			return chunk.Err
		}
		if chunk.Text == "" {
			continue
		}
		outputTokens += (len(chunk.Text) + 3) / 4

		cbd := contentBlockDeltaEvent{
			Type:  "content_block_delta",
			Index: 0,
			Delta: textDelta{Type: "text_delta", Text: chunk.Text},
		}
		if err := writeEvent(w, "content_block_delta", cbd); err != nil {
			return err
		}
		flush()
	}

	// 4. content_block_stop
	cbStop := contentBlockStopEvent{Type: "content_block_stop", Index: 0}
	if err := writeEvent(w, "content_block_stop", cbStop); err != nil {
		return err
	}
	flush()

	// 5. message_delta
	md := messageDeltaEvent{
		Type:  "message_delta",
		Delta: stopDelta{StopReason: "end_turn", StopSequence: nil},
		Usage: deltaUsage{OutputTokens: outputTokens},
	}
	if err := writeEvent(w, "message_delta", md); err != nil {
		return err
	}
	flush()

	// 6. message_stop
	ms := messageStopEvent{Type: "message_stop"}
	if err := writeEvent(w, "message_stop", ms); err != nil {
		return err
	}
	flush()

	return nil
}

// Complete drains ch, collects all text, and returns a non-streaming Message.
func Complete(id string, model string, ch <-chan chat.StreamChunk) (*Message, error) {
	var text string
	for chunk := range ch {
		if chunk.Err != nil {
			return nil, chunk.Err
		}
		text += chunk.Text
	}

	tokens := (len(text) + 3) / 4

	return &Message{
		ID:   id,
		Type: "message",
		Role: "assistant",
		Content: []contentBlock{
			{Type: "text", Text: text},
		},
		Model:        model,
		StopReason:   "end_turn",
		StopSequence: nil,
		Usage: usage{
			InputTokens:  0,
			OutputTokens: tokens,
		},
	}, nil
}

// ErrorJSON returns a JSON-encoded Anthropic error response.
// errType defaults to "api_error" if empty.
func ErrorJSON(msg, errType string) []byte {
	if errType == "" {
		errType = "api_error"
	}
	b, _ := json.Marshal(errorResponse{
		Type:  "error",
		Error: errorBody{Type: errType, Message: msg},
	})
	return b
}

// ── internal helpers ──────────────────────────────────────────────────────────

func writeEvent(w io.Writer, eventName string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, b)
	return err
}
