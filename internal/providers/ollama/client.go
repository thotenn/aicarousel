// Package ollama implements the Provider interface for local Ollama instances
// via its native streaming chat endpoint.
//
// It deliberately uses /api/chat rather than the OpenAI-compatible
// /v1/chat/completions: the compatibility layer accepts no "options" object, so
// there is no way to set num_ctx through it. Ollama's default context window is
// 4096 tokens (2048 on older installs) and it truncates from the front, which
// drops the system prompt first.
package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/thotenn/aicarousel/internal/chat"
	"github.com/thotenn/aicarousel/internal/config"
	"github.com/thotenn/aicarousel/internal/providers/provparams"
)

// defaultNumCtx is used when OLLAMA_NUM_CTX is unset or invalid.
const defaultNumCtx = 8192

// maxLineBytes caps a single NDJSON line (chunks are small; this only guards
// against a pathological response).
const maxLineBytes = 1 << 20

type client struct {
	name       string
	key        string
	model      string
	params     provparams.Params
	numCtx     int
	keepAlive  string
	url        string
	httpClient *http.Client
}

// New creates a production Ollama client. The base URL is taken from
// config.Cfg.OllamaBaseURL (default "http://localhost:11434") and the context
// window from config.Cfg.OllamaNumCtx (default 8192).
func New(_ string, params provparams.Params) chat.Provider {
	base := config.Cfg.OllamaBaseURL
	if base == "" {
		base = "http://localhost:11434"
	}
	return newClient(base+"/api/chat", params, &http.Client{})
}

func newClient(url string, params provparams.Params, h *http.Client) *client {
	numCtx := config.Cfg.OllamaNumCtx
	if numCtx <= 0 {
		numCtx = defaultNumCtx
	}
	return &client{
		name:       "Ollama",
		key:        "ollama",
		model:      params.Model,
		params:     params,
		numCtx:     numCtx,
		keepAlive:  config.Cfg.OllamaKeepAlive,
		url:        url,
		httpClient: h,
	}
}

func (c *client) Name() string  { return c.name }
func (c *client) Key() string   { return c.key }
func (c *client) Model() string { return c.model }

// reqBody is Ollama's native /api/chat payload. Generation settings live in
// "options", the only place num_ctx can be set.
type reqBody struct {
	Model    string             `json:"model"`
	Messages []chat.ChatMessage `json:"messages"`
	Stream   bool               `json:"stream"`
	Options  reqOptions         `json:"options"`
	// KeepAlive controls how long the model stays resident in RAM after this
	// request. Loading a cold model is what makes the first request slow enough
	// for the router probe to give up on it. Raw JSON because Ollama reads the
	// two types differently — see keepAliveValue.
	KeepAlive json.RawMessage `json:"keep_alive,omitempty"`
}

type reqOptions struct {
	NumCtx      int      `json:"num_ctx"`
	NumPredict  int      `json:"num_predict"`
	Temperature float64  `json:"temperature"`
	TopP        float64  `json:"top_p"`
	Stop        []string `json:"stop,omitempty"`
}

// chatChunk is one NDJSON line of the native stream. Reasoning models emit their
// chain of thought in a separate "thinking" field, which is deliberately ignored.
type chatChunk struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

func (c *client) Chat(ctx context.Context, msgs []chat.ChatMessage) (<-chan chat.StreamChunk, error) {
	opts := reqOptions{
		NumCtx:      c.numCtx,
		NumPredict:  c.params.MaxCompletionTokens,
		Temperature: c.params.Temperature,
		TopP:        c.params.TopP,
	}
	if c.params.Stop != nil && *c.params.Stop != "" {
		opts.Stop = []string{*c.params.Stop}
	}

	body, err := json.Marshal(reqBody{
		Model:     c.model,
		Messages:  msgs,
		Stream:    true,
		Options:   opts,
		KeepAlive: keepAliveValue(c.keepAlive),
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
	req.Header.Set("Accept", "application/x-ndjson")

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

		if err := c.forwardStream(ctx, resp.Body, out); err != nil &&
			!errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			select {
			case out <- chat.StreamChunk{Err: err}:
			case <-ctx.Done():
			}
		}
	}()

	return out, nil
}

// keepAliveValue renders OLLAMA_KEEP_ALIVE the way Ollama's API reads it: a
// JSON number is seconds and a negative one means "keep it loaded forever",
// while a JSON string goes through time.ParseDuration ("30m", "1h").
//
// The two are not interchangeable: ParseDuration rejects "-1" — the exact value
// Ollama's own docs give for forever — because it has no unit. So a bare
// integer has to go over the wire as a number, and everything else as a string.
// An empty value is omitted entirely, leaving Ollama's 5-minute default.
func keepAliveValue(raw string) json.RawMessage {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return json.RawMessage(strconv.Itoa(n))
	}
	quoted, err := json.Marshal(raw)
	if err != nil {
		return nil // unreachable for a string, but never send junk
	}
	return quoted
}

// forwardStream reads the NDJSON body line by line and forwards content deltas.
func (c *client) forwardStream(ctx context.Context, body io.Reader, out chan<- chat.StreamChunk) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var chunk chatChunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			// Ollama may emit malformed lines — skip silently (debug-level only).
			slog.Debug("ollama: skip malformed NDJSON line", "err", err)
			continue
		}
		if chunk.Message.Content == "" {
			continue
		}

		select {
		case out <- chat.StreamChunk{Text: chunk.Message.Content}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("ollama: read stream: %w", err)
	}
	return nil
}
