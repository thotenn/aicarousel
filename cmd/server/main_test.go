package main

import (
	"testing"

	"github.com/thotenn/aicarousel/internal/chat"
	"github.com/thotenn/aicarousel/internal/providers/provparams"
)

func TestApplyOptions_OverridesDefaults(t *testing.T) {
	stop := "<end>"
	opts := chat.Options{
		Temperature: chat.Float64Ptr(0.2),
		TopP:        chat.Float64Ptr(0.5),
		MaxTokens:   chat.IntPtr(500),
		Stop:        &stop,
	}

	got := applyOptions(provparams.DefaultParams("m1"), opts)

	if got.Temperature != 0.2 {
		t.Errorf("Temperature = %v, want 0.2", got.Temperature)
	}
	if got.TopP != 0.5 {
		t.Errorf("TopP = %v, want 0.5", got.TopP)
	}
	if got.MaxCompletionTokens != 500 {
		t.Errorf("MaxCompletionTokens = %v, want 500", got.MaxCompletionTokens)
	}
	if got.Stop == nil || *got.Stop != "<end>" {
		t.Errorf("Stop = %v, want %q", got.Stop, "<end>")
	}
	if got.Model != "m1" {
		t.Errorf("Model = %q, want m1", got.Model)
	}
}

func TestApplyOptions_KeepsDefaultsWhenUnset(t *testing.T) {
	defaults := provparams.DefaultParams("m1")

	got := applyOptions(defaults, chat.Options{})

	if got != defaults {
		t.Errorf("empty options must leave defaults untouched: got %+v, want %+v", got, defaults)
	}
}

func TestApplyOptions_PartialOverride(t *testing.T) {
	defaults := provparams.DefaultParams("m1")

	got := applyOptions(defaults, chat.Options{Temperature: chat.Float64Ptr(0.2)})

	if got.Temperature != 0.2 {
		t.Errorf("Temperature = %v, want 0.2", got.Temperature)
	}
	if got.TopP != defaults.TopP {
		t.Errorf("TopP = %v, want the default %v", got.TopP, defaults.TopP)
	}
	if got.MaxCompletionTokens != defaults.MaxCompletionTokens {
		t.Errorf("MaxCompletionTokens = %v, want the default %v",
			got.MaxCompletionTokens, defaults.MaxCompletionTokens)
	}
}
