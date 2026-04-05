package provsettings_test

import (
	"context"
	"testing"

	"github.com/thotenn/aicarousel/internal/db/provsettings"
	"github.com/thotenn/aicarousel/testutil"
)

var ctx = context.Background()

func newRepo(t *testing.T) provsettings.Repo {
	t.Helper()
	return provsettings.New(testutil.NewTestDB(t))
}

// --- All ---

func TestAll_returnsSortedByPriority(t *testing.T) {
	r := newRepo(t)
	settings, err := r.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	// Default migration inserts 4 providers.
	if len(settings) != 4 {
		t.Fatalf("All: got %d, want 4", len(settings))
	}
	for i := 1; i < len(settings); i++ {
		if settings[i].Priority < settings[i-1].Priority {
			t.Errorf("not sorted by priority: %v", settings)
		}
	}
}

// --- Get ---

func TestGet_existing(t *testing.T) {
	r := newRepo(t)
	s, err := r.Get(ctx, "cerebras")
	if err != nil || s == nil {
		t.Fatalf("Get cerebras: s=%v err=%v", s, err)
	}
	if s.ProviderKey != "cerebras" {
		t.Errorf("ProviderKey: got %q", s.ProviderKey)
	}
	if s.IsEnabled != 1 {
		t.Errorf("IsEnabled: got %d, want 1", s.IsEnabled)
	}
}

func TestGet_missing(t *testing.T) {
	r := newRepo(t)
	s, err := r.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if s != nil {
		t.Error("Get on missing key should return nil")
	}
}

// --- Enabled / EnabledKeys ---

func TestEnabled_allDefaultsAreEnabled(t *testing.T) {
	r := newRepo(t)
	settings, err := r.Enabled(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(settings) != 4 {
		t.Errorf("Enabled: got %d, want 4", len(settings))
	}
}

func TestEnabledKeys(t *testing.T) {
	r := newRepo(t)
	keys, err := r.EnabledKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 4 {
		t.Errorf("EnabledKeys: got %d, want 4", len(keys))
	}
}

// --- IsEnabled ---

func TestIsEnabled_existingEnabled(t *testing.T) {
	r := newRepo(t)
	ok, err := r.IsEnabled(ctx, "groq")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("groq should be enabled by default")
	}
}

func TestIsEnabled_missingRowTreatedAsEnabled(t *testing.T) {
	r := newRepo(t)
	ok, err := r.IsEnabled(ctx, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("missing row should be treated as enabled")
	}
}

func TestIsEnabled_afterToggleDisable(t *testing.T) {
	r := newRepo(t)
	if _, err := r.Toggle(ctx, "cerebras", false); err != nil {
		t.Fatal(err)
	}
	ok, err := r.IsEnabled(ctx, "cerebras")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("cerebras should be disabled after Toggle(false)")
	}
}

// --- Toggle ---

func TestToggle_disableAndReenable(t *testing.T) {
	r := newRepo(t)

	ok, err := r.Toggle(ctx, "groq", false)
	if err != nil || !ok {
		t.Fatalf("Toggle disable: ok=%v err=%v", ok, err)
	}
	s, _ := r.Get(ctx, "groq")
	if s.IsEnabled != 0 {
		t.Errorf("IsEnabled after disable: got %d", s.IsEnabled)
	}

	ok, err = r.Toggle(ctx, "groq", true)
	if err != nil || !ok {
		t.Fatalf("Toggle enable: ok=%v err=%v", ok, err)
	}
	s, _ = r.Get(ctx, "groq")
	if s.IsEnabled != 1 {
		t.Errorf("IsEnabled after re-enable: got %d", s.IsEnabled)
	}
}

func TestToggle_missingKey_returnsFalse(t *testing.T) {
	r := newRepo(t)
	ok, err := r.Toggle(ctx, "nonexistent", true)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("Toggle on missing key should return false")
	}
}

// --- UpdatePriority ---

func TestUpdatePriority(t *testing.T) {
	r := newRepo(t)
	ok, err := r.UpdatePriority(ctx, "cerebras", 99)
	if err != nil || !ok {
		t.Fatalf("UpdatePriority: ok=%v err=%v", ok, err)
	}
	s, _ := r.Get(ctx, "cerebras")
	if s.Priority != 99 {
		t.Errorf("Priority: got %d, want 99", s.Priority)
	}
}

// --- Reorder ---

func TestReorder(t *testing.T) {
	r := newRepo(t)
	order := []string{"gemini", "openrouter", "groq", "cerebras"}

	if err := r.Reorder(ctx, order); err != nil {
		t.Fatalf("Reorder: %v", err)
	}

	settings, err := r.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range order {
		if settings[i].ProviderKey != want {
			t.Errorf("after Reorder[%d]: got %q, want %q", i, settings[i].ProviderKey, want)
		}
		if settings[i].Priority != i+1 {
			t.Errorf("priority[%d]: got %d, want %d", i, settings[i].Priority, i+1)
		}
	}
}

// --- Ensure ---

func TestEnsure_insertsNewProvider(t *testing.T) {
	r := newRepo(t)
	if err := r.Ensure(ctx, "ollama", nil); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	s, err := r.Get(ctx, "ollama")
	if err != nil || s == nil {
		t.Fatalf("Get after Ensure: %v", err)
	}
	if s.IsEnabled != 1 {
		t.Errorf("IsEnabled: got %d", s.IsEnabled)
	}
}

func TestEnsure_idempotent(t *testing.T) {
	r := newRepo(t)
	for i := 0; i < 3; i++ {
		if err := r.Ensure(ctx, "zai", nil); err != nil {
			t.Fatalf("Ensure run %d: %v", i+1, err)
		}
	}
	all, _ := r.All(ctx)
	count := 0
	for _, s := range all {
		if s.ProviderKey == "zai" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Ensure idempotent: found %d rows for 'zai', want 1", count)
	}
}

func TestEnsure_withExplicitPriority(t *testing.T) {
	r := newRepo(t)
	p := 42
	if err := r.Ensure(ctx, "custom", &p); err != nil {
		t.Fatal(err)
	}
	s, _ := r.Get(ctx, "custom")
	if s.Priority != 42 {
		t.Errorf("Priority: got %d, want 42", s.Priority)
	}
}

// --- Sync ---

func TestSync_addsNewAndDeletesStale(t *testing.T) {
	r := newRepo(t)

	// canonical list: drop openrouter, add ollama and zai
	canonical := []string{"cerebras", "groq", "gemini", "ollama", "zai"}
	if err := r.Sync(ctx, canonical); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := r.All(ctx)
	if err != nil {
		t.Fatal(err)
	}

	present := make(map[string]bool)
	for _, s := range all {
		present[s.ProviderKey] = true
	}

	for _, want := range canonical {
		if !present[want] {
			t.Errorf("Sync: %q should be present", want)
		}
	}
	if present["openrouter"] {
		t.Error("Sync: openrouter should have been removed")
	}
}

func TestSync_idempotent(t *testing.T) {
	r := newRepo(t)
	canonical := []string{"cerebras", "groq", "openrouter", "gemini"}
	for i := 0; i < 2; i++ {
		if err := r.Sync(ctx, canonical); err != nil {
			t.Fatalf("Sync run %d: %v", i+1, err)
		}
	}
	all, _ := r.All(ctx)
	if len(all) != 4 {
		t.Errorf("Sync idempotent: got %d rows, want 4", len(all))
	}
}
