package modelsconfig

import (
	"sync"
	"time"
)

const cacheTTL = time.Second

// cache holds the loaded ModelsConfig with metadata.
// All fields are protected by cacheMu.
var (
	cacheMu      sync.RWMutex
	cacheData    ModelsConfig
	cacheAt      time.Time
	cacheFromEnv bool // true when data was loaded from MODELS_CONFIG env
)

// cacheRead returns (copy, true) if the cache is valid for the requested source.
// Returns (nil, false) on miss or TTL expiry.
func cacheRead(wantFromEnv bool) (ModelsConfig, bool) {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	return cacheReadLocked(wantFromEnv)
}

// cacheReadLocked is the same as cacheRead but assumes the caller already holds
// at least cacheMu.RLock (used for the double-check inside Load's write lock).
func cacheReadLocked(wantFromEnv bool) (ModelsConfig, bool) {
	if cacheData == nil {
		return nil, false
	}
	if cacheFromEnv != wantFromEnv {
		return nil, false // source type changed (env was added/removed)
	}
	if !wantFromEnv && time.Since(cacheAt) >= cacheTTL {
		return nil, false // file cache expired
	}
	return deepCopy(cacheData), true
}

// cacheWrite stores cfg into the cache. Caller must hold cacheMu write lock.
func cacheWrite(cfg ModelsConfig, fromEnv bool) {
	cacheData = cfg
	cacheAt = time.Now()
	cacheFromEnv = fromEnv
}

// InvalidateCache resets the in-memory cache.
// The next call to Load will re-read from the authoritative source.
// Called automatically by Save; also exported for tests.
func InvalidateCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cacheData = nil
	cacheAt = time.Time{}
	cacheFromEnv = false
}

// deepCopy returns a full structural copy of cfg.
// This prevents callers from mutating the cached map/slices.
func deepCopy(cfg ModelsConfig) ModelsConfig {
	if cfg == nil {
		return nil
	}
	out := make(ModelsConfig, len(cfg))
	for k, v := range cfg {
		models := make([]string, len(v.Models))
		copy(models, v.Models)
		out[k] = ProviderModelConfig{
			Default:        v.Default,
			EnableFallback: v.EnableFallback,
			Models:         models,
		}
	}
	return out
}
