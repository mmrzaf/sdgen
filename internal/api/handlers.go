
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/mmrzaf/sdgen/internal/app"
	"github.com/mmrzaf/sdgen/internal/domain"
	"github.com/mmrzaf/sdgen/internal/infra/repos/scenarios"
	"github.com/mmrzaf/sdgen/internal/infra/repos/targets"
)

type Handler struct {
	scenarioRepo *scenarios.FileRepository
	targetRepo   *targets.FileRepository
	runService   *app.RunService
}

func NewHandler(
	scenarioRepo *scenarios.FileRepository,
	targetRepo *targets.FileRepository,
	runService *app.RunService,
) *Handler {
	return &Handler{
		scenarioRepo: scenarioRepo,
		targetRepo:   targetRepo,
		runService:   runService,
	}
}

func (h *Handler) ListScenarios(w http.ResponseWriter, r *http.Request) {
	scenarios, err := h.scenarioRepo.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, scenarios)
}

func (h *Handler) GetScenario(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	scenario, err := h.scenarioRepo.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	respondJSON(w, scenario)
}

func (h *Handler) ListTargets(w http.ResponseWriter, r *http.Request) {
	targets, err := h.targetRepo.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, targets)
}

func (h *Handler) GetTarget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	target, err := h.targetRepo.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	respondJSON(w, target)
}

func (h *Handler) CreateRun(w http.ResponseWriter, r *http.Request) {
	var req domain.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	run, err := h.runService.StartRun(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, run)
}

func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	status := r.URL.Query().Get("status")

	runs, err := h.runService.ListRuns(limit, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, runs)
}

func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	run, err := h.runService.GetRun(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	respondJSON(w, run)
}

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

