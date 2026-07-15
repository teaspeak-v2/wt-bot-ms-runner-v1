package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/teaspeak-v2/wt-bot-ms-runner-v1/internal/models"
	"github.com/teaspeak-v2/wt-bot-ms-runner-v1/internal/runner"
)

// RunnerHandler exposes the bot runner HTTP API.
type RunnerHandler struct {
	runner *runner.Runner
	logger *slog.Logger
}

// NewRunnerHandler creates a new handler.
func NewRunnerHandler(r *runner.Runner, logger *slog.Logger) *RunnerHandler {
	return &RunnerHandler{runner: r, logger: logger}
}

// Routes registers the handler routes.
func (h *RunnerHandler) Routes(r chi.Router) {
	r.Post("/bots/{id}/spawn", h.Spawn)
	r.Post("/bots/{id}/stop", h.Stop)
	r.Get("/bots/{id}/status", h.Status)
	r.Get("/containers", h.ListContainers)
	r.Get("/containers/{name}/logs", h.ContainerLogs)
	r.Get("/infra", h.InfraReport)
}

func (h *RunnerHandler) parseID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "id"))
}

// Spawn starts a bot container for the given bot ID.
func (h *RunnerHandler) Spawn(w http.ResponseWriter, r *http.Request) {
	id, err := h.parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid bot id", err)
		return
	}

	res, err := h.runner.Spawn(r.Context(), id)
	if err != nil {
		h.logger.Error("spawn failed", "error", err, "bot_id", id)
		writeError(w, http.StatusInternalServerError, "failed to spawn bot", err)
		return
	}

	writeJSON(w, http.StatusCreated, models.SpawnResponse{ContainerID: res.ContainerID, Status: res.Status})
}

// Stop stops a bot container.
func (h *RunnerHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id, err := h.parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid bot id", err)
		return
	}

	if err := h.runner.Stop(r.Context(), id); err != nil {
		h.logger.Error("stop failed", "error", err, "bot_id", id)
		writeError(w, http.StatusInternalServerError, "failed to stop bot", err)
		return
	}

	writeJSON(w, http.StatusOK, models.StopResponse{Message: "bot stopped"})
}

// Status returns the current status of a bot container.
func (h *RunnerHandler) Status(w http.ResponseWriter, r *http.Request) {
	id, err := h.parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid bot id", err)
		return
	}

	res, err := h.runner.Status(r.Context(), id)
	if err != nil {
		h.logger.Error("status failed", "error", err, "bot_id", id)
		writeError(w, http.StatusInternalServerError, "failed to get status", err)
		return
	}

	writeJSON(w, http.StatusOK, models.SpawnResponse{ContainerID: res.ContainerID, Status: res.Status})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string, err error) {
	resp := models.ErrorResponse{}
	resp.Error.Code = http.StatusText(status)
	resp.Error.Message = message
	if err != nil {
		resp.Error.Message = message + ": " + err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// ListContainers returns all containers on the host.
func (h *RunnerHandler) ListContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := h.runner.ListContainers(r.Context())
	if err != nil {
		h.logger.Error("list containers failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list containers", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"containers": containers})
}

// ContainerLogs returns logs from a container by name.
func (h *RunnerHandler) ContainerLogs(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "container name required", nil)
		return
	}
	tail := 200
	if t := r.URL.Query().Get("tail"); t != "" {
		if v, err := parseIntDefault(t, 200); err == nil {
			tail = v
		}
	}
	logs, err := h.runner.ContainerLogs(r.Context(), name, tail)
	if err != nil {
		h.logger.Error("container logs failed", "error", err, "container", name)
		writeError(w, http.StatusInternalServerError, "failed to get logs", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"container": name, "logs": logs})
}

// InfraReport returns a summary of the Docker infrastructure.
func (h *RunnerHandler) InfraReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.runner.InfraReport(r.Context())
	if err != nil {
		h.logger.Error("infra report failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get infra report", err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func parseIntDefault(s string, def int) (int, error) {
	if s == "" {
		return def, nil
	}
	v := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def, fmt.Errorf("invalid integer")
		}
		v = v*10 + int(c-'0')
	}
	return v, nil
}
