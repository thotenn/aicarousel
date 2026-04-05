package httpapi

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/thotenn/aicarousel-go/internal/auth"
	"github.com/thotenn/aicarousel-go/internal/db/apikeys"
)

// New wraps the provided mux with the full middleware chain:
//
//	corsMiddleware → recoverMiddleware → auth.Gate → mux
//
// Callers (e.g. cmd/server/main.go) are responsible for registering routes on
// mux before calling New. This keeps the httpapi package free of sub-package
// imports, preventing import cycles.
func New(keysRepo apikeys.Repo, mux *http.ServeMux) http.Handler {
	return corsMiddleware(recoverMiddleware(auth.Gate(keysRepo, mux)))
}

// corsMiddleware adds CORS headers to every response and handles OPTIONS
// preflight requests with 204 before they reach auth or routing.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		h.Set("Access-Control-Allow-Headers",
			"Content-Type, Authorization, x-api-key, anthropic-version, anthropic-beta")
		h.Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoverMiddleware catches panics anywhere in the handler chain and returns 500.
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("panic recovered",
					"err", v,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
