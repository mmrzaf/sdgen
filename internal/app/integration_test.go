package app

import (
	"os"
	"testing"
	"time"

	"github.com/mmrzaf/sdgen/internal/domain"
	"github.com/mmrzaf/sdgen/internal/infra/repos/runs"
	"github.com/mmrzaf/sdgen/internal/infra/repos/scenarios"
	"github.com/mmrzaf/sdgen/internal/infra/repos/targets"
	"github.com/mmrzaf/sdgen/internal/logging"
	"github.com/mmrzaf/sdgen/internal/registry"
)

func TestTargetTestAndPlan_SQLite(t *testing.T) {
	dbf, err := os.CreateTemp("", "sdgen_int_*.db")
	if err != nil {
		t.Fatal(err)
	}
	_ = dbf.Close()
	defer os.Remove(dbf.Name())

	runRepo := runs.NewSQLiteRepository(dbf.Name())
	if err := runRepo.Init(); err != nil {
		t.Fatal(err)
	}
	targetRepo := targets.NewSQLiteRepository(runRepo.DB())
	scRepo := scenarios.NewFileRepository("./does-not-matter")
	logger := logging.NewLogger("error")
	genRegistry := registry.DefaultGeneratorRegistry()
	svc := NewRunService(scRepo, targetRepo, runRepo, genRegistry, logger, 1000)

	tgt := &domain.TargetConfig{
		Name: "sqlite1",
		Kind: "sqlite",
		DSN:  dbf.Name(),
	}
	if err := targetRepo.Create(tgt); err != nil {
		t.Fatal(err)
	}

	check, err := svc.TestTarget(tgt.ID)
	if check == nil {
		t.Fatalf("expected check, err=%v", err)
	}
	if !check.OK {
		t.Fatalf("expected OK, got: %#v", check)
	}

	scale := 2.0
	req := &domain.RunRequest{
		Scenario: &domain.Scenario{
			ID:      "inline",
			Name:    "s1",
			Version: "1",
			Entities: []domain.Entity{
				{
					Name:        "users",
					TargetTable: "users",
					Rows:        10,
					Columns: []domain.Column{
						{Name: "id", Type: domain.ColumnTypeInt, Generator: domain.GeneratorSpec{Type: "uniform_int", Params: map[string]interface{}{"min": 1, "max": 100000}}},
					},
				},
			},
		},
		TargetID: tgt.ID,
		Scale:    &scale,
		EntityScales: map[string]float64{
			"users": 1.5,
		},
		EntityCounts: map[string]int64{
			"users":   25,
			"unknown": 10,
		},
		Mode: "create",
	}

	beforeRuns, err := svc.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeRuns) != 0 {
		t.Fatalf("expected no runs before plan, got %d", len(beforeRuns))
	}

	plan, err := svc.PlanRun(req)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scale != 2.0 {
		t.Fatalf("expected scale 2, got %v", plan.Scale)
	}
	if plan.ResolvedCounts["users"] != 25 {
		t.Fatalf("expected explicit entity_count override to win, got %v", plan.ResolvedCounts)
	}
	if len(plan.ExecutionOrder) != 1 || plan.ExecutionOrder[0] != "users" {
		t.Fatalf("unexpected execution order: %#v", plan.ExecutionOrder)
	}
	if len(plan.Warnings) == 0 {
		t.Fatalf("expected warnings for unknown entity overrides, got %#v", plan.Warnings)
	}

	afterRuns, err := svc.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterRuns) != 0 {
		t.Fatalf("plan should not create run rows; got %d", len(afterRuns))
	}
}

func TestPlanRun_AppliesEntityScaleBeforeExplicitCounts(t *testing.T) {
	dbf, err := os.CreateTemp("", "sdgen_int_*.db")
	if err != nil {
		t.Fatal(err)
	}
	_ = dbf.Close()
	defer os.Remove(dbf.Name())

	runRepo := runs.NewSQLiteRepository(dbf.Name())
	if err := runRepo.Init(); err != nil {
		t.Fatal(err)
	}
	targetRepo := targets.NewSQLiteRepository(runRepo.DB())
	scRepo := scenarios.NewFileRepository("./does-not-matter")
	logger := logging.NewLogger("error")
	svc := NewRunService(scRepo, targetRepo, runRepo, registry.DefaultGeneratorRegistry(), logger, 1000)

	tgt := &domain.TargetConfig{Name: "sqlite2", Kind: "sqlite", DSN: dbf.Name()}
	if err := targetRepo.Create(tgt); err != nil {
		t.Fatal(err)
	}

	scale := 2.0
	req := &domain.RunRequest{
		Scenario: &domain.Scenario{
			ID:      "inline2",
			Name:    "s2",
			Version: "1",
			Entities: []domain.Entity{
				{
					Name:        "events",
					TargetTable: "events",
					Rows:        10,
					Columns: []domain.Column{
						{Name: "id", Type: domain.ColumnTypeInt, Generator: domain.GeneratorSpec{Type: "uniform_int", Params: map[string]interface{}{"min": 1, "max": 9999}}},
					},
				},
			},
		},
		TargetID: tgt.ID,
		Scale:    &scale,
		EntityScales: map[string]float64{
			"events": 3.0,
		},
		EntityCounts: map[string]int64{
			"events": 55,
		},
		Mode: "create",
	}
	plan, err := svc.PlanRun(req)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ResolvedCounts["events"] != 55 {
		t.Fatalf("explicit entity_counts should win over scale and entity_scales, got %#v", plan.ResolvedCounts)
	}
}

