package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thotenn/aicarousel/internal/db/apikeys"
	"github.com/thotenn/aicarousel/testutil"
)

// ── mock repo ────────────────────────────────────────────────────────────────

// mockRepo is a minimal stub for apikeys.Repo.
// Validate returns the configured key/err; all other methods are no-ops.
type mockRepo struct {
	key       *apikeys.ApiKey // returned by Validate
	err       error           // returned by Validate
	callCount int             // incremented on each Validate call
	calledKey string          // last key passed to Validate
}

func (m *mockRepo) Validate(_ context.Context, key string) (*apikeys.ApiKey, error) {
	m.callCount++
	m.calledKey = key
	return m.key, m.err
}
func (m *mockRepo) Create(_ context.Context, _ string) (string, apikeys.ApiKey, error) {
	return "", apikeys.ApiKey{}, nil
}
func (m *mockRepo) List(_ context.Context) ([]apikeys.ApiKey, error) { return nil, nil }
func (m *mockRepo) Revoke(_ context.Context, _ int64) (bool, error)  { return false, nil }
func (m *mockRepo) Delete(_ context.Context, _ int64) (bool, error)  { return false, nil }
func (m *mockRepo) GetByID(_ context.Context, _ int64) (*apikeys.ApiKey, error) {
	return nil, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// ok200 is a simple next handler that returns 200 OK.
var ok200 = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func gateWith(repo apikeys.Repo) http.Handler { return Gate(repo, ok200) }

func newReq(method, path string, headers map[string]string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func validMock() *mockRepo {
	return &mockRepo{key: &apikeys.ApiKey{ID: 1, IsActive: 1}}
}
func invalidMock() *mockRepo { return &mockRepo{key: nil} }

// ── public paths ──────────────────────────────────────────────────────────────

func TestGate_PublicPaths_Bypass(t *testing.T) {
	paths := []string{"/health", "/v1/models", "/v1/models/gpt-4", "/v1/models/some/nested"}
	repo := invalidMock() // would return invalid if called

	for _, p := range paths {
		w := httptest.NewRecorder()
		gateWith(repo).ServeHTTP(w, newReq("GET", p, nil))
		if w.Code != http.StatusOK {
			t.Errorf("path %q: got %d, want 200 (public path should bypass auth)", p, w.Code)
		}
		if repo.callCount != 0 {
			t.Errorf("path %q: Validate should not be called for public paths", p)
		}
	}
}

// ── missing key ───────────────────────────────────────────────────────────────

func TestGate_MissingKey_OpenAIPath_401(t *testing.T) {
	w := httptest.NewRecorder()
	gateWith(invalidMock()).ServeHTTP(w, newReq("POST", "/v1/chat/completions", nil))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", w.Code)
	}
	assertOpenAIErrorShape(t, w.Body.Bytes())
}

func TestGate_MissingKey_AnthropicPath_401(t *testing.T) {
	w := httptest.NewRecorder()
	gateWith(invalidMock()).ServeHTTP(w, newReq("POST", "/v1/messages", nil))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", w.Code)
	}
	assertAnthropicErrorShape(t, w.Body.Bytes())
}

// ── invalid key ───────────────────────────────────────────────────────────────

func TestGate_InvalidKey_Bearer_OpenAIShape(t *testing.T) {
	w := httptest.NewRecorder()
	gateWith(invalidMock()).ServeHTTP(w, newReq("POST", "/v1/chat/completions",
		map[string]string{"Authorization": "Bearer sk-badbadbadbad"}))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", w.Code)
	}
	assertOpenAIErrorShape(t, w.Body.Bytes())
}

func TestGate_InvalidKey_XApiKey_OpenAIShape(t *testing.T) {
	w := httptest.NewRecorder()
	gateWith(invalidMock()).ServeHTTP(w, newReq("POST", "/v1/chat/completions",
		map[string]string{"x-api-key": "sk-badbadbadbad"}))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", w.Code)
	}
	assertOpenAIErrorShape(t, w.Body.Bytes())
}

func TestGate_InvalidKey_AnthropicPath_AnthropicShape(t *testing.T) {
	w := httptest.NewRecorder()
	gateWith(invalidMock()).ServeHTTP(w, newReq("POST", "/v1/messages",
		map[string]string{"x-api-key": "sk-badbad"}))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", w.Code)
	}
	assertAnthropicErrorShape(t, w.Body.Bytes())
}

func TestGate_InvalidKey_MessagesCountTokens_AnthropicShape(t *testing.T) {
	w := httptest.NewRecorder()
	gateWith(invalidMock()).ServeHTTP(w, newReq("POST", "/v1/messages/count_tokens",
		map[string]string{"x-api-key": "sk-badbad"}))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", w.Code)
	}
	assertAnthropicErrorShape(t, w.Body.Bytes())
}

// ── valid key ─────────────────────────────────────────────────────────────────

func TestGate_ValidKey_Bearer_PassesThrough(t *testing.T) {
	repo := validMock()
	w := httptest.NewRecorder()
	gateWith(repo).ServeHTTP(w, newReq("POST", "/v1/chat/completions",
		map[string]string{"Authorization": "Bearer sk-abc123"}))

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if repo.callCount != 1 {
		t.Errorf("Validate call count: got %d, want 1", repo.callCount)
	}
	if repo.calledKey != "sk-abc123" {
		t.Errorf("Validate called with %q, want %q", repo.calledKey, "sk-abc123")
	}
}

