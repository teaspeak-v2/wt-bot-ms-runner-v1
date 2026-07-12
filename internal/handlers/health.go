package handlers

import (
	"encoding/json"
	"net/http"
)

// HealthHandler handles health and readiness probes.
type HealthHandler struct {
	runnerReady func() error
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(runnerReady func() error) *HealthHandler {
	return &HealthHandler{runnerReady: runnerReady}
}

// Healthz returns a liveness probe.
func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz returns a readiness probe.
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	if h.runnerReady != nil {
		if err := h.runnerReady(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready", "error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Root returns a simple root response.
func (h *HealthHandler) Root(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]string{"service": "wt-bot-ms-runner-v1"})
}
