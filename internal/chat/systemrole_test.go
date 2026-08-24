package chat

import (
	"strings"
	"testing"

	"github.com/thotenn/aicarousel/internal/config"
)

// withModelsWithoutSystemRole temporarily overrides the configured fragments.
func withModelsWithoutSystemRole(t *testing.T, fragments []string) {
	t.Helper()
	original := config.Cfg.ModelsWithoutSystemRole
	config.Cfg.ModelsWithoutSystemRole = fragments
	t.Cleanup(func() { config.Cfg.ModelsWithoutSystemRole = original })
}

func TestModelSupportsSystemRole(t *testing.T) {
	withModelsWithoutSystemRole(t, nil) // nil = defaults apply

	tests := []struct {
		model string
		want  bool
	}{
		{"gemma2:2b", false},
		{"gemma3:1b", false},
		{"gemma-3-12b", false},
		{"GEMMA2:9B", false},
		{"llama3.2:latest", true},
		{"qwen2.5:3b", true},
		{"gemini-2.5-flash-lite", true},
		{"", true},
	}

	for _, tt := range tests {
		if got := ModelSupportsSystemRole(tt.model); got != tt.want {
			t.Errorf("ModelSupportsSystemRole(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestModelSupportsSystemRole_ConfigOverride(t *testing.T) {
	withModelsWithoutSystemRole(t, []string{"phi", "custom-model"})

	if !ModelSupportsSystemRole("gemma2:2b") {
		t.Error("gemma should support the system role once the defaults are replaced")
	}
	if ModelSupportsSystemRole("phi3:mini") {
		t.Error("phi3:mini should be detected as lacking a system role")
	}
}

func TestModelSupportsSystemRole_DisabledWithEmptyList(t *testing.T) {
	withModelsWithoutSystemRole(t, []string{})

	if !ModelSupportsSystemRole("gemma2:2b") {
		t.Error("an empty (non-nil) list must disable the adaptation")
	}
}

func TestMergeSystemIntoFirstUser(t *testing.T) {
	withModelsWithoutSystemRole(t, nil)

	msgs := []ChatMessage{
		{Role: "system", Content: "You are Thotenn."},
		{Role: "user", Content: "hola"},
		{Role: "assistant", Content: "todo bien"},
		{Role: "user", Content: "y vos?"},
	}

	got := MergeSystemIntoFirstUser(msgs)

	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for _, m := range got {
		if m.Role == "system" {
			t.Fatal("no system message may survive the merge")
		}
	}
	if got[0].Role != "user" {
		t.Errorf("first role = %q, want user", got[0].Role)
	}
	if !strings.Contains(got[0].Content, "SYSTEM INSTRUCTIONS") ||
		!strings.Contains(got[0].Content, "You are Thotenn.") ||
		!strings.Contains(got[0].Content, "END OF SYSTEM INSTRUCTIONS") {
		t.Errorf("merged content missing delimiters or instructions: %q", got[0].Content)
	}
	if !strings.HasSuffix(got[0].Content, "hola") {
		t.Errorf("merged content must end with the original user text, got %q", got[0].Content)
	}
	// Later turns untouched.
	if got[1] != (ChatMessage{Role: "assistant", Content: "todo bien"}) {
		t.Errorf("assistant turn altered: %+v", got[1])
	}
	if got[2] != (ChatMessage{Role: "user", Content: "y vos?"}) {
		t.Errorf("second user turn altered: %+v", got[2])
	}
}

func TestMergeSystemIntoFirstUser_DoesNotMutateInput(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "system", Content: "You are Thotenn."},
		{Role: "user", Content: "hola"},
	}

	MergeSystemIntoFirstUser(msgs)

	if msgs[0].Role != "system" || msgs[1].Content != "hola" {
		t.Errorf("input slice was mutated: %+v", msgs)
	}
}

func TestMergeSystemIntoFirstUser_MultipleSystemMessages(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "system", Content: "First."},
		{Role: "system", Content: "Second."},
		{Role: "user", Content: "hola"},
	}

	got := MergeSystemIntoFirstUser(msgs)

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if strings.Index(got[0].Content, "First.") > strings.Index(got[0].Content, "Second.") {
		t.Error("system messages must keep their original order")
	}
}

func TestMergeSystemIntoFirstUser_NoSystemMessage(t *testing.T) {
	msgs := []ChatMessage{{Role: "user", Content: "hola"}}

	got := MergeSystemIntoFirstUser(msgs)

	if len(got) != 1 || got[0] != msgs[0] {
		t.Errorf("messages without a system entry must be returned untouched, got %+v", got)
	}
}

func TestMergeSystemIntoFirstUser_EmptySystemMessage(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "system", Content: "   "},
		{Role: "user", Content: "hola"},
	}

	got := MergeSystemIntoFirstUser(msgs)

	if len(got) != 2 || got[0].Role != "system" {
		t.Errorf("blank system messages must not trigger a merge, got %+v", got)
	}
}

func TestMergeSystemIntoFirstUser_NoUserTurn(t *testing.T) {
	msgs := []ChatMessage{{Role: "system", Content: "You are Thotenn."}}

	got := MergeSystemIntoFirstUser(msgs)

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Role != "user" || !strings.Contains(got[0].Content, "You are Thotenn.") {
		t.Errorf("instructions must become a user turn, got %+v", got[0])
	}
}

func TestAdaptMessagesForModel(t *testing.T) {
	withModelsWithoutSystemRole(t, nil)

	msgs := []ChatMessage{
		{Role: "system", Content: "You are Thotenn."},
		{Role: "user", Content: "hola"},
	}

	kept := AdaptMessagesForModel(msgs, "llama3.2:latest")
	if len(kept) != 2 || kept[0].Role != "system" {
		t.Errorf("system role must survive for supporting models, got %+v", kept)
	}

	merged := AdaptMessagesForModel(msgs, "gemma2:2b")
	if len(merged) != 1 || merged[0].Role != "user" {
		t.Errorf("system role must be merged for gemma, got %+v", merged)
	}
}
