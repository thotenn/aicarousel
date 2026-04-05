package openai

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/thotenn/aicarousel-go/internal/chat"
)

// makeChunks converts strings to a closed StreamChunk channel.
func makeChunks(texts ...string) <-chan chat.StreamChunk {
	ch := make(chan chat.StreamChunk, len(texts))
	for _, t := range texts {
		ch <- chat.StreamChunk{Text: t}
	}
	close(ch)
	return ch
}

// ── Stream ────────────────────────────────────────────────────────────────────

func TestStream_BasicOutput(t *testing.T) {
	var buf bytes.Buffer
	err := Stream(&buf, nil, "chatcmpl-abc123", 1700000000, "aicarousel", makeChunks("Hello", " world"))
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()

	// Must contain data: lines
	if !strings.Contains(out, "data: {") {
		t.Error("no data frames found")
	}
	// Must end with [DONE]
	if !strings.HasSuffix(strings.TrimRight(out, "\n"), "data: [DONE]") {
		t.Errorf("missing [DONE] terminator:\n%s", out)
	}
}

func TestStream_FirstChunkHasRole(t *testing.T) {
	var buf bytes.Buffer
	_ = Stream(&buf, nil, "chatcmpl-abc123", 1700000000, "aicarousel", makeChunks("hi"))

	lines := sseDataLines(buf.String())
	if len(lines) < 1 {
		t.Fatal("no data lines")
	}

	var first streamChunk
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if first.Choices[0].Delta.Role != "assistant" {
		t.Errorf("first chunk role: got %q, want %q", first.Choices[0].Delta.Role, "assistant")
	}
	if first.Choices[0].Delta.Content != "hi" {
		t.Errorf("first chunk content: got %q, want %q", first.Choices[0].Delta.Content, "hi")
	}
}

func TestStream_SubsequentChunksNoRole(t *testing.T) {
	var buf bytes.Buffer
	_ = Stream(&buf, nil, "chatcmpl-abc123", 1700000000, "aicarousel", makeChunks("a", "b", "c"))

	lines := sseDataLines(buf.String())
	// lines[0]=first chunk, lines[1]=second, ..., last=final empty, then [DONE]
	if len(lines) < 2 {
		t.Fatal("expected multiple data lines")
	}
	var second streamChunk
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("unmarshal second: %v", err)
	}
	if second.Choices[0].Delta.Role != "" {
		t.Errorf("second chunk should have no role, got %q", second.Choices[0].Delta.Role)
	}
}

func TestStream_FinalChunkFinishReason(t *testing.T) {
	var buf bytes.Buffer
	_ = Stream(&buf, nil, "chatcmpl-abc123", 1700000000, "aicarousel", makeChunks("x"))

	lines := sseDataLines(buf.String())
	// last data line before [DONE] is the final chunk
	finalLine := lines[len(lines)-1]
	var final streamChunk
	if err := json.Unmarshal([]byte(finalLine), &final); err != nil {
		t.Fatalf("unmarshal final: %v", err)
	}
	if final.Choices[0].FinishReason == nil || *final.Choices[0].FinishReason != "stop" {
		t.Errorf("final chunk finish_reason: got %v, want \"stop\"", final.Choices[0].FinishReason)
	}
	if final.Choices[0].Delta.Role != "" || final.Choices[0].Delta.Content != "" {
		t.Errorf("final delta must be empty, got role=%q content=%q",
			final.Choices[0].Delta.Role, final.Choices[0].Delta.Content)
	}
}

func TestStream_FieldOrder(t *testing.T) {
	var buf bytes.Buffer
	_ = Stream(&buf, nil, "chatcmpl-abc123", 1700000000, "aicarousel", makeChunks("hi"))

	lines := sseDataLines(buf.String())
	raw := lines[0]

	// id must appear before object, object before created, created before model, model before choices
	checkOrder(t, raw, "\"id\"", "\"object\"")
	checkOrder(t, raw, "\"object\"", "\"created\"")
	checkOrder(t, raw, "\"created\"", "\"model\"")
	checkOrder(t, raw, "\"model\"", "\"choices\"")
}

func TestStream_IDAndMetaPreserved(t *testing.T) {
	var buf bytes.Buffer
	_ = Stream(&buf, nil, "chatcmpl-testid99", 9999999, "mymodel", makeChunks("ok"))

	lines := sseDataLines(buf.String())
	var c streamChunk
	json.Unmarshal([]byte(lines[0]), &c) //nolint:errcheck
	if c.ID != "chatcmpl-testid99" {
		t.Errorf("id: got %q, want %q", c.ID, "chatcmpl-testid99")
	}
	if c.Created != 9999999 {
		t.Errorf("created: got %d, want %d", c.Created, 9999999)
	}
	if c.Model != "mymodel" {
		t.Errorf("model: got %q, want %q", c.Model, "mymodel")
	}
}

func TestStream_EmptyChunksSkipped(t *testing.T) {
	// Empty text chunks must not produce data frames.
	ch := make(chan chat.StreamChunk, 3)
	ch <- chat.StreamChunk{Text: ""}
	ch <- chat.StreamChunk{Text: "hello"}
	ch <- chat.StreamChunk{Text: ""}
	close(ch)

	var buf bytes.Buffer
	_ = Stream(&buf, nil, "id", 1, "m", ch)

	lines := sseDataLines(buf.String())
	// Expect exactly: first(hello) + final = 2 data lines
	if len(lines) != 2 {
		t.Errorf("expected 2 data lines, got %d:\n%s", len(lines), buf.String())
	}
}

