package anthropic

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

// parseEvents extracts (eventName, jsonPayload) pairs from the SSE output.
func parseEvents(s string) []struct{ name, data string } {
	var out []struct{ name, data string }
	var name string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "event: ") {
			name = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			out = append(out, struct{ name, data string }{name, data})
			name = ""
		}
	}
	return out
}

// ── Stream: event sequence ────────────────────────────────────────────────────

func TestStream_EventSequence(t *testing.T) {
	var buf bytes.Buffer
	err := Stream(&buf, nil, "msg_abc123456789012345678901", "claude-3-5-sonnet", makeChunks("Hello", " world"))
	if err != nil {
		t.Fatal(err)
	}

	events := parseEvents(buf.String())
	want := []string{
		"message_start",
		"content_block_start",
		"content_block_delta",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}
	if len(events) != len(want) {
		t.Fatalf("event count: got %d, want %d\n%s", len(events), len(want), buf.String())
	}
	for i, ev := range events {
		if ev.name != want[i] {
			t.Errorf("event[%d]: got %q, want %q", i, ev.name, want[i])
		}
	}
}

func TestStream_MessageStart(t *testing.T) {
	var buf bytes.Buffer
	_ = Stream(&buf, nil, "msg_testid", "mymodel", makeChunks("hi"))
	events := parseEvents(buf.String())

	var ev messageStartEvent
	if err := json.Unmarshal([]byte(events[0].data), &ev); err != nil {
		t.Fatalf("unmarshal message_start: %v", err)
	}
	if ev.Type != "message_start" {
		t.Errorf("type: %q", ev.Type)
	}
	if ev.Message.ID != "msg_testid" {
		t.Errorf("message.id: %q", ev.Message.ID)
	}
	if ev.Message.Type != "message" {
		t.Errorf("message.type: %q", ev.Message.Type)
	}
	if ev.Message.Role != "assistant" {
		t.Errorf("message.role: %q", ev.Message.Role)
	}
	if ev.Message.Model != "mymodel" {
		t.Errorf("message.model: %q", ev.Message.Model)
	}
	if ev.Message.StopReason != nil {
		t.Errorf("stop_reason should be null")
	}
	if ev.Message.StopSequence != nil {
		t.Errorf("stop_sequence should be null")
	}
	if ev.Message.Usage.InputTokens != 0 || ev.Message.Usage.OutputTokens != 0 {
		t.Errorf("usage should be 0/0")
	}
}

func TestStream_MessageStartFieldOrder(t *testing.T) {
	var buf bytes.Buffer
	_ = Stream(&buf, nil, "msg_testid", "mymodel", makeChunks("hi"))
	events := parseEvents(buf.String())

	// Re-marshal just the message sub-object to check its field order.
	var ev messageStartEvent
	if err := json.Unmarshal([]byte(events[0].data), &ev); err != nil {
		t.Fatal(err)
	}
	msgBytes, _ := json.Marshal(ev.Message)
	raw := string(msgBytes)

	// message fields: id, type, role, content, model, stop_reason, stop_sequence, usage
	checkOrder(t, raw, "\"id\"", "\"type\"")
	checkOrder(t, raw, "\"type\"", "\"role\"")
	checkOrder(t, raw, "\"role\"", "\"content\"")
	checkOrder(t, raw, "\"content\"", "\"model\"")
	checkOrder(t, raw, "\"model\"", "\"stop_reason\"")
	checkOrder(t, raw, "\"stop_reason\"", "\"stop_sequence\"")
}

func TestStream_ContentBlockStart(t *testing.T) {
	var buf bytes.Buffer
	_ = Stream(&buf, nil, "id", "m", makeChunks("hi"))
	events := parseEvents(buf.String())

	var ev contentBlockStartEvent
	if err := json.Unmarshal([]byte(events[1].data), &ev); err != nil {
		t.Fatalf("unmarshal content_block_start: %v", err)
	}
	if ev.Type != "content_block_start" {
		t.Errorf("type: %q", ev.Type)
	}
	if ev.Index != 0 {
		t.Errorf("index: %d", ev.Index)
	}
	if ev.ContentBlock.Type != "text" || ev.ContentBlock.Text != "" {
		t.Errorf("content_block: %+v", ev.ContentBlock)
	}
}

