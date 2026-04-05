package sseparse_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/thotenn/aicarousel-go/internal/providers/sseparse"
)

type event struct {
	name string
	data string
}

func collect(input string) ([]event, error) {
	var events []event
	err := sseparse.ForEachEvent(strings.NewReader(input), func(name string, data []byte) error {
		events = append(events, event{name: name, data: string(data)})
		return nil
	})
	return events, err
}

// ─── basic parsing ────────────────────────────────────────────────────────────

func TestForEachEvent_SimpleData(t *testing.T) {
	input := "data: hello\n\ndata: world\n\n"
	got, err := collect(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d: %v", len(got), got)
	}
	if got[0].data != "hello" || got[1].data != "world" {
		t.Errorf("unexpected data: %v", got)
	}
}

func TestForEachEvent_NamedEvent(t *testing.T) {
	input := "event: content_block_delta\ndata: some text\n\n"
	got, err := collect(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].name != "content_block_delta" {
		t.Errorf("event name: got %q, want %q", got[0].name, "content_block_delta")
	}
	if got[0].data != "some text" {
		t.Errorf("data: got %q, want %q", got[0].data, "some text")
	}
}

func TestForEachEvent_DoneTerminator(t *testing.T) {
	input := "data: first\n\ndata: [DONE]\n\n"
	got, err := collect(input)
	if err != nil {
		t.Fatal(err)
	}
	// [DONE] must not be passed to fn; only "first" should appear.
	if len(got) != 1 {
		t.Fatalf("expected 1 event before [DONE], got %d: %v", len(got), got)
	}
	if got[0].data != "first" {
		t.Errorf("got %q, want %q", got[0].data, "first")
	}
}

func TestForEachEvent_MultiLineData(t *testing.T) {
	input := "data: line1\ndata: line2\ndata: line3\n\n"
	got, err := collect(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	want := "line1\nline2\nline3"
	if got[0].data != want {
		t.Errorf("got %q, want %q", got[0].data, want)
	}
}

func TestForEachEvent_Comments_Ignored(t *testing.T) {
	input := ": this is a comment\ndata: real\n\n"
	got, err := collect(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].data != "real" {
		t.Errorf("unexpected: %v", got)
	}
}

func TestForEachEvent_TrailingEventNoBlankLine(t *testing.T) {
	// No final blank line; event should still be dispatched.
	input := "data: trailing"
	got, err := collect(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].data != "trailing" {
		t.Errorf("unexpected: %v", got)
	}
}

func TestForEachEvent_EmptyInput(t *testing.T) {
	got, err := collect("")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected no events, got %v", got)
	}
}

// ─── fn return values ─────────────────────────────────────────────────────────

func TestForEachEvent_FnReturnsEOF_StopsCleanly(t *testing.T) {
	input := "data: a\n\ndata: b\n\ndata: c\n\n"
	var seen []string
	err := sseparse.ForEachEvent(strings.NewReader(input), func(_ string, data []byte) error {
		seen = append(seen, string(data))
		if string(data) == "a" {
			return io.EOF
		}
		return nil
	})
	if err != nil {
		t.Errorf("expected nil on io.EOF from fn, got %v", err)
	}
	if len(seen) != 1 {
		t.Errorf("expected fn called once (stopped at EOF), got %d calls", len(seen))
	}
}

func TestForEachEvent_FnReturnsError_Propagated(t *testing.T) {
	sentinel := errors.New("parse error")
	input := "data: x\n\n"
	err := sseparse.ForEachEvent(strings.NewReader(input), func(_ string, _ []byte) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

// ─── real-world SSE shapes ────────────────────────────────────────────────────

func TestForEachEvent_OpenAIShape(t *testing.T) {
	input := "" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n" +
		"data: [DONE]\n\n"

	var parts []string
	err := sseparse.ForEachEvent(strings.NewReader(input), func(_ string, data []byte) error {
		parts = append(parts, string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 data events, got %d", len(parts))
	}
}

func TestForEachEvent_AnthropicShape(t *testing.T) {
	input := "" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n"

	var events []event
	err := sseparse.ForEachEvent(strings.NewReader(input), func(name string, data []byte) error {
		events = append(events, event{name: name, data: string(data)})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].name != "content_block_delta" {
		t.Errorf("first event name: got %q", events[0].name)
	}
}
