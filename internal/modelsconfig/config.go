package modelsconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
)

// ProviderModelConfig holds model configuration for one provider.
type ProviderModelConfig struct {
	Default        string   `json:"default"`
	EnableFallback bool     `json:"enableFallback"`
	Models         []string `json:"models"`
}

// ModelsConfig maps provider key → ProviderModelConfig.
// It is the exact structure of models.json (and MODELS_CONFIG env var value).
type ModelsConfig map[string]ProviderModelConfig

// ConfigPath is the filesystem path to models.json.
// Override in tests to point to a temporary copy.
var ConfigPath = "models.json"

// Load returns the active models configuration.
//
// Priority (CRITICAL — no hybrid state):
//  1. If MODELS_CONFIG env var is non-empty, parse it once and cache it.
//     The file is NEVER read in this case.
//  2. Otherwise read models.json with a 1-second TTL file cache.
func Load() (ModelsConfig, error) {
	envJSON := os.Getenv("MODELS_CONFIG")
	fromEnv := envJSON != ""

	// Fast path: read lock.
	if hit, ok := cacheRead(fromEnv); ok {
		return hit, nil
	}

	// Slow path: acquire write lock, double-check, then load from source.
	cacheMu.Lock()
	defer cacheMu.Unlock()

	// Double-check now that we hold the write lock.
	if hit, ok := cacheReadLocked(fromEnv); ok {
		return hit, nil
	}

	var (
		cfg ModelsConfig
		err error
	)
	if fromEnv {
		cfg, err = parseEnvJSON(envJSON)
	} else {
		cfg, err = readFile()
	}
	if err != nil {
		return nil, err
	}

	cacheWrite(cfg, fromEnv)
	return deepCopy(cfg), nil
}

// Save writes cfg to models.json (2-space pretty-printed JSON + trailing newline)
// and invalidates the in-memory cache.
func Save(cfg ModelsConfig) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("modelsconfig: marshal: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(ConfigPath, data, 0o644); err != nil {
		return fmt.Errorf("modelsconfig: write %s: %w", ConfigPath, err)
	}
	InvalidateCache()
	return nil
}

// Validate checks that all provider configs satisfy structural constraints:
//   - default is non-empty and present in models
//   - models is non-empty and contains no empty strings
func Validate(cfg ModelsConfig) error {
	for key, pc := range cfg {
		if pc.Default == "" {
			return fmt.Errorf("modelsconfig: provider %q: default is empty", key)
		}
		if len(pc.Models) == 0 {
			return fmt.Errorf("modelsconfig: provider %q: models is empty", key)
		}
		for _, m := range pc.Models {
			if m == "" {
				return fmt.Errorf("modelsconfig: provider %q: models contains empty string", key)
			}
		}
		if !slices.Contains(pc.Models, pc.Default) {
			return fmt.Errorf("modelsconfig: provider %q: default %q is not in models", key, pc.Default)
		}
	}
	return nil
}

// ─── CRUD ────────────────────────────────────────────────────────────────────

// AddModel appends model to a provider's models list.
// Errors if the provider doesn't exist or the model is already present.
func AddModel(providerKey, model string) error {
	cfg, err := loadForCRUD()
	if err != nil {
		return err
	}
	pc, ok := cfg[providerKey]
	if !ok {
		return fmt.Errorf("modelsconfig: provider %q not found", providerKey)
	}
	if slices.Contains(pc.Models, model) {
		return fmt.Errorf("modelsconfig: provider %q already has model %q", providerKey, model)
	}
	pc.Models = append(pc.Models, model)
	cfg[providerKey] = pc
	return Save(cfg)
}

// RemoveModel deletes model from a provider's models list.
// Cannot remove the default model or the last remaining model.
func RemoveModel(providerKey, model string) error {
	cfg, err := loadForCRUD()
	if err != nil {
		return err
	}
	pc, ok := cfg[providerKey]
	if !ok {
		return fmt.Errorf("modelsconfig: provider %q not found", providerKey)
	}
	idx := slices.Index(pc.Models, model)
	if idx == -1 {
		return fmt.Errorf("modelsconfig: model %q not found in provider %q", model, providerKey)
	}
	if pc.Default == model {
		return fmt.Errorf("modelsconfig: cannot remove default model %q from %q; set a new default first", model, providerKey)
	}
	if len(pc.Models) == 1 {
		return fmt.Errorf("modelsconfig: cannot remove last model from provider %q", providerKey)
	}
	pc.Models = slices.Delete(pc.Models, idx, idx+1)
	cfg[providerKey] = pc
	return Save(cfg)
}

