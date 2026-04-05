// Package groq implements the Provider interface for the Groq AI API
// (OpenAI-compatible streaming endpoint).
package groq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/thotenn/aicarousel-go/internal/chat"
	"github.com/thotenn/aicarousel-go/internal/config"
	"github.com/thotenn/aicarousel-go/internal/providers/provparams"
	"github.com/thotenn/aicarousel-go/internal/providers/sseparse"
)

const productionURL = "https://api.groq.com/openai/v1/chat/completions"

type client struct {
	name       string
	key        string
	model      string
	apiKey     string
	params     provparams.Params
	url        string
	httpClient *http.Client
}

// New creates a production Groq client bound to params.Model.
func New(apiKey string, params provparams.Params) chat.Provider {
	return newClient(productionURL, apiKey, params, &http.Client{})
}

func newClient(url, apiKey string, params provparams.Params, h *http.Client) *client {
	return &client{
		name:       "Groq",
		key:        "groq",
		model:      params.Model,
		apiKey:     apiKey,
		params:     params,
		url:        url,
		httpClient: h,
	}
}

func (c *client) Name() string  { return c.name }
func (c *client) Key() string   { return c.key }
func (c *client) Model() string { return c.model }

type reqBody struct {
	Model               string             `json:"model"`
	Messages            []chat.ChatMessage `json:"messages"`
	Stream              bool               `json:"stream"`
	MaxCompletionTokens int                `json:"max_completion_tokens"`
	Temperature         float64            `json:"temperature"`
	TopP                float64            `json:"top_p"`
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
		Model:               c.model,
		Messages:            msgs,
		Stream:              true,
		MaxCompletionTokens: c.params.MaxCompletionTokens,
		Temperature:         c.params.Temperature,
		TopP:                c.params.TopP,
	})
	if err != nil {
		return nil, fmt.Errorf("groq: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("groq: build request: %w", err)
	}
	req.Header.Set("User-Agent", "AICarousel-Go/"+config.Version)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("groq: request: %w", err)
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		return nil, fmt.Errorf("groq: %d: %s", resp.StatusCode, b)
	}

	out := make(chan chat.StreamChunk, 10)
	go func() {
		defer close(out)
		defer resp.Body.Close() //nolint:errcheck

		err := sseparse.ForEachEvent(resp.Body, func(_ string, data []byte) error {
			var chunk deltaChunk
			if err := json.Unmarshal(data, &chunk); err != nil || len(chunk.Choices) == 0 {
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
