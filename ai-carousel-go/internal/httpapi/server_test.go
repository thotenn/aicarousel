package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thotenn/aicarousel-go/internal/db/apikeys"
	"github.com/thotenn/aicarousel-go/internal/httpapi"
)

// ── minimal stubs ─────────────────────────────────────────────────────────────

type stubKeysRepo struct{}

func (s *stubKeysRepo) Validate(_ context.Context, _ string) (*apikeys.ApiKey, error) {
	return &apikeys.ApiKey{ID: 1, IsActive: 1}, nil // always valid
}
func (s *stubKeysRepo) Create(_ context.Context, _ string) (string, apikeys.ApiKey, error) {
	return "", apikeys.ApiKey{}, nil
}
func (s *stubKeysRepo) List(_ context.Context) ([]apikeys.ApiKey, error)    { return nil, nil }
func (s *stubKeysRepo) Revoke(_ context.Context, _ int64) (bool, error)     { return false, nil }
func (s *stubKeysRepo) Delete(_ context.Context, _ int64) (bool, error)     { return false, nil }
func (s *stubKeysRepo) GetByID(_ context.Context, _ int64) (*apikeys.ApiKey, error) { return nil, nil }


func newTestHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return httpapi.New(&stubKeysRepo{}, mux)
}

// ── CORS: OPTIONS on known path ───────────────────────────────────────────────

func TestCORS_Options_KnownPath(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/v1/chat/completions", nil)
	newTestHandler().ServeHTTP(w, r)

	assertCORS(t, w, http.StatusNoContent)
}

// ── CORS: OPTIONS on UNKNOWN path (critical requirement) ─────────────────────

func TestCORS_Options_UnknownPath(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/totally/unknown/path", nil)
	newTestHandler().ServeHTTP(w, r)

	// Must return 204 with CORS headers — never 401 or 404.
	assertCORS(t, w, http.StatusNoContent)
}

// ── CORS: headers on non-OPTIONS ──────────────────────────────────────────────

func TestCORS_Headers_NonOptions(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	newTestHandler().ServeHTTP(w, r)

	assertCORSHeaders(t, w)
}

// ── CORS: OPTIONS never reaches auth ─────────────────────────────────────────

func TestCORS_Options_SkipsAuth(t *testing.T) {
	// Repo that always rejects — if auth ran, request would 401.
	rejectRepo := &rejectRepo{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := httpapi.New(rejectRepo, mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/v1/chat/completions", nil)
	handler.ServeHTTP(w, r)

	// Must be 204, NOT 401.
	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS hit auth: got %d, want 204", w.Code)
	}
}

// ── recover middleware ────────────────────────────────────────────────────────

func TestRecover_PanicReturns500(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /panic", func(w http.ResponseWriter, _ *http.Request) {
		panic("test panic")
	})
	handler := httpapi.New(&stubKeysRepo{}, mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/panic", nil)
	r.Header.Set("Authorization", "Bearer sk-anything") // pass auth
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("panic: got %d, want 500", w.Code)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func assertCORS(t *testing.T, w *httptest.ResponseRecorder, wantStatus int) {
	t.Helper()
	if w.Code != wantStatus {
		t.Errorf("status: got %d, want %d", w.Code, wantStatus)
	}
	assertCORSHeaders(t, w)
}

func assertCORSHeaders(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("ACAO: got %q, want *", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("ACAM header missing")
	}
	if got := w.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("ACAH header missing")
	}
}

// rejectRepo always returns nil (invalid key).
type rejectRepo struct{}

func (r *rejectRepo) Validate(_ context.Context, _ string) (*apikeys.ApiKey, error) {
	return nil, nil
}
func (r *rejectRepo) Create(_ context.Context, _ string) (string, apikeys.ApiKey, error) {
	return "", apikeys.ApiKey{}, nil
}
func (r *rejectRepo) List(_ context.Context) ([]apikeys.ApiKey, error)    { return nil, nil }
func (r *rejectRepo) Revoke(_ context.Context, _ int64) (bool, error)     { return false, nil }
func (r *rejectRepo) Delete(_ context.Context, _ int64) (bool, error)     { return false, nil }
func (r *rejectRepo) GetByID(_ context.Context, _ int64) (*apikeys.ApiKey, error) { return nil, nil }