// SetDefaultModel sets the default model for a provider.
// The model must already be in the provider's models list.
func SetDefaultModel(providerKey, model string) error {
	cfg, err := loadForCRUD()
	if err != nil {
		return err
	}
	pc, ok := cfg[providerKey]
	if !ok {
		return fmt.Errorf("modelsconfig: provider %q not found", providerKey)
	}
	if !slices.Contains(pc.Models, model) {
		return fmt.Errorf("modelsconfig: model %q is not in provider %q models", model, providerKey)
	}
	pc.Default = model
	cfg[providerKey] = pc
	return Save(cfg)
}

// ToggleFallback flips the enableFallback flag for a provider.
// Returns the new value.
func ToggleFallback(providerKey string) (bool, error) {
	cfg, err := loadForCRUD()
	if err != nil {
		return false, err
	}
	pc, ok := cfg[providerKey]
	if !ok {
		return false, fmt.Errorf("modelsconfig: provider %q not found", providerKey)
	}
	pc.EnableFallback = !pc.EnableFallback
	cfg[providerKey] = pc
	if err := Save(cfg); err != nil {
		return false, err
	}
	return pc.EnableFallback, nil
}

// ReorderModels replaces a provider's model list with orderedModels.
// orderedModels must be a permutation (same elements, any order) of the existing list.
func ReorderModels(providerKey string, orderedModels []string) error {
	cfg, err := loadForCRUD()
	if err != nil {
		return err
	}
	pc, ok := cfg[providerKey]
	if !ok {
		return fmt.Errorf("modelsconfig: provider %q not found", providerKey)
	}
	if len(orderedModels) != len(pc.Models) {
		return fmt.Errorf("modelsconfig: reorder length mismatch: got %d, want %d", len(orderedModels), len(pc.Models))
	}
	existing := make(map[string]bool, len(pc.Models))
	for _, m := range pc.Models {
		existing[m] = true
	}
	for _, m := range orderedModels {
		if !existing[m] {
			return fmt.Errorf("modelsconfig: %q is not an existing model for provider %q", m, providerKey)
		}
	}
	pc.Models = orderedModels
	cfg[providerKey] = pc
	return Save(cfg)
}

// UpdateModel renames oldModel to newModel within a provider.
// If oldModel was the default, the default is updated to newModel.
// No-op if oldModel == newModel.
func UpdateModel(providerKey, oldModel, newModel string) error {
	if oldModel == newModel {
		return nil
	}
	cfg, err := loadForCRUD()
	if err != nil {
		return err
	}
	pc, ok := cfg[providerKey]
	if !ok {
		return fmt.Errorf("modelsconfig: provider %q not found", providerKey)
	}
	idx := slices.Index(pc.Models, oldModel)
	if idx == -1 {
		return fmt.Errorf("modelsconfig: model %q not found in provider %q", oldModel, providerKey)
	}
	if slices.Contains(pc.Models, newModel) {
		return fmt.Errorf("modelsconfig: model %q already exists in provider %q", newModel, providerKey)
	}
	pc.Models[idx] = newModel
	if pc.Default == oldModel {
		pc.Default = newModel
	}
	cfg[providerKey] = pc
	return Save(cfg)
}

// EnsureProvider adds the provider with the given config if it does not exist.
// Idempotent: if the provider already exists, it is left unchanged.
func EnsureProvider(providerKey string, pc ProviderModelConfig) error {
	cfg, err := loadForCRUD()
	if err != nil {
		return err
	}
	if _, ok := cfg[providerKey]; ok {
		return nil
	}
	if err := Validate(ModelsConfig{providerKey: pc}); err != nil {
		return err
	}
	cfg[providerKey] = pc
	return Save(cfg)
}

// GetConfiguredProviders returns a sorted list of all provider keys in the
// active configuration (file or env).
func GetConfiguredProviders() ([]string, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// ─── internal helpers ────────────────────────────────────────────────────────

// loadForCRUD reads directly from the file, bypassing the cache.
// CRUD operations always operate on the file to avoid stale reads before saves.
func loadForCRUD() (ModelsConfig, error) {
	return readFile()
}

// readFile reads and parses models.json from ConfigPath.
func readFile() (ModelsConfig, error) {
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("modelsconfig: read %s: %w", ConfigPath, err)
	}
	var cfg ModelsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("modelsconfig: parse %s: %w", ConfigPath, err)
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// parseEnvJSON parses and validates a JSON string from the MODELS_CONFIG env var.
func parseEnvJSON(jsonStr string) (ModelsConfig, error) {
	var cfg ModelsConfig
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return nil, fmt.Errorf("modelsconfig: parse MODELS_CONFIG: %w", err)
	}
	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("modelsconfig: validate MODELS_CONFIG: %w", err)
	}
	return cfg, nil
}