func TestStream_FlushCalled(t *testing.T) {
	flushCount := 0
	flush := func() { flushCount++ }

	var buf bytes.Buffer
	_ = Stream(&buf, flush, "id", 1, "m", makeChunks("a", "b"))

	// 2 content chunks + 1 final + 1 [DONE] = 4 flushes
	if flushCount != 4 {
		t.Errorf("expected 4 flushes, got %d", flushCount)
	}
}

func TestStream_ErrorChunk(t *testing.T) {
	ch := make(chan chat.StreamChunk, 2)
	ch <- chat.StreamChunk{Text: "partial"}
	ch <- chat.StreamChunk{Err: errTest("boom")}
	close(ch)

	var buf bytes.Buffer
	err := Stream(&buf, nil, "id", 1, "m", ch)
	if err == nil || err.Error() != "boom" {
		t.Errorf("expected error 'boom', got %v", err)
	}
}

// ── Complete ──────────────────────────────────────────────────────────────────

func TestComplete_Basic(t *testing.T) {
	resp, err := Complete("chatcmpl-xyz", 1700000000, "aicarousel", makeChunks("Hello", " world"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "chatcmpl-xyz" {
		t.Errorf("id: got %q", resp.ID)
	}
	if resp.Object != "chat.completion" {
		t.Errorf("object: got %q", resp.Object)
	}
	if resp.Choices[0].Message.Content != "Hello world" {
		t.Errorf("content: got %q", resp.Choices[0].Message.Content)
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Errorf("role: got %q", resp.Choices[0].Message.Role)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason: got %q", resp.Choices[0].FinishReason)
	}
}

func TestComplete_TokenEstimation(t *testing.T) {
	cases := []struct {
		text   string
		tokens int
	}{
		{"", 0},
		{"abc", 1},       // ceil(3/4)=1
		{"abcd", 1},      // ceil(4/4)=1
		{"abcde", 2},     // ceil(5/4)=2
		{"abcdefgh", 2},  // ceil(8/4)=2
		{"abcdefghi", 3}, // ceil(9/4)=3
	}
	for _, tc := range cases {
		ch := makeChunks(tc.text)
		resp, _ := Complete("id", 1, "m", ch)
		if resp.Usage.CompletionTokens != tc.tokens {
			t.Errorf("text=%q: got %d tokens, want %d", tc.text, resp.Usage.CompletionTokens, tc.tokens)
		}
		if resp.Usage.TotalTokens != tc.tokens {
			t.Errorf("text=%q: total_tokens got %d, want %d", tc.text, resp.Usage.TotalTokens, tc.tokens)
		}
		if resp.Usage.PromptTokens != 0 {
			t.Errorf("prompt_tokens should be 0")
		}
	}
}

func TestComplete_JSONFieldOrder(t *testing.T) {
	resp, _ := Complete("chatcmpl-abc", 1234, "aicarousel", makeChunks("hi"))
	b, _ := json.Marshal(resp)
	raw := string(b)

	checkOrder(t, raw, "\"id\"", "\"object\"")
	checkOrder(t, raw, "\"object\"", "\"created\"")
	checkOrder(t, raw, "\"created\"", "\"model\"")
	checkOrder(t, raw, "\"model\"", "\"choices\"")
	checkOrder(t, raw, "\"choices\"", "\"usage\"")
}

func TestComplete_ErrorChunk(t *testing.T) {
	ch := make(chan chat.StreamChunk, 1)
	ch <- chat.StreamChunk{Err: errTest("provider down")}
	close(ch)

	_, err := Complete("id", 1, "m", ch)
	if err == nil || err.Error() != "provider down" {
		t.Errorf("expected error, got %v", err)
	}
}

// ── ErrorJSON ─────────────────────────────────────────────────────────────────

func TestErrorJSON_WithCode(t *testing.T) {
	code := "invalid_api_key"
	b := ErrorJSON("bad key", "invalid_request_error", &code)

	var resp errorResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error.Message != "bad key" {
		t.Errorf("message: %q", resp.Error.Message)
	}
	if resp.Error.Type != "invalid_request_error" {
		t.Errorf("type: %q", resp.Error.Type)
	}
	if resp.Error.Code == nil || *resp.Error.Code != "invalid_api_key" {
		t.Errorf("code: %v", resp.Error.Code)
	}
	if resp.Error.Param != nil {
		t.Errorf("param should be null")
	}
}

func TestErrorJSON_NilCode(t *testing.T) {
	b := ErrorJSON("oops", "api_error", nil)
	raw := string(b)
	if !strings.Contains(raw, `"code":null`) {
		t.Errorf("expected null code in: %s", raw)
	}
}

func TestErrorJSON_DefaultType(t *testing.T) {
	b := ErrorJSON("err", "", nil)
	var resp errorResponse
	json.Unmarshal(b, &resp) //nolint:errcheck
	if resp.Error.Type != "server_error" {
		t.Errorf("default type: got %q", resp.Error.Type)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// sseDataLines returns the JSON payloads from "data: {json}" lines (skips [DONE]).
func sseDataLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			if payload != "[DONE]" {
				out = append(out, payload)
			}
		}
	}
	return out
}

func checkOrder(t *testing.T, s, before, after string) {
	t.Helper()
	bi := strings.Index(s, before)
	ai := strings.Index(s, after)
	if bi < 0 || ai < 0 || bi >= ai {
		t.Errorf("field order: %q must appear before %q in:\n%s", before, after, s)
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }
