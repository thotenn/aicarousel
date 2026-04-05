package secure

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// GenerateAPIKey creates a new application API key.
//
// Returns:
//   - plain:  full key, format "sk-<64 hex chars>" (67 chars total)
//   - hash:   SHA-256 hex digest of plain (stored in the database)
//   - prefix: first 10 chars of plain + "..." for safe display (e.g. "sk-xxxxxxx...")
//
// The plain key is shown to the user exactly once; only hash is persisted.
func GenerateAPIKey() (plain, hash, prefix string) {
	b := make([]byte, 32) // 32 bytes → 64 hex chars
	if _, err := rand.Read(b); err != nil {
		panic("secure: crypto/rand unavailable: " + err.Error())
	}
	plain = "sk-" + hex.EncodeToString(b)
	hash = HashKey(plain)
	prefix = MaskKey(plain)
	return
}

// HashKey returns the SHA-256 hex digest of the given API key.
// This is the value stored in api_keys.key_hash.
func HashKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// MaskKey returns a display-safe prefix of the key.
// Format: first 10 characters + "..." (e.g. "sk-xxxxxxx...").
// Safe to log at any level.
func MaskKey(key string) string {
	if len(key) <= 10 {
		return key + "..."
	}
	return key[:10] + "..."
}
