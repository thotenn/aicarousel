// Package gemini implements the Provider interface for the Google Gemini API
// using its native streaming SSE endpoint.
package gemini

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

const productionBaseURL = "https://generativelanguage.googleapis.com"

type client struct {
	name       string
	key        string
	model      string
	apiKey     string
	params     provparams.Params
	baseURL    string
	httpClient *http.Client
}

// New creates a production Gemini client bound to params.Model.
func New(apiKey string, params provparams.Params) chat.Provider {
	return newClient(productionBaseURL, apiKey, params, &http.Client{})
}

func newClient(baseURL, apiKey string, params provparams.Params, h *http.Client) *client {
	return &client{
		name:       "Gemini",
		key:        "gemini",
		model:      params.Model,
		apiKey:     apiKey,
		params:     params,
		baseURL:    baseURL,
		httpClient: h,
	}
}

func (c *client) Name() string  { return c.name }
func (c *client) Key() string   { return c.key }
func (c *client) Model() string { return c.model }

// endpointURL returns the full streaming URL including model and API key.
func (c *client) endpointURL() string {
	return fmt.Sprintf(
		"%s/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s",
		c.baseURL, c.model, c.apiKey,
	)
}

// Gemini request types.

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type systemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type generationConfig struct {
	Temperature     float64 `json:"temperature"`
	TopP            float64 `json:"topP"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

type reqBody struct {
	Contents            []geminiContent   `json:"contents"`
	SystemInstruction   *systemInstruction `json:"systemInstruction,omitempty"`
	GenerationConfig    generationConfig   `json:"generationConfig"`
}

// Gemini response types.

type geminiChunk struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (c *client) Chat(ctx context.Context, msgs []chat.ChatMessage) (<-chan chat.StreamChunk, error) {
	// Map messages: extract system, convert roles.
	var sysParts []string
	var contents []geminiContent
	for _, m := range msgs {
		switch m.Role {
		case "system":
			sysParts = append(sysParts, m.Content)
		case "assistant":
			contents = append(contents, geminiContent{
				Role:  "model",
				Parts: []geminiPart{{Text: m.Content}},
			})
		default: // "user"
			contents = append(contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: m.Content}},
			})
		}
	}

	rb := reqBody{
		Contents: contents,
		GenerationConfig: generationConfig{
			Temperature:     c.params.Temperature,
			TopP:            c.params.TopP,
			MaxOutputTokens: c.params.MaxCompletionTokens,
		},
	}
	if len(sysParts) > 0 {
		rb.SystemInstruction = &systemInstruction{
			Parts: []geminiPart{{Text: strings.Join(sysParts, "\n")}},
		}
	}

	body, err := json.Marshal(rb)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpointURL(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini: build request: %w", err)
	}
	req.Header.Set("User-Agent", "AICarousel-Go/"+config.Version)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: request: %w", err)
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		return nil, fmt.Errorf("gemini: %d: %s", resp.StatusCode, b)
	}

	out := make(chan chat.StreamChunk, 10)
	go func() {
		defer close(out)
		defer resp.Body.Close() //nolint:errcheck

		err := sseparse.ForEachEvent(resp.Body, func(_ string, data []byte) error {
			var chunk geminiChunk
			if err := json.Unmarshal(data, &chunk); err != nil || len(chunk.Candidates) == 0 {
				return nil
			}
			// Join all parts text.
			var sb strings.Builder
			for _, part := range chunk.Candidates[0].Content.Parts {
				sb.WriteString(part.Text)
			}
			text := sb.String()
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
