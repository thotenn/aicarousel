package modelsconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ─── test helpers ────────────────────────────────────────────────────────────

// withTempConfig writes cfg to a temp file, points ConfigPath at it, and
// resets the cache and ConfigPath after the test.
func withTempConfig(t *testing.T, cfg ModelsConfig) {
	t.Helper()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal temp config: %v", err)
	}
	f, err := os.CreateTemp(t.TempDir(), "models*.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close() //nolint:errcheck
		t.Fatal(err)
	}
	f.Close() //nolint:errcheck

	prev := ConfigPath
	ConfigPath = f.Name()
	t.Cleanup(func() {
		ConfigPath = prev
		InvalidateCache()
	})
	InvalidateCache()
}

// sampleConfig returns a minimal but valid ModelsConfig for tests.
func sampleConfig() ModelsConfig {
	return ModelsConfig{
		"provA": {Default: "model-1", EnableFallback: true, Models: []string{"model-1", "model-2"}},
		"provB": {Default: "model-x", EnableFallback: false, Models: []string{"model-x"}},
	}
}

// ─── Validate ────────────────────────────────────────────────────────────────

func TestValidate_valid(t *testing.T) {
	if err := Validate(sampleConfig()); err != nil {
		t.Errorf("unexpected error on valid config: %v", err)
	}
}

func TestValidate_emptyDefault(t *testing.T) {
	cfg := ModelsConfig{"p": {Default: "", EnableFallback: false, Models: []string{"m"}}}
	if err := Validate(cfg); err == nil {
		t.Error("expected error for empty default")
	}
}

func TestValidate_emptyModels(t *testing.T) {
	cfg := ModelsConfig{"p": {Default: "m", EnableFallback: false, Models: nil}}
	if err := Validate(cfg); err == nil {
		t.Error("expected error for empty models")
	}
}

func TestValidate_defaultNotInModels(t *testing.T) {
	cfg := ModelsConfig{"p": {Default: "not-here", Models: []string{"model-a"}}}
	if err := Validate(cfg); err == nil {
		t.Error("expected error when default not in models")
	}
}

func TestValidate_emptyModelName(t *testing.T) {
	cfg := ModelsConfig{"p": {Default: "a", Models: []string{"a", ""}}}
	if err := Validate(cfg); err == nil {
		t.Error("expected error for empty model name in list")
	}
}

// ─── Load / Cache ────────────────────────────────────────────────────────────

func TestLoad_fromFile(t *testing.T) {
	withTempConfig(t, sampleConfig())
	t.Setenv("MODELS_CONFIG", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg) != 2 {
		t.Errorf("expected 2 providers, got %d", len(cfg))
	}
}

func TestLoad_cacheHit(t *testing.T) {
	withTempConfig(t, sampleConfig())
	t.Setenv("MODELS_CONFIG", "")

	_, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	// Delete the file to prove the second call uses the cache.
	os.Remove(ConfigPath) //nolint:errcheck

	cfg2, err := Load()
	if err != nil {
		t.Fatalf("second Load (from cache) failed: %v", err)
	}
	if len(cfg2) == 0 {
		t.Error("expected non-empty cached config")
	}
}

func TestLoad_cacheTTLExpiry(t *testing.T) {
	withTempConfig(t, sampleConfig())
	t.Setenv("MODELS_CONFIG", "")

	if _, err := Load(); err != nil {
		t.Fatal(err)
	}

	// Force cache expiry.
	cacheMu.Lock()
	cacheAt = time.Now().Add(-2 * cacheTTL)
	cacheMu.Unlock()

	// Update the file with different content.
	newCfg := ModelsConfig{
		"provZ": {Default: "z-model", Models: []string{"z-model"}, EnableFallback: false},
	}
	data, _ := json.MarshalIndent(newCfg, "", "  ")
	os.WriteFile(ConfigPath, data, 0o644) //nolint:errcheck

	cfg2, err := Load()
	if err != nil {
		t.Fatalf("Load after TTL expiry: %v", err)
	}
	if _, ok := cfg2["provZ"]; !ok {
		t.Error("expected fresh config after TTL expiry, still got stale data")
	}
}

func TestLoad_envOverrideBypassesFile(t *testing.T) {
	withTempConfig(t, sampleConfig())

	envCfg := `{"envProv":{"default":"env-model","enableFallback":false,"models":["env-model"]}}`
	t.Setenv("MODELS_CONFIG", envCfg)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with MODELS_CONFIG: %v", err)
	}
	if _, ok := cfg["envProv"]; !ok {
		t.Error("expected env provider in config")
	}
	// File providers must NOT appear.
	if _, ok := cfg["provA"]; ok {
		t.Error("file providers must not appear when MODELS_CONFIG env is set (no hybrid state)")
	}
}

