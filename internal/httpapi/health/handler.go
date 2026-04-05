package health

import (
	"encoding/json"
	"net/http"
)

// Register mounts the health-check route on mux.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", handle)
}

func handle(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct { //nolint:errcheck
		Status  string `json:"status"`
		Service string `json:"service"`
	}{"ok", "aicarousel"})
}
