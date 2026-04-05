package secure

import (
	"strings"
	"testing"
)

// --- GenerateAPIKey ---

func TestGenerateAPIKey_plainFormat(t *testing.T) {
	plain, _, _ := GenerateAPIKey()
	if !strings.HasPrefix(plain, "sk-") {
		t.Errorf("plain should start with sk-, got %q", plain)
	}
	if len(plain) != 67 {
		t.Errorf("plain length: got %d, want 67 (sk- + 64 hex)", len(plain))
	}
}

func TestGenerateAPIKey_plainHexChars(t *testing.T) {
	plain, _, _ := GenerateAPIKey()
	hex := plain[3:] // strip "sk-"
	for _, c := range hex {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("non-hex char %q in plain key %q", c, plain)
		}
	}
}

func TestGenerateAPIKey_hashConsistency(t *testing.T) {
	plain, hash, _ := GenerateAPIKey()
	if got := HashKey(plain); got != hash {
		t.Errorf("hash mismatch: GenerateAPIKey hash=%q, HashKey(plain)=%q", hash, got)
	}
}

func TestGenerateAPIKey_prefixFormat(t *testing.T) {
	plain, _, prefix := GenerateAPIKey()
	if !strings.HasSuffix(prefix, "...") {
		t.Errorf("prefix should end with ..., got %q", prefix)
	}
	// prefix is first 10 chars of plain + "..."
	want := plain[:10] + "..."
	if prefix != want {
		t.Errorf("prefix: got %q, want %q", prefix, want)
	}
}

func TestGenerateAPIKey_uniqueness(t *testing.T) {
	const n = 500
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		plain, _, _ := GenerateAPIKey()
		if seen[plain] {
			t.Fatalf("duplicate key after %d iterations", i)
		}
		seen[plain] = true
	}
}

// --- HashKey ---

func TestHashKey_deterministic(t *testing.T) {
	key := "sk-deadbeef"
	h1 := HashKey(key)
	h2 := HashKey(key)
	if h1 != h2 {
		t.Errorf("HashKey is not deterministic: %q vs %q", h1, h2)
	}
}

func TestHashKey_length(t *testing.T) {
	h := HashKey("sk-test")
	// SHA-256 → 32 bytes → 64 hex chars
	if len(h) != 64 {
		t.Errorf("hash length: got %d, want 64", len(h))
	}
}

func TestHashKey_differentInputs(t *testing.T) {
	h1 := HashKey("sk-aaaa")
	h2 := HashKey("sk-bbbb")
	if h1 == h2 {
		t.Error("different inputs produced the same hash")
	}
}

func TestHashKey_hexOnly(t *testing.T) {
	h := HashKey("sk-test")
	for _, c := range h {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("non-hex char %q in hash %q", c, h)
		}
	}
}

// --- MaskKey ---

func TestMaskKey_normal(t *testing.T) {
	key := "sk-abcdefghijklmnopqrstuvwxyz"
	got := MaskKey(key)
	want := "sk-abcdefg..."
	if got != want {
		t.Errorf("MaskKey: got %q, want %q", got, want)
	}
}

func TestMaskKey_exactlyTen(t *testing.T) {
	key := "sk-abcdefg" // exactly 10 chars
	got := MaskKey(key)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("MaskKey should always end with ..., got %q", got)
	}
}

func TestMaskKey_short(t *testing.T) {
	key := "sk-ab" // shorter than 10
	got := MaskKey(key)
	if got != key+"..." {
		t.Errorf("MaskKey short key: got %q, want %q", got, key+"...")
	}
}

func TestMaskKey_realKey(t *testing.T) {
	plain, _, prefix := GenerateAPIKey()
	if !strings.HasPrefix(prefix, plain[:10]) {
		t.Errorf("prefix should start with first 10 chars of key")
	}
}
