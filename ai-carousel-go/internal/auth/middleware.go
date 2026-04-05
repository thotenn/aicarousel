package auth

import (
	"net/http"
	"strings"

	"github.com/thotenn/aicarousel-go/internal/db/apikeys"
	anthropic "github.com/thotenn/aicarousel-go/internal/formatters/anthropic"
	openai "github.com/thotenn/aicarousel-go/internal/formatters/openai"
)

// Gate returns an http.Handler that validates API keys before delegating to next.
// Public paths bypass validation entirely.
func Gate(repo apikeys.Repo, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublic(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		key := extractKey(r)
		if key == "" {
			writeAuthError(w, r.URL.Path,
				"Missing API key. Include it as 'Authorization: Bearer sk-xxx' or 'x-api-key: sk-xxx'")
			return
		}

		ak, err := repo.Validate(r.Context(), key)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if ak == nil {
			writeAuthError(w, r.URL.Path, "Invalid API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isPublic reports whether path is exempt from authentication.
func isPublic(path string) bool {
	return path == "/health" ||
		path == "/v1/models" ||
		strings.HasPrefix(path, "/v1/models/")
}

// extractKey pulls the plain-text API key from Authorization: Bearer or x-api-key.
func extractKey(r *http.Request) string {
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		return a[7:]
	}
	return r.Header.Get("x-api-key")
}

// writeAuthError writes a 401 response in the correct error shape for the path.
func writeAuthError(w http.ResponseWriter, path, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	if strings.HasPrefix(path, "/v1/messages") {
		w.Write(anthropic.ErrorJSON(msg, "authentication_error")) //nolint:errcheck
	} else {
		code := "invalid_api_key"
		w.Write(openai.ErrorJSON(msg, "invalid_request_error", &code)) //nolint:errcheck
	}
}
