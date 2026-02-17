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
	targetRepo   targets.Repository
	runService   *app.RunService
}

func NewHandler(scenarioRepo *scenarios.FileRepository, targetRepo targets.Repository, runService *app.RunService) *Handler {
	return &Handler{
		scenarioRepo: scenarioRepo,
		targetRepo:   targetRepo,
		runService:   runService,
	}
}

func (h *Handler) ListScenarios(w http.ResponseWriter, r *http.Request) {
	list, err := h.scenarioRepo.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

func (h *Handler) GetScenario(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sc, err := h.scenarioRepo.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, sc)
}

// Targets CRUD (DB-backed) with DSN redacted on output

func (h *Handler) ListTargets(w http.ResponseWriter, r *http.Request) {
	list, err := h.targetRepo.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, targets.RedactTargets(list))
}

func (h *Handler) GetTarget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := h.targetRepo.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, targets.RedactTarget(t))
}

func (h *Handler) CreateTarget(w http.ResponseWriter, r *http.Request) {
	var t domain.TargetConfig
	if err := decodeJSONStrict(r, &t); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := h.runService.Validator().ValidateTarget(&t); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.targetRepo.Create(&t); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, targets.RedactTarget(&t))
}

func (h *Handler) UpdateTarget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var t domain.TargetConfig
	if err := decodeJSONStrict(r, &t); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if t.ID != "" && t.ID != id {
		http.Error(w, "id mismatch", http.StatusBadRequest)
		return
	}
	t.ID = id
	if err := h.runService.Validator().ValidateTarget(&t); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.targetRepo.Update(&t); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, targets.RedactTarget(&t))
}

func (h *Handler) DeleteTarget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.targetRepo.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TestTarget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, err := h.runService.TestTarget(id)
	if res != nil {
		writeJSON(w, res)
		return
	}
	http.Error(w, err.Error(), http.StatusBadRequest)
}

// Runs

func (h *Handler) CreateRun(w http.ResponseWriter, r *http.Request) {
	var req domain.RunRequest
	if err := decodeJSONStrict(r, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	run, err := h.runService.StartRun(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, run)
}

func (h *Handler) PlanRun(w http.ResponseWriter, r *http.Request) {
	var req domain.RunRequest
	if err := decodeJSONStrict(r, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	plan, err := h.runService.PlanRun(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, plan)
}

func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := h.runService.ListRuns(50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, runs)
}

func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := h.runService.GetRun(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, run)
}

func (h *Handler) GetRunLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit := 200
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 2000 {
			limit = n
		}
	}
	logs, err := h.runService.ListRunLogs(id, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, logs)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSONStrict(r *http.Request, out any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}
