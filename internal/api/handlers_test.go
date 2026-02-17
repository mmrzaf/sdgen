package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mmrzaf/sdgen/internal/app"
	"github.com/mmrzaf/sdgen/internal/domain"
	"github.com/mmrzaf/sdgen/internal/infra/repos/runs"
	"github.com/mmrzaf/sdgen/internal/infra/repos/scenarios"
	"github.com/mmrzaf/sdgen/internal/infra/repos/targets"
	"github.com/mmrzaf/sdgen/internal/logging"
	"github.com/mmrzaf/sdgen/internal/registry"
)

func newTestHandler(t *testing.T) (*Handler, *runs.SQLiteRepository) {
	t.Helper()

	scenariosDir := t.TempDir()
	scenarioPath := filepath.Join(scenariosDir, "s1.yaml")
	if err := os.WriteFile(scenarioPath, []byte(`
id: s1
name: scenario1
version: "1"
entities:
  - name: users
    target_table: users
    rows: 1
    columns:
      - name: id
        type: int
        generator:
          type: uniform_int
          params:
            min: 1
            max: 10
`), 0o644); err != nil {
		t.Fatal(err)
	}

	dbf, err := os.CreateTemp("", "sdgen_api_*.db")
	if err != nil {
		t.Fatal(err)
	}
	_ = dbf.Close()
	t.Cleanup(func() { _ = os.Remove(dbf.Name()) })

	runRepo := runs.NewSQLiteRepository(dbf.Name())
	if err := runRepo.Init(); err != nil {
		t.Fatal(err)
	}
	scRepo := scenarios.NewFileRepository(scenariosDir)
	targetRepo := targets.NewSQLiteRepository(runRepo.DB())
	runSvc := app.NewRunService(scRepo, targetRepo, runRepo, registry.DefaultGeneratorRegistry(), logging.NewLogger("error"), 1000)
	return NewHandler(scRepo, targetRepo, runSvc), runRepo
}

func TestGetRun_ReturnsProgressFields(t *testing.T) {
	h, runRepo := newTestHandler(t)
	run := &domain.Run{
		ID:                    "run-1",
		ScenarioID:            "s1",
		ScenarioName:          "scenario1",
		ScenarioVersion:       "1",
		TargetID:              "t1",
		TargetName:            "target1",
		TargetKind:            "sqlite",
		Seed:                  1,
		ConfigHash:            "abc",
		Status:                domain.RunStatusRunning,
		StartedAt:             time.Now().UTC(),
		Mode:                  "create",
		ProgressRowsGenerated: 12,
		ProgressRowsTotal:     50,
		ProgressEntitiesDone:  1,
		ProgressEntitiesTotal: 3,
		ProgressCurrentEntity: "orders",
	}
	if err := runRepo.Create(run); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/run-1", nil)
	req.SetPathValue("id", "run-1")
	rec := httptest.NewRecorder()
	h.GetRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var got domain.Run
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ProgressRowsGenerated != 12 || got.ProgressRowsTotal != 50 {
		t.Fatalf("unexpected progress rows: %#v", got)
	}
	if got.ProgressEntitiesDone != 1 || got.ProgressEntitiesTotal != 3 || got.ProgressCurrentEntity != "orders" {
		t.Fatalf("unexpected progress entities: %#v", got)
	}
}

func TestGetRunLogs_ReturnsMostRecentLogs(t *testing.T) {
	h, runRepo := newTestHandler(t)
	run := &domain.Run{
		ID:              "run-2",
		ScenarioID:      "s1",
		ScenarioName:    "scenario1",
		ScenarioVersion: "1",
		TargetID:        "t1",
		TargetName:      "target1",
		TargetKind:      "sqlite",
		Seed:            1,
		ConfigHash:      "abc",
		Status:          domain.RunStatusRunning,
		StartedAt:       time.Now().UTC(),
		Mode:            "create",
	}
	if err := runRepo.Create(run); err != nil {
		t.Fatal(err)
	}
	if err := runRepo.AppendRunLog("run-2", "info", "first"); err != nil {
		t.Fatal(err)
	}
	if err := runRepo.AppendRunLog("run-2", "info", "second"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/run-2/logs?limit=2", nil)
	req.SetPathValue("id", "run-2")
	rec := httptest.NewRecorder()
	h.GetRunLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var logs []*domain.RunLog
	if err := json.Unmarshal(rec.Body.Bytes(), &logs); err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	if logs[0].Message != "second" {
		t.Fatalf("expected most recent log first, got %#v", logs[0])
	}
}
