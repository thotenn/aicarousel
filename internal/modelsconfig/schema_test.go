package modelsconfig_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/thotenn/aicarousel/internal/modelsconfig"
)

// ─── schema loading ───────────────────────────────────────────────────────────

// schemaContent is the raw text of schema.json, loaded once at test start.
// Working directory during `go test` is the package directory.
var schemaContent string

// realModelsContent is the raw text of the project's models.json.
var realModelsContent string

func init() {
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			panic("schema_test: cannot read " + path + ": " + err.Error())
		}
		return string(b)
	}
	schemaContent = read("schema.json")
	realModelsContent = read("../../models.json")
}

func newValidator(t *testing.T) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft7

	const id = "schema.json"
	if err := compiler.AddResource(id, strings.NewReader(schemaContent)); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	s, err := compiler.Compile(id)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return s
}

func validateJSON(t *testing.T, s *jsonschema.Schema, raw string) error {
	t.Helper()
	var v interface{}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("invalid JSON in fixture: %v", err)
	}
	return s.Validate(v)
}

// ─── real models.json ─────────────────────────────────────────────────────────

func TestSchema_realModelsJSON(t *testing.T) {
	s := newValidator(t)
	if err := validateJSON(t, s, realModelsContent); err != nil {
		t.Errorf("models.json fails schema validation: %v", err)
	}
}

// ─── valid fixtures ───────────────────────────────────────────────────────────

func TestSchema_validFixtures(t *testing.T) {
	s := newValidator(t)

	cases := []struct {
		name string
		raw  string
	}{
		{
			name: "empty object",
			raw:  `{}`,
		},
		{
			name: "minimal single provider",
			raw:  `{"p":{"default":"m","enableFallback":false,"models":["m"]}}`,
		},
		{
			name: "multiple providers",
			raw: `{
				"cerebras":{"default":"a","enableFallback":true,"models":["a","b"]},
				"groq":{"default":"fast","enableFallback":false,"models":["fast"]}
			}`,
		},
		{
			name: "enableFallback true with multiple models",
			raw:  `{"p":{"default":"first","enableFallback":true,"models":["first","second","third"]}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateJSON(t, s, tc.raw); err != nil {
				t.Errorf("expected valid, got: %v", err)
			}
		})
	}
}

// ─── invalid fixtures ─────────────────────────────────────────────────────────

func TestSchema_invalidFixtures(t *testing.T) {
	s := newValidator(t)

	cases := []struct {
		name string
		raw  string
	}{
		{"missing default", `{"p":{"enableFallback":false,"models":["m"]}}`},
		{"missing enableFallback", `{"p":{"default":"m","models":["m"]}}`},
		{"missing models", `{"p":{"default":"m","enableFallback":false}}`},
		{"empty models array", `{"p":{"default":"m","enableFallback":false,"models":[]}}`},
		{"default is empty string", `{"p":{"default":"","enableFallback":false,"models":["m"]}}`},
		{"model item is empty string", `{"p":{"default":"m","enableFallback":false,"models":["m",""]}}`},
		{"default is number", `{"p":{"default":42,"enableFallback":false,"models":["m"]}}`},
		{"enableFallback is string", `{"p":{"default":"m","enableFallback":"yes","models":["m"]}}`},
		{"models is string", `{"p":{"default":"m","enableFallback":false,"models":"m"}}`},
		{"models item is number", `{"p":{"default":"m","enableFallback":false,"models":[42]}}`},
		{"extra property in provider", `{"p":{"default":"m","enableFallback":false,"models":["m"],"extra":"field"}}`},
		{"duplicate models (uniqueItems)", `{"p":{"default":"m","enableFallback":false,"models":["m","m"]}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateJSON(t, s, tc.raw); err == nil {
				t.Errorf("expected validation error for %q", tc.name)
			}
		})
	}
}

// ─── Go struct round-trip for valid fixtures ──────────────────────────────────

func TestSchema_validFixtures_structRoundTrip(t *testing.T) {
	type providerCfg struct {
		Default        string   `json:"default"`
		EnableFallback bool     `json:"enableFallback"`
		Models         []string `json:"models"`
	}

	cases := []struct {
		name         string
		raw          string
		wantKey      string
		wantDefault  string
		wantFallback bool
		wantModels   []string
	}{
		{
			name:         "fallback true",
			raw:          `{"prov":{"default":"m1","enableFallback":true,"models":["m1","m2"]}}`,
			wantKey:      "prov",
			wantDefault:  "m1",
			wantFallback: true,
			wantModels:   []string{"m1", "m2"},
		},
		{
			name:         "fallback false",
			raw:          `{"groq":{"default":"fast","enableFallback":false,"models":["fast","slow"]}}`,
			wantKey:      "groq",
			wantDefault:  "fast",
			wantFallback: false,
			wantModels:   []string{"fast", "slow"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg map[string]providerCfg
			if err := json.Unmarshal([]byte(tc.raw), &cfg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			pc, ok := cfg[tc.wantKey]
			if !ok {
				t.Fatalf("key %q not found", tc.wantKey)
			}
			if pc.Default != tc.wantDefault {
				t.Errorf("default: got %q, want %q", pc.Default, tc.wantDefault)
			}
			if pc.EnableFallback != tc.wantFallback {
				t.Errorf("enableFallback: got %v, want %v", pc.EnableFallback, tc.wantFallback)
			}
			if len(pc.Models) != len(tc.wantModels) {
				t.Errorf("models len: got %d, want %d", len(pc.Models), len(tc.wantModels))
			}
		})
	}
}

// ─── ensure modelsconfig package types are also exercised ─────────────────────

func TestSchema_modelsConfigTypeRoundTrip(t *testing.T) {
	raw := realModelsContent
	cfg, err := modelsconfig.Load()
	// Load may fail if ../../models.json path differs at test time; use raw JSON directly.
	if err != nil {
		// Fall back: parse raw JSON into the package type.
		var m modelsconfig.ModelsConfig
		if err2 := json.Unmarshal([]byte(raw), &m); err2 != nil {
			t.Fatalf("unmarshal real models.json: %v", err2)
		}
		if err3 := modelsconfig.Validate(m); err3 != nil {
			t.Fatalf("Validate real models.json: %v", err3)
		}
		return
	}
	if len(cfg) == 0 {
		t.Error("expected non-empty config from models.json")
	}
	if err := modelsconfig.Validate(cfg); err != nil {
		t.Errorf("Validate(Load()): %v", err)
	}
}
