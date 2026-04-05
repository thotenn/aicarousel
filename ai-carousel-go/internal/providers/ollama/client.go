// Package ollama implements the Provider interface for local Ollama instances
// via its OpenAI-compatible streaming endpoint.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/thotenn/aicarousel-go/internal/chat"
	"github.com/thotenn/aicarousel-go/internal/config"
	"github.com/thotenn/aicarousel-go/internal/providers/provparams"
	"github.com/thotenn/aicarousel-go/internal/providers/sseparse"
)

type client struct {
	name       string
	key        string
	model      string
	params     provparams.Params
	url        string
	httpClient *http.Client
}

// New creates a production Ollama client. The base URL is taken from
// config.Cfg.OllamaBaseURL (default "http://localhost:11434").
func New(_ string, params provparams.Params) chat.Provider {
	base := config.Cfg.OllamaBaseURL
	if base == "" {
		base = "http://localhost:11434"
	}
	return newClient(base+"/v1/chat/completions", params, &http.Client{})
}

func newClient(url string, params provparams.Params, h *http.Client) *client {
	return &client{
		name:       "Ollama",
		key:        "ollama",
		model:      params.Model,
		params:     params,
		url:        url,
		httpClient: h,
	}
}

func (c *client) Name() string  { return c.name }
func (c *client) Key() string   { return c.key }
func (c *client) Model() string { return c.model }

// reqBody uses max_tokens (not max_completion_tokens) as expected by Ollama.
type reqBody struct {
	Model       string             `json:"model"`
	Messages    []chat.ChatMessage `json:"messages"`
	Stream      bool               `json:"stream"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature"`
	TopP        float64            `json:"top_p"`
}

type deltaChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func (c *client) Chat(ctx context.Context, msgs []chat.ChatMessage) (<-chan chat.StreamChunk, error) {
	body, err := json.Marshal(reqBody{
		Model:       c.model,
		Messages:    msgs,
		Stream:      true,
		MaxTokens:   c.params.MaxCompletionTokens,
		Temperature: c.params.Temperature,
		TopP:        c.params.TopP,
	})
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: build request: %w", err)
	}
	req.Header.Set("User-Agent", "AICarousel-Go/"+config.Version)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: request: %w", err)
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		return nil, fmt.Errorf("ollama: %d: %s", resp.StatusCode, b)
	}

	out := make(chan chat.StreamChunk, 10)
	go func() {
		defer close(out)
		defer resp.Body.Close() //nolint:errcheck

		err := sseparse.ForEachEvent(resp.Body, func(_ string, data []byte) error {
			var chunk deltaChunk
			if err := json.Unmarshal(data, &chunk); err != nil {
				// Ollama may emit malformed lines — skip silently (debug-level only).
				slog.Debug("ollama: skip malformed SSE line", "err", err)
				return nil
			}
			if len(chunk.Choices) == 0 {
				return nil
			}
			text := chunk.Choices[0].Delta.Content
			if text == "" {
				return nil
			}
			select {
			case out <- chat.StreamChunk{Text: text}:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			select {
			case out <- chat.StreamChunk{Err: err}:
			case <-ctx.Done():
			}
		}
	}()

	return out, nil
}