func TestLoad_envOverride_noFileRead(t *testing.T) {
	// Point ConfigPath at a non-existent file — if Load touches it, it would fail.
	ConfigPath = filepath.Join(t.TempDir(), "nonexistent.json")
	t.Cleanup(func() { ConfigPath = "models.json"; InvalidateCache() })
	InvalidateCache()

	envCfg := `{"p":{"default":"m","enableFallback":false,"models":["m"]}}`
	t.Setenv("MODELS_CONFIG", envCfg)

	if _, err := Load(); err != nil {
		t.Fatalf("Load should not read file when MODELS_CONFIG is set, got: %v", err)
	}
}

func TestLoad_envOverride_invalidJSON(t *testing.T) {
	t.Setenv("MODELS_CONFIG", "not-json")
	InvalidateCache()
	t.Cleanup(InvalidateCache)

	if _, err := Load(); err == nil {
		t.Error("expected error for invalid MODELS_CONFIG JSON")
	}
}

func TestLoad_envOverride_invalidConfig(t *testing.T) {
	t.Setenv("MODELS_CONFIG", `{"p":{"default":"missing","enableFallback":false,"models":["m"]}}`)
	InvalidateCache()
	t.Cleanup(InvalidateCache)

	if _, err := Load(); err == nil {
		t.Error("expected error: default not in models")
	}
}

func TestLoad_returnsCopy(t *testing.T) {
	withTempConfig(t, sampleConfig())
	t.Setenv("MODELS_CONFIG", "")

	cfg1, _ := Load()
	cfg1["provA"] = ProviderModelConfig{Default: "mutated", Models: []string{"mutated"}}

	cfg2, _ := Load()
	if cfg2["provA"].Default == "mutated" {
		t.Error("Load should return a copy; mutation of result should not affect cache")
	}
}

// ─── Save ────────────────────────────────────────────────────────────────────