func TestPlanRun_IncludeExcludeEntities(t *testing.T) {
	dbf, err := os.CreateTemp("", "sdgen_int_*.db")
	if err != nil {
		t.Fatal(err)
	}
	_ = dbf.Close()
	defer os.Remove(dbf.Name())

	runRepo := runs.NewSQLiteRepository(dbf.Name())
	if err := runRepo.Init(); err != nil {
		t.Fatal(err)
	}
	targetRepo := targets.NewSQLiteRepository(runRepo.DB())
	scRepo := scenarios.NewFileRepository("./does-not-matter")
	logger := logging.NewLogger("error")
	svc := NewRunService(scRepo, targetRepo, runRepo, registry.DefaultGeneratorRegistry(), logger, 1000)

	tgt := &domain.TargetConfig{Name: "sqlite3", Kind: "sqlite", DSN: dbf.Name()}
	if err := targetRepo.Create(tgt); err != nil {
		t.Fatal(err)
	}

	req := &domain.RunRequest{
		Scenario: &domain.Scenario{
			ID:      "inline3",
			Name:    "s3",
			Version: "1",
			Entities: []domain.Entity{
				{
					Name:        "users",
					TargetTable: "users",
					Rows:        10,
					Columns: []domain.Column{
						{Name: "id", Type: domain.ColumnTypeInt, Generator: domain.GeneratorSpec{Type: "uniform_int", Params: map[string]interface{}{"min": 1, "max": 9999}}},
					},
				},
				{
					Name:        "events",
					TargetTable: "events",
					Rows:        100,
					Columns: []domain.Column{
						{Name: "id", Type: domain.ColumnTypeInt, Generator: domain.GeneratorSpec{Type: "uniform_int", Params: map[string]interface{}{"min": 1, "max": 999999}}},
					},
				},
			},
		},
		TargetID:        tgt.ID,
		Mode:            "create",
		IncludeEntities: []string{"users", "events"},
		ExcludeEntities: []string{"events"},
		EntityCounts:    map[string]int64{"events": 77},
		EntityScales:    map[string]float64{"events": 2},
	}
	plan, err := svc.PlanRun(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ExecutionOrder) != 1 || plan.ExecutionOrder[0] != "users" {
		t.Fatalf("expected only users entity, got %#v", plan.ExecutionOrder)
	}
	if _, ok := plan.ResolvedCounts["events"]; ok {
		t.Fatalf("excluded entity should not be in resolved counts: %#v", plan.ResolvedCounts)
	}
	if len(plan.Warnings) == 0 {
		t.Fatalf("expected warning for excluded entity overrides, got %#v", plan.Warnings)
	}
}

func TestStartRun_CompletesSuccess_SQLite(t *testing.T) {
	dbf, err := os.CreateTemp("", "sdgen_int_target_*.db")
	if err != nil {
		t.Fatal(err)
	}
	_ = dbf.Close()
	defer os.Remove(dbf.Name())

	runsDB, err := os.CreateTemp("", "sdgen_int_runs_*.db")
	if err != nil {
		t.Fatal(err)
	}
	_ = runsDB.Close()
	defer os.Remove(runsDB.Name())

	runRepo := runs.NewSQLiteRepository(runsDB.Name())
	if err := runRepo.Init(); err != nil {
		t.Fatal(err)
	}
	targetRepo := targets.NewSQLiteRepository(runRepo.DB())
	scRepo := scenarios.NewFileRepository("./does-not-matter")
	logger := logging.NewLogger("error")
	svc := NewRunService(scRepo, targetRepo, runRepo, registry.DefaultGeneratorRegistry(), logger, 1000)

	req := &domain.RunRequest{
		Scenario: &domain.Scenario{
			ID:      "inline-run",
			Name:    "run-scenario",
			Version: "1",
			Entities: []domain.Entity{
				{
					Name:        "users",
					TargetTable: "users",
					Rows:        20,
					Columns: []domain.Column{
						{Name: "id", Type: domain.ColumnTypeInt, Generator: domain.GeneratorSpec{Type: "uniform_int", Params: map[string]interface{}{"min": 1, "max": 99999}}},
						{Name: "name", Type: domain.ColumnTypeString, Generator: domain.GeneratorSpec{Type: "faker_name"}},
					},
				},
			},
		},
		Target: &domain.TargetConfig{
			Name: "inline-sqlite",
			Kind: "sqlite",
			DSN:  dbf.Name(),
		},
		Mode: "create",
	}

	run, err := svc.StartRun(req)
	if err != nil {
		t.Fatal(err)
	}
	if run == nil || run.ID == "" {
		t.Fatalf("expected run id, got %#v", run)
	}

	deadline := time.Now().Add(8 * time.Second)
	for {
		cur, err := svc.GetRun(run.ID)
		if err != nil {
			t.Fatal(err)
		}
		if cur.Status == domain.RunStatusSuccess {
			return
		}
		if cur.Status == domain.RunStatusFailed {
			t.Fatalf("run failed: %s", cur.Error)
		}
		if time.Now().After(deadline) {
			t.Fatalf("run did not complete by deadline, last status=%s", cur.Status)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
