package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/thotenn/aicarousel-go/internal/chat"
	oaifmt "github.com/thotenn/aicarousel-go/internal/formatters/openai"
	"github.com/thotenn/aicarousel-go/internal/httpapi"
	"github.com/thotenn/aicarousel-go/internal/util/id"
)

// chatRouter is the minimal interface handlers require from the chat.Router.
type chatRouter interface {
	Handle(ctx context.Context, msgs []chat.ChatMessage) (<-chan chat.StreamChunk, error)
}

// Handler holds dependencies for OpenAI-compatible HTTP endpoints.
type Handler struct {
	router chatRouter
}

// New creates a Handler backed by the given router.
func New(r chatRouter) *Handler { return &Handler{router: r} }

// Register mounts OpenAI routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/chat/completions", h.chatCompletions)
	mux.HandleFunc("GET /v1/models", h.models)
	mux.HandleFunc("GET /v1/models/{id}", h.modelInfo)
}

// ── request body ─────────────────────────────────────────────────────────────

type completionRequest struct {
	Model    string            `json:"model"`
	Messages []messageRequest  `json:"messages"`
	Stream   *bool             `json:"stream"`
}

type messageRequest struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ── model list structs ────────────────────────────────────────────────────────

type modelList struct {
	Object string      `json:"object"`
	Data   []modelInfo `json:"data"`
}

type modelInfo struct {
	ID         string      `json:"id"`
	Object     string      `json:"object"`
	Created    int64       `json:"created"`
	OwnedBy    string      `json:"owned_by"`
	Permission []any       `json:"permission"`
	Root       string      `json:"root"`
	Parent     interface{} `json:"parent"`
}

// ── handlers ─────────────────────────────────────────────────────────────────

func (h *Handler) chatCompletions(w http.ResponseWriter, r *http.Request) {
	var req completionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", "invalid_request_error", nil, http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		writeError(w, "messages is required and must be a non-empty array",
			"invalid_request_error", strPtr("messages"), http.StatusBadRequest)
		return
	}

	msgs := make([]chat.ChatMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = chat.ChatMessage{Role: m.Role, Content: m.Content}
	}

	model := req.Model
	if model == "" {
		model = "aicarousel"
	}

	// stream defaults to true
	shouldStream := req.Stream == nil || *req.Stream

	ch, err := h.router.Handle(r.Context(), msgs)
	if err != nil {
		writeError(w, "All AI services failed", "server_error", nil, http.StatusServiceUnavailable)
		return
	}

	reqID := id.NewChatCmplID()
	created := time.Now().Unix()

	if shouldStream {
		flusher, ok := httpapi.AssertFlusher(w)
		if !ok {
			return
		}
		httpapi.SetSSEHeaders(w)
		if err := oaifmt.Stream(w, flusher.Flush, reqID, created, model, ch); err != nil {
			// Stream already partially written; best effort.
			return
		}
		return
	}

	// Non-streaming: collect and return single JSON object.
	resp, err := oaifmt.Complete(reqID, created, model, ch)
	if err != nil {
		writeError(w, "All AI services failed", "server_error", nil, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

func (h *Handler) models(w http.ResponseWriter, r *http.Request) {
	now := time.Now().Unix()
	list := modelList{
		Object: "list",
		Data: []modelInfo{
			{ID: "aicarousel", Object: "model", Created: now, OwnedBy: "aicarousel", Permission: []any{}, Root: "aicarousel", Parent: nil},
			{ID: "gpt-4", Object: "model", Created: now, OwnedBy: "openai", Permission: []any{}, Root: "gpt-4", Parent: nil},
			{ID: "gpt-3.5-turbo", Object: "model", Created: now, OwnedBy: "openai", Permission: []any{}, Root: "gpt-3.5-turbo", Parent: nil},
			{ID: "claude-3-5-sonnet-20241022", Object: "model", Created: now, OwnedBy: "anthropic", Permission: []any{}, Root: "claude-3-5-sonnet-20241022", Parent: nil},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list) //nolint:errcheck
}

func (h *Handler) modelInfo(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	// Strip any residual prefix if PathValue includes it.
	modelID = strings.TrimPrefix(modelID, "/v1/models/")
	if modelID == "" {
		http.NotFound(w, r)
		return
	}
	info := modelInfo{
		ID:         modelID,
		Object:     "model",
		Created:    time.Now().Unix(),
		OwnedBy:    "aicarousel",
		Permission: []any{},
		Root:       modelID,
		Parent:     nil,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info) //nolint:errcheck
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeError(w http.ResponseWriter, msg, errType string, param *string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	type errBody struct {
		Message string  `json:"message"`
		Type    string  `json:"type"`
		Param   *string `json:"param"`
		Code    *string `json:"code"`
	}
	type errResp struct {
		Error errBody `json:"error"`
	}
	json.NewEncoder(w).Encode(errResp{Error: errBody{ //nolint:errcheck
		Message: msg,
		Type:    errType,
		Param:   param,
		Code:    nil,
	}})
}

func strPtr(s string) *string { return &s }