func TestStream_ContentBlockDelta(t *testing.T) {
	var buf bytes.Buffer
	_ = Stream(&buf, nil, "id", "m", makeChunks("Hello", " world"))
	events := parseEvents(buf.String())

	// events[2] and events[3] are content_block_delta
	for i, wantText := range []string{"Hello", " world"} {
		var ev contentBlockDeltaEvent
		if err := json.Unmarshal([]byte(events[2+i].data), &ev); err != nil {
			t.Fatalf("unmarshal delta[%d]: %v", i, err)
		}
		if ev.Type != "content_block_delta" {
			t.Errorf("delta[%d] type: %q", i, ev.Type)
		}
		if ev.Delta.Type != "text_delta" {
			t.Errorf("delta[%d] delta.type: %q", i, ev.Delta.Type)
		}
		if ev.Delta.Text != wantText {
			t.Errorf("delta[%d] text: got %q, want %q", i, ev.Delta.Text, wantText)
		}
	}
}

func TestStream_ContentBlockStop(t *testing.T) {
	var buf bytes.Buffer
	_ = Stream(&buf, nil, "id", "m", makeChunks("x"))
	events := parseEvents(buf.String())

	// events[3] = content_block_stop (after message_start, content_block_start, 1 delta)
	var ev contentBlockStopEvent
	if err := json.Unmarshal([]byte(events[3].data), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.Type != "content_block_stop" || ev.Index != 0 {
		t.Errorf("content_block_stop: %+v", ev)
	}
}

func TestStream_MessageDelta(t *testing.T) {
	var buf bytes.Buffer
	_ = Stream(&buf, nil, "id", "m", makeChunks("abcd")) // 1 token
	events := parseEvents(buf.String())

	// events[4] = message_delta
	var ev messageDeltaEvent
	if err := json.Unmarshal([]byte(events[4].data), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.Type != "message_delta" {
		t.Errorf("type: %q", ev.Type)
	}
	if ev.Delta.StopReason != "end_turn" {
		t.Errorf("stop_reason: %q", ev.Delta.StopReason)
	}
	if ev.Delta.StopSequence != nil {
		t.Errorf("stop_sequence should be null")
	}
	if ev.Usage.OutputTokens != 1 {
		t.Errorf("output_tokens: got %d, want 1", ev.Usage.OutputTokens)
	}
}

func TestStream_MessageStop(t *testing.T) {
	var buf bytes.Buffer
	_ = Stream(&buf, nil, "id", "m", makeChunks("x"))
	events := parseEvents(buf.String())

	// last event
	last := events[len(events)-1]
	if last.name != "message_stop" {
		t.Errorf("last event: got %q, want message_stop", last.name)
	}
	var ev messageStopEvent
	if err := json.Unmarshal([]byte(last.data), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Type != "message_stop" {
		t.Errorf("type: %q", ev.Type)
	}
}

func TestStream_TokenAccumulation(t *testing.T) {
	// "ab" → ceil(2/4)=1, "cde" → ceil(3/4)=1 → total=2
	// Events: message_start[0], content_block_start[1], delta[2], delta[3],
	//         content_block_stop[4], message_delta[5], message_stop[6]
	var buf bytes.Buffer
	_ = Stream(&buf, nil, "id", "m", makeChunks("ab", "cde"))
	events := parseEvents(buf.String())

	// message_delta is events[5] (2 delta events → offset by 1 vs 1-chunk case)
	var md messageDeltaEvent
	json.Unmarshal([]byte(events[5].data), &md) //nolint:errcheck
	if md.Usage.OutputTokens != 2 {
		t.Errorf("output_tokens: got %d, want 2", md.Usage.OutputTokens)
	}
}

func TestStream_EmptyChunksSkipped(t *testing.T) {
	ch := make(chan chat.StreamChunk, 3)
	ch <- chat.StreamChunk{Text: ""}
	ch <- chat.StreamChunk{Text: "hello"}
	ch <- chat.StreamChunk{Text: ""}
	close(ch)

	var buf bytes.Buffer
	_ = Stream(&buf, nil, "id", "m", ch)
	events := parseEvents(buf.String())

	// message_start, content_block_start, 1 delta, content_block_stop, message_delta, message_stop = 6
	if len(events) != 6 {
		t.Errorf("expected 6 events, got %d", len(events))
	}
}

func TestStream_FlushCalled(t *testing.T) {
	flushCount := 0
	flush := func() { flushCount++ }

	var buf bytes.Buffer
	_ = Stream(&buf, flush, "id", "m", makeChunks("a", "b"))

	// message_start + content_block_start + 2 deltas + content_block_stop + message_delta + message_stop = 7
	if flushCount != 7 {
		t.Errorf("expected 7 flushes, got %d", flushCount)
	}
}

func TestStream_ErrorChunk(t *testing.T) {
	ch := make(chan chat.StreamChunk, 2)
	ch <- chat.StreamChunk{Text: "partial"}
	ch <- chat.StreamChunk{Err: errTest("stream failed")}
	close(ch)

	var buf bytes.Buffer
	err := Stream(&buf, nil, "id", "m", ch)
	if err == nil || err.Error() != "stream failed" {
		t.Errorf("expected error, got %v", err)
	}
}

// ── Complete ──────────────────────────────────────────────────────────────────

func TestComplete_Basic(t *testing.T) {
	msg, err := Complete("msg_abc", "claude-3-5-sonnet", makeChunks("Hello", " world"))
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID != "msg_abc" {
		t.Errorf("id: %q", msg.ID)
	}
	if msg.Type != "message" {
		t.Errorf("type: %q", msg.Type)
	}
	if msg.Role != "assistant" {
		t.Errorf("role: %q", msg.Role)
	}
	if msg.Model != "claude-3-5-sonnet" {
		t.Errorf("model: %q", msg.Model)
	}
	if len(msg.Content) != 1 || msg.Content[0].Text != "Hello world" {
		t.Errorf("content: %+v", msg.Content)
	}
	if msg.StopReason != "end_turn" {
		t.Errorf("stop_reason: %q", msg.StopReason)
	}
	if msg.StopSequence != nil {
		t.Errorf("stop_sequence should be nil")
	}
	if msg.Usage.InputTokens != 0 {
		t.Errorf("input_tokens should be 0")
	}
}

func TestComplete_TokenEstimation(t *testing.T) {
	cases := []struct {
		text   string
		tokens int
	}{
		{"", 0},
		{"abc", 1},
		{"abcd", 1},
		{"abcde", 2},
		{"12345678", 2},
		{"123456789", 3},
	}
	for _, tc := range cases {
		msg, _ := Complete("id", "m", makeChunks(tc.text))
		if msg.Usage.OutputTokens != tc.tokens {
			t.Errorf("text=%q: got %d, want %d", tc.text, msg.Usage.OutputTokens, tc.tokens)
		}
	}
}

func TestComplete_JSONFieldOrder(t *testing.T) {
	msg, _ := Complete("msg_abc", "mymodel", makeChunks("hi"))
	b, _ := json.Marshal(msg)
	raw := string(b)

	checkOrder(t, raw, "\"id\"", "\"type\"")
	checkOrder(t, raw, "\"type\"", "\"role\"")
	checkOrder(t, raw, "\"role\"", "\"content\"")
	checkOrder(t, raw, "\"content\"", "\"model\"")
	checkOrder(t, raw, "\"model\"", "\"stop_reason\"")
}

func TestComplete_ErrorChunk(t *testing.T) {
	ch := make(chan chat.StreamChunk, 1)
	ch <- chat.StreamChunk{Err: errTest("provider down")}
	close(ch)

	_, err := Complete("id", "m", ch)
	if err == nil || err.Error() != "provider down" {
		t.Errorf("expected error, got %v", err)
	}
}

// ── ErrorJSON ─────────────────────────────────────────────────────────────────

func TestErrorJSON_Structure(t *testing.T) {
	b := ErrorJSON("bad auth", "authentication_error")
	var resp errorResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Type != "error" {
		t.Errorf("type: %q", resp.Type)
	}
	if resp.Error.Type != "authentication_error" {
		t.Errorf("error.type: %q", resp.Error.Type)
	}
	if resp.Error.Message != "bad auth" {
		t.Errorf("error.message: %q", resp.Error.Message)
	}
}

func TestErrorJSON_DefaultType(t *testing.T) {
	b := ErrorJSON("oops", "")
	var resp errorResponse
	json.Unmarshal(b, &resp) //nolint:errcheck
	if resp.Error.Type != "api_error" {
		t.Errorf("default type: got %q", resp.Error.Type)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

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
