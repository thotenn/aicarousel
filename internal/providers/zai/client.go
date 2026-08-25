// Package zai implements the Provider interface for the Z.ai API, which uses
// an Anthropic-compatible SSE streaming protocol.
package zai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/thotenn/aicarousel/internal/chat"
	"github.com/thotenn/aicarousel/internal/config"
	"github.com/thotenn/aicarousel/internal/providers/provparams"
	"github.com/thotenn/aicarousel/internal/providers/sseparse"
)

const defaultBaseURL = "https://api.z.ai/api/anthropic"

type client struct {
	name       string
	key        string
	model      string
	apiKey     string
	params     provparams.Params
	url        string
	httpClient *http.Client
}

// New creates a production Z.ai client. The base URL is taken from
// config.Cfg.ZaiBaseURL (default "https://api.z.ai/api/anthropic").
func New(apiKey string, params provparams.Params) chat.Provider {
	base := config.Cfg.ZaiBaseURL
	if base == "" {
		base = defaultBaseURL
	}
	return newClient(base+"/v1/messages", apiKey, params, &http.Client{})
}

func newClient(url, apiKey string, params provparams.Params, h *http.Client) *client {
	return &client{
		name:       "Z.ai",
		key:        "zai",
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

// zaiMessage is the Anthropic-shaped message format accepted by Z.ai.
type zaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// reqBody is the Anthropic-shaped request body for Z.ai.
type reqBody struct {
	Model       string       `json:"model"`
	MaxTokens   int          `json:"max_tokens"`
	Messages    []zaiMessage `json:"messages"`
	System      string       `json:"system,omitempty"`
	Stream      bool         `json:"stream"`
	Temperature float64      `json:"temperature"`
	TopP        float64      `json:"top_p"`
}

// anthropicDelta is the SSE event shape for content_block_delta events.
type anthropicDelta struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

func (c *client) Chat(ctx context.Context, msgs []chat.ChatMessage) (<-chan chat.StreamChunk, error) {
	// Extract system messages and build the message list.
	var systemParts []string
	var zaiMsgs []zaiMessage
	for _, m := range msgs {
		switch m.Role {
		case "system":
			systemParts = append(systemParts, m.Content)
		default:
			zaiMsgs = append(zaiMsgs, zaiMessage{Role: m.Role, Content: m.Content})
		}
	}

	rb := reqBody{
		Model:       c.model,
		MaxTokens:   c.params.MaxCompletionTokens,
		Messages:    zaiMsgs,
		Stream:      true,
		Temperature: c.params.Temperature,
		TopP:        c.params.TopP,
	}
	if len(systemParts) > 0 {
		rb.System = strings.Join(systemParts, "\n")
	}

	body, err := json.Marshal(rb)
	if err != nil {
		return nil, fmt.Errorf("zai: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("zai: build request: %w", err)
	}
	req.Header.Set("User-Agent", "AICarousel-Go/"+config.Version)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("zai: request: %w", err)
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		return nil, fmt.Errorf("zai: %d: %s", resp.StatusCode, b)
	}

	out := make(chan chat.StreamChunk, 10)
	go func() {
		defer close(out)
		defer resp.Body.Close() //nolint:errcheck

		err := sseparse.ForEachEvent(resp.Body, func(event string, data []byte) error {
			if event != "content_block_delta" {
				return nil
			}
			var ev anthropicDelta
			if err := json.Unmarshal(data, &ev); err != nil {
				return nil
			}
			if ev.Type != "content_block_delta" || ev.Delta.Type != "text_delta" {
				return nil
			}
			if ev.Delta.Text == "" {
				return nil
			}
			select {
			case out <- chat.StreamChunk{Text: ev.Delta.Text}:
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
