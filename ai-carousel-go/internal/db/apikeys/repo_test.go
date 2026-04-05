package apikeys_test

import (
	"context"
	"strings"
	"testing"

	"github.com/thotenn/aicarousel-go/internal/db/apikeys"
	"github.com/thotenn/aicarousel-go/testutil"
)

var ctx = context.Background()

func newRepo(t *testing.T) apikeys.Repo {
	t.Helper()
	return apikeys.New(testutil.NewTestDB(t))
}

// --- Create ---

func TestCreate_plainKeyFormat(t *testing.T) {
	r := newRepo(t)
	plain, _, err := r.Create(ctx, "test")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.HasPrefix(plain, "sk-") {
		t.Errorf("plain key should start with sk-, got %q", plain)
	}
	if len(plain) != 67 {
		t.Errorf("plain key length: got %d, want 67", len(plain))
	}
}

func TestCreate_recordFields(t *testing.T) {
	r := newRepo(t)
	plain, rec, err := r.Create(ctx, "my-app")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	wantPrefix := plain[:10] + "..."
	if rec.KeyPrefix != wantPrefix {
		t.Errorf("KeyPrefix: got %q, want %q", rec.KeyPrefix, wantPrefix)
	}
	if len(rec.KeyHash) != 64 {
		t.Errorf("KeyHash length: got %d, want 64", len(rec.KeyHash))
	}
	if rec.KeyHash == plain {
		t.Error("KeyHash must not equal the plain key")
	}
	if !rec.Name.Valid || rec.Name.String != "my-app" {
		t.Errorf("Name: got %v, want 'my-app'", rec.Name)
	}
	if rec.IsActive != 1 {
		t.Errorf("IsActive: got %d, want 1", rec.IsActive)
	}
	if rec.UsageCount != 0 {
		t.Errorf("UsageCount: got %d, want 0", rec.UsageCount)
	}
}

func TestCreate_withoutName(t *testing.T) {
	r := newRepo(t)
	_, rec, err := r.Create(ctx, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.Name.Valid {
		t.Errorf("Name should be NULL for empty string, got %v", rec.Name)
	}
}

func TestCreate_uniqueKeys(t *testing.T) {
	r := newRepo(t)
	plain1, _, _ := r.Create(ctx, "")
	plain2, _, _ := r.Create(ctx, "")
	if plain1 == plain2 {
		t.Error("two generated keys must be unique")
	}
}

// --- Validate ---

func TestValidate_validKey(t *testing.T) {
	r := newRepo(t)
	plain, _, err := r.Create(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Validate(ctx, plain)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil for valid key")
	}
}

func TestValidate_unknownKey(t *testing.T) {
	r := newRepo(t)
	// Valid format but not in DB.
	fakeKey := "sk-" + strings.Repeat("a", 64)
	got, err := r.Validate(ctx, fakeKey)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got != nil {
		t.Error("expected nil for unknown key")
	}
}

func TestValidate_wrongPrefix(t *testing.T) {
	r := newRepo(t)
	got, err := r.Validate(ctx, "bearer-abc123")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got != nil {
		t.Error("expected nil for key without sk- prefix")
	}
}

func TestValidate_bumpsUsageCount(t *testing.T) {
	r := newRepo(t)
	plain, rec, err := r.Create(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	const calls = 3
	for i := 0; i < calls; i++ {
		if _, err := r.Validate(ctx, plain); err != nil {
			t.Fatalf("Validate call %d: %v", i+1, err)
		}
	}

	after, err := r.GetByID(ctx, rec.ID)
	if err != nil || after == nil {
		t.Fatalf("GetByID after Validate: %v", err)
	}
	if after.UsageCount != calls {
		t.Errorf("UsageCount: got %d, want %d", after.UsageCount, calls)
	}
	if !after.LastUsedAt.Valid {
		t.Error("LastUsedAt should be set after Validate")
	}
}

func TestValidate_revokedKeyIsInvalid(t *testing.T) {
	r := newRepo(t)
	plain, rec, err := r.Create(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Revoke(ctx, rec.ID); err != nil {
		t.Fatal(err)
	}
	got, err := r.Validate(ctx, plain)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("revoked key must not validate")
	}
}

// --- Revoke ---

func TestRevoke_setsInactive(t *testing.T) {
	r := newRepo(t)
	_, rec, err := r.Create(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := r.Revoke(ctx, rec.ID)
	if err != nil || !ok {
		t.Fatalf("Revoke: ok=%v err=%v", ok, err)
	}
	after, _ := r.GetByID(ctx, rec.ID)
	if after.IsActive != 0 {
		t.Errorf("IsActive after Revoke: got %d, want 0", after.IsActive)
	}
}

func TestRevoke_missingID_returnsFalse(t *testing.T) {
	r := newRepo(t)
	ok, err := r.Revoke(ctx, 99999)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("Revoke on non-existent id should return false")
	}
}

// --- Delete ---

func TestDelete_removesRow(t *testing.T) {
	r := newRepo(t)
	_, rec, err := r.Create(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := r.Delete(ctx, rec.ID)
	if err != nil || !ok {
		t.Fatalf("Delete: ok=%v err=%v", ok, err)
	}
	got, _ := r.GetByID(ctx, rec.ID)
	if got != nil {
		t.Error("GetByID should return nil after Delete")
	}
}

func TestDelete_missingID_returnsFalse(t *testing.T) {
	r := newRepo(t)
	ok, err := r.Delete(ctx, 99999)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("Delete on non-existent id should return false")
	}
}

// --- List ---

func TestList_noHashReturned(t *testing.T) {
	r := newRepo(t)
	if _, _, err := r.Create(ctx, "test"); err != nil {
		t.Fatal(err)
	}
	keys, err := r.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) == 0 {
		t.Fatal("List: expected at least one key")
	}
	for _, k := range keys {
		if k.KeyHash != "" {
			t.Error("List must not include key_hash")
		}
	}
}

func TestList_multipleKeys(t *testing.T) {
	r := newRepo(t)
	for i := 0; i < 3; i++ {
		if _, _, err := r.Create(ctx, ""); err != nil {
			t.Fatal(err)
		}
	}
	keys, err := r.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) < 3 {
		t.Errorf("List: got %d, want >= 3", len(keys))
	}
}

// --- GetByID ---

func TestGetByID_found(t *testing.T) {
	r := newRepo(t)
	_, rec, err := r.Create(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.GetByID(ctx, rec.ID)
	if err != nil || got == nil {
		t.Fatalf("GetByID: got=%v err=%v", got, err)
	}
	if got.ID != rec.ID {
		t.Errorf("ID mismatch: got %d, want %d", got.ID, rec.ID)
	}
}

func TestGetByID_notFound(t *testing.T) {
	r := newRepo(t)
	got, err := r.GetByID(ctx, 99999)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("GetByID should return nil for missing id")
	}
}
