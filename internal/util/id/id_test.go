package id

import (
	"strings"
	"testing"
)

func TestNewChatCmplID_prefix(t *testing.T) {
	id := NewChatCmplID()
	if !strings.HasPrefix(id, "chatcmpl-") {
		t.Errorf("expected prefix chatcmpl-, got %q", id)
	}
}

func TestNewChatCmplID_suffixLength(t *testing.T) {
	id := NewChatCmplID()
	suffix := strings.TrimPrefix(id, "chatcmpl-")
	if len(suffix) != 24 {
		t.Errorf("expected 24 hex chars suffix, got %d in %q", len(suffix), id)
	}
}

func TestNewChatCmplID_hexOnly(t *testing.T) {
	id := NewChatCmplID()
	suffix := strings.TrimPrefix(id, "chatcmpl-")
	for _, c := range suffix {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("non-hex char %q in suffix of %q", c, id)
		}
	}
}

func TestNewMsgID_prefix(t *testing.T) {
	id := NewMsgID()
	if !strings.HasPrefix(id, "msg_") {
		t.Errorf("expected prefix msg_, got %q", id)
	}
}

func TestNewMsgID_suffixLength(t *testing.T) {
	id := NewMsgID()
	suffix := strings.TrimPrefix(id, "msg_")
	if len(suffix) != 24 {
		t.Errorf("expected 24 hex chars suffix, got %d in %q", len(suffix), id)
	}
}

func TestNewMsgID_hexOnly(t *testing.T) {
	id := NewMsgID()
	suffix := strings.TrimPrefix(id, "msg_")
	for _, c := range suffix {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("non-hex char %q in suffix of %q", c, id)
		}
	}
}

func TestNewChatCmplID_uniqueness(t *testing.T) {
	const n = 1000
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		id := NewChatCmplID()
		if seen[id] {
			t.Fatalf("duplicate ID after %d iterations: %q", i, id)
		}
		seen[id] = true
	}
}

func TestNewMsgID_uniqueness(t *testing.T) {
	const n = 1000
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		id := NewMsgID()
		if seen[id] {
			t.Fatalf("duplicate ID after %d iterations: %q", i, id)
		}
		seen[id] = true
	}
}

func TestIDs_crossTypeUniqueness(t *testing.T) {
	// Different prefixes guarantee no collision between the two ID types.
	cid := NewChatCmplID()
	mid := NewMsgID()
	if cid == mid {
		t.Errorf("chatcmpl and msg IDs should never be equal: %q == %q", cid, mid)
	}
}