func TestSave_writesToFile(t *testing.T) {
	withTempConfig(t, sampleConfig())
	t.Setenv("MODELS_CONFIG", "")

	newCfg := ModelsConfig{
		"saved": {Default: "saved-model", EnableFallback: true, Models: []string{"saved-model"}},
	}
	if err := Save(newCfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var loaded ModelsConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded["saved"]; !ok {
		t.Error("saved provider not found in file after Save")
	}
}

func TestSave_invalidatesCache(t *testing.T) {
	withTempConfig(t, sampleConfig())
	t.Setenv("MODELS_CONFIG", "")

	Load() //nolint:errcheck // populate cache

	newCfg := ModelsConfig{
		"fresh": {Default: "fresh-model", Models: []string{"fresh-model"}},
	}
	Save(newCfg) //nolint:errcheck

	cfg, _ := Load()
	if _, ok := cfg["fresh"]; !ok {
		t.Error("expected fresh config after Save+Load")
	}
}

func TestSave_rejectsInvalidConfig(t *testing.T) {
	withTempConfig(t, sampleConfig())
	bad := ModelsConfig{"p": {Default: "missing", Models: []string{"other"}}}
	if err := Save(bad); err == nil {
		t.Error("Save should reject config where default not in models")
	}
}

// ─── CRUD ────────────────────────────────────────────────────────────────────

func TestAddModel(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := AddModel("provA", "model-3"); err != nil {
		t.Fatalf("AddModel: %v", err)
	}
	cfg, _ := Load()
	if !containsStr(cfg["provA"].Models, "model-3") {
		t.Error("model-3 not added")
	}
}

func TestAddModel_duplicate(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := AddModel("provA", "model-1"); err == nil {
		t.Error("expected error adding duplicate model")
	}
}

func TestAddModel_unknownProvider(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := AddModel("nonexistent", "m"); err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestRemoveModel(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := RemoveModel("provA", "model-2"); err != nil {
		t.Fatalf("RemoveModel: %v", err)
	}
	cfg, _ := Load()
	if containsStr(cfg["provA"].Models, "model-2") {
		t.Error("model-2 still present after removal")
	}
}

func TestRemoveModel_default(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := RemoveModel("provA", "model-1"); err == nil {
		t.Error("expected error removing default model")
	}
}

func TestRemoveModel_last(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := RemoveModel("provB", "model-x"); err == nil {
		t.Error("expected error removing last model")
	}
}

func TestRemoveModel_notFound(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := RemoveModel("provA", "ghost"); err == nil {
		t.Error("expected error for non-existent model")
	}
}

func TestSetDefaultModel(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := SetDefaultModel("provA", "model-2"); err != nil {
		t.Fatalf("SetDefaultModel: %v", err)
	}
	cfg, _ := Load()
	if cfg["provA"].Default != "model-2" {
		t.Errorf("default: got %q, want model-2", cfg["provA"].Default)
	}
}

func TestSetDefaultModel_notInList(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := SetDefaultModel("provA", "ghost"); err == nil {
		t.Error("expected error for model not in list")
	}
}

func TestToggleFallback(t *testing.T) {
	withTempConfig(t, sampleConfig())

	// provA starts with EnableFallback=true
	newVal, err := ToggleFallback("provA")
	if err != nil {
		t.Fatal(err)
	}
	if newVal {
		t.Error("expected false after toggling true→false")
	}

	newVal2, _ := ToggleFallback("provA")
	if !newVal2 {
		t.Error("expected true after toggling false→true")
	}
}

func TestToggleFallback_unknownProvider(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if _, err := ToggleFallback("ghost"); err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestReorderModels(t *testing.T) {
	withTempConfig(t, sampleConfig())
	newOrder := []string{"model-2", "model-1"}
	if err := ReorderModels("provA", newOrder); err != nil {
		t.Fatalf("ReorderModels: %v", err)
	}
	cfg, _ := Load()
	if cfg["provA"].Models[0] != "model-2" {
		t.Errorf("reorder: first model should be model-2, got %q", cfg["provA"].Models[0])
	}
}

func TestReorderModels_wrongLength(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := ReorderModels("provA", []string{"model-1"}); err == nil {
		t.Error("expected error for wrong length")
	}
}

func TestReorderModels_unknownModel(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := ReorderModels("provA", []string{"model-1", "ghost"}); err == nil {
		t.Error("expected error for unknown model in reorder list")
	}
}

func TestUpdateModel(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := UpdateModel("provA", "model-2", "model-2-renamed"); err != nil {
		t.Fatalf("UpdateModel: %v", err)
	}
	cfg, _ := Load()
	if containsStr(cfg["provA"].Models, "model-2") {
		t.Error("old model still present")
	}
	if !containsStr(cfg["provA"].Models, "model-2-renamed") {
		t.Error("renamed model not present")
	}
}

func TestUpdateModel_renamesDefault(t *testing.T) {
	withTempConfig(t, sampleConfig())
	// provA default = model-1; rename it
	if err := UpdateModel("provA", "model-1", "model-1-new"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := Load()
	if cfg["provA"].Default != "model-1-new" {
		t.Errorf("default not updated: got %q", cfg["provA"].Default)
	}
}

func TestUpdateModel_noop(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := UpdateModel("provA", "model-1", "model-1"); err != nil {
		t.Errorf("noop rename should not error: %v", err)
	}
}

func TestUpdateModel_collisionError(t *testing.T) {
	withTempConfig(t, sampleConfig())
	if err := UpdateModel("provA", "model-1", "model-2"); err == nil {
		t.Error("expected error renaming to existing model name")
	}
}

func TestEnsureProvider_addsNew(t *testing.T) {
	withTempConfig(t, sampleConfig())
	newPC := ProviderModelConfig{Default: "new-m", EnableFallback: false, Models: []string{"new-m"}}
	if err := EnsureProvider("provNew", newPC); err != nil {
		t.Fatalf("EnsureProvider: %v", err)
	}
	cfg, _ := Load()
	if _, ok := cfg["provNew"]; !ok {
		t.Error("provNew not present after EnsureProvider")
	}
}

func TestEnsureProvider_idempotent(t *testing.T) {
	withTempConfig(t, sampleConfig())
	orig, _ := Load()

	// Try to overwrite provA with different content.
	pc := ProviderModelConfig{Default: "different", Models: []string{"different"}}
	if err := EnsureProvider("provA", pc); err != nil {
		t.Fatal(err)
	}

	cfg, _ := Load()
	if cfg["provA"].Default != orig["provA"].Default {
		t.Error("EnsureProvider must not overwrite existing provider")
	}
}

func TestGetConfiguredProviders(t *testing.T) {
	withTempConfig(t, sampleConfig())
	t.Setenv("MODELS_CONFIG", "")

	providers, err := GetConfiguredProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 2 {
		t.Errorf("got %d providers, want 2", len(providers))
	}
	// Must be sorted.
	if providers[0] > providers[1] {
		t.Errorf("providers not sorted: %v", providers)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