func TestGate_ValidKey_XApiKey_PassesThrough(t *testing.T) {
	repo := validMock()
	w := httptest.NewRecorder()
	gateWith(repo).ServeHTTP(w, newReq("POST", "/v1/messages",
		map[string]string{"x-api-key": "sk-xyz789"}))

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if repo.calledKey != "sk-xyz789" {
		t.Errorf("Validate called with %q, want %q", repo.calledKey, "sk-xyz789")
	}
}

// BearerTakesPrecedence: when both headers are present, Bearer wins.
func TestGate_BearerTakesPrecedence(t *testing.T) {
	repo := validMock()
	w := httptest.NewRecorder()
	gateWith(repo).ServeHTTP(w, newReq("POST", "/v1/chat/completions", map[string]string{
		"Authorization": "Bearer sk-bearer",
		"x-api-key":     "sk-xapikey",
	}))

	if repo.calledKey != "sk-bearer" {
		t.Errorf("expected Bearer to win, got %q", repo.calledKey)
	}
}

// ── repo error → 500 ──────────────────────────────────────────────────────────

func TestGate_RepoError_500(t *testing.T) {
	repo := &mockRepo{err: errTest("db down")}
	w := httptest.NewRecorder()
	gateWith(repo).ServeHTTP(w, newReq("POST", "/v1/chat/completions",
		map[string]string{"Authorization": "Bearer sk-anything"}))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", w.Code)
	}
}

// ── validate called once per request ─────────────────────────────────────────

func TestGate_ValidateCalledOnce(t *testing.T) {
	repo := validMock()
	w := httptest.NewRecorder()
	gateWith(repo).ServeHTTP(w, newReq("POST", "/chat",
		map[string]string{"Authorization": "Bearer sk-test"}))

	if repo.callCount != 1 {
		t.Errorf("Validate called %d times, want 1", repo.callCount)
	}
}

// ── integration: real DB + usage_count bump ───────────────────────────────────

func TestGate_Integration_UsageCountBump(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := apikeys.New(db)

	plain, rec, err := repo.Create(context.Background(), "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if rec.UsageCount != 0 {
		t.Fatalf("initial usage_count: %d", rec.UsageCount)
	}

	// Hit a protected endpoint with the real key.
	w := httptest.NewRecorder()
	Gate(repo, ok200).ServeHTTP(w, newReq("POST", "/v1/chat/completions",
		map[string]string{"Authorization": "Bearer " + plain}))

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	// Validate bumps usage_count in the DB.
	after, err := repo.GetByID(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.UsageCount != 1 {
		t.Errorf("usage_count after 1 request: got %d, want 1", after.UsageCount)
	}

	// A second request increments again.
	w2 := httptest.NewRecorder()
	Gate(repo, ok200).ServeHTTP(w2, newReq("POST", "/v1/chat/completions",
		map[string]string{"Authorization": "Bearer " + plain}))
	after2, _ := repo.GetByID(context.Background(), rec.ID)
	if after2.UsageCount != 2 {
		t.Errorf("usage_count after 2 requests: got %d, want 2", after2.UsageCount)
	}
}

func TestGate_Integration_RevokedKey_Rejected(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := apikeys.New(db)

	plain, rec, err := repo.Create(context.Background(), "revoked-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Revoke(context.Background(), rec.ID); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	Gate(repo, ok200).ServeHTTP(w, newReq("POST", "/v1/chat/completions",
		map[string]string{"Authorization": "Bearer " + plain}))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked key: got %d, want 401", w.Code)
	}
}

// ── JSON shape assertions ─────────────────────────────────────────────────────

func assertOpenAIErrorShape(t *testing.T, body []byte) {
	t.Helper()
	var resp struct {
		Error struct {
			Message string  `json:"message"`
			Type    string  `json:"type"`
			Param   *string `json:"param"`
			Code    *string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("openai error shape: unmarshal failed: %v\nbody: %s", err, body)
	}
	if resp.Error.Message == "" {
		t.Error("openai error.message is empty")
	}
	if resp.Error.Type != "invalid_request_error" {
		t.Errorf("openai error.type: got %q, want \"invalid_request_error\"", resp.Error.Type)
	}
	if resp.Error.Param != nil {
		t.Errorf("openai error.param: got %v, want null", resp.Error.Param)
	}
	if resp.Error.Code == nil || *resp.Error.Code != "invalid_api_key" {
		t.Errorf("openai error.code: got %v, want \"invalid_api_key\"", resp.Error.Code)
	}
	// Field order: "error" must be the only top-level key
	raw := string(body)
	if !strings.HasPrefix(strings.TrimSpace(raw), `{"error"`) {
		t.Errorf("openai error must start with {\"error\": got %s", raw)
	}
}

func assertAnthropicErrorShape(t *testing.T, body []byte) {
	t.Helper()
	var resp struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("anthropic error shape: unmarshal failed: %v\nbody: %s", err, body)
	}
	if resp.Type != "error" {
		t.Errorf("anthropic top-level type: got %q, want \"error\"", resp.Type)
	}
	if resp.Error.Type != "authentication_error" {
		t.Errorf("anthropic error.type: got %q, want \"authentication_error\"", resp.Error.Type)
	}
	if resp.Error.Message == "" {
		t.Error("anthropic error.message is empty")
	}
}

// ── misc helpers ──────────────────────────────────────────────────────────────

type errTest string

func (e errTest) Error() string { return string(e) }

// Ensure mockRepo satisfies the interface at compile time.
var _ apikeys.Repo = (*mockRepo)(nil)

// Unused import guard for sql package (used in mockRepo).
var _ = sql.NullString{}
