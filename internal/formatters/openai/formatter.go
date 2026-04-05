package openai

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/thotenn/aicarousel/internal/chat"
)

// ── streaming structs ────────────────────────────────────────────────────────

type streamChunk struct {
	ID      string          `json:"id"`
	Object  string          `json:"object"`
	Created int64           `json:"created"`
	Model   string          `json:"model"`
	Choices []streamChoice  `json:"choices"`
}

type streamChoice struct {
	Index        int     `json:"index"`
	Delta        delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// ── complete (non-streaming) structs ────────────────────────────────────────

// ChatCompletion is the non-streaming response returned by Complete.
type ChatCompletion struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []completeChoice `json:"choices"`
	Usage   usage            `json:"usage"`
}

type completeChoice struct {
	Index        int     `json:"index"`
	Message      message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ── error struct ─────────────────────────────────────────────────────────────

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    *string `json:"code"`
}

// ── public API ────────────────────────────────────────────────────────────────

// Stream writes OpenAI SSE frames to w for every chunk received from ch.
// flush is called after each write; pass nil for tests (no-op).
// The caller is responsible for setting Content-Type: text/event-stream.
func Stream(w io.Writer, flush func(), id string, created int64, model string, ch <-chan chat.StreamChunk) error {
	if flush == nil {
		flush = func() {}
	}

	stop := "stop"
	isFirst := true

	for chunk := range ch {
		if chunk.Err != nil {
			return chunk.Err
		}
		if chunk.Text == "" {
			continue
		}

		d := delta{Content: chunk.Text}
		if isFirst {
			d.Role = "assistant"
			isFirst = false
		}

		c := streamChunk{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   model,
			Choices: []streamChoice{
				{Index: 0, Delta: d, FinishReason: nil},
			},
		}
		b, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return err
		}
		flush()
	}

	// Final chunk: empty delta + finish_reason "stop"
	final := streamChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []streamChoice{
			{Index: 0, Delta: delta{}, FinishReason: &stop},
		},
	}
	b, err := json.Marshal(final)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
		return err
	}
	flush()

	if _, err := fmt.Fprint(w, "data: [DONE]\n\n"); err != nil {
		return err
	}
	flush()

	return nil
}

// Complete drains ch, collects all text, and returns a non-streaming ChatCompletion.
func Complete(id string, created int64, model string, ch <-chan chat.StreamChunk) (*ChatCompletion, error) {
	var text string
	for chunk := range ch {
		if chunk.Err != nil {
			return nil, chunk.Err
		}
		text += chunk.Text
	}

	tokens := (len(text) + 3) / 4

	return &ChatCompletion{
		ID:      id,
		Object:  "chat.completion",
		Created: created,
		Model:   model,
		Choices: []completeChoice{
			{
				Index:        0,
				Message:      message{Role: "assistant", Content: text},
				FinishReason: "stop",
			},
		},
		Usage: usage{
			PromptTokens:     0,
			CompletionTokens: tokens,
			TotalTokens:      tokens,
		},
	}, nil
}

// ErrorJSON returns a JSON-encoded OpenAI error response.
// errType defaults to "server_error" if empty. code may be nil.
func ErrorJSON(msg, errType string, code *string) []byte {
	if errType == "" {
		errType = "server_error"
	}
	b, _ := json.Marshal(errorResponse{
		Error: errorBody{
			Message: msg,
			Type:    errType,
			Param:   nil,
			Code:    code,
		},
	})
	return b
}
