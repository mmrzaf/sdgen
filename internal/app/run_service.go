package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/mmrzaf/sdgen/internal/domain"
	"github.com/mmrzaf/sdgen/internal/exec"
	"github.com/mmrzaf/sdgen/internal/hashing"
	"github.com/mmrzaf/sdgen/internal/infra/repos/runs"
	"github.com/mmrzaf/sdgen/internal/infra/repos/scenarios"
	"github.com/mmrzaf/sdgen/internal/infra/repos/targets"
	esTarget "github.com/mmrzaf/sdgen/internal/infra/targets/elasticsearch"
	pgTarget "github.com/mmrzaf/sdgen/internal/infra/targets/postgres"
	sqliteTarget "github.com/mmrzaf/sdgen/internal/infra/targets/sqlite"
	"github.com/mmrzaf/sdgen/internal/logging"
	"github.com/mmrzaf/sdgen/internal/registry"
	"github.com/mmrzaf/sdgen/internal/validation"
)

type RunService struct {
	scenarioRepo *scenarios.FileRepository
	targetRepo   targets.Repository
	runRepo      *runs.SQLiteRepository
	genRegistry  *registry.GeneratorRegistry
	validator    *validation.Validator
	logger       *logging.Logger
	batchSize    int
}

func NewRunService(
	scenarioRepo *scenarios.FileRepository,
	targetRepo targets.Repository,
	runRepo *runs.SQLiteRepository,
	genRegistry *registry.GeneratorRegistry,
	logger *logging.Logger,
	batchSize int,
) *RunService {
	if batchSize <= 0 {
		batchSize = 1000
	}
	if logger == nil {
		logger = logging.NewLogger("info")
	}
	return &RunService{
		scenarioRepo: scenarioRepo,
		targetRepo:   targetRepo,
		runRepo:      runRepo,
		genRegistry:  genRegistry,
		validator:    validation.NewValidator(genRegistry),
		logger:       logger.WithComponent("run_service"),
		batchSize:    batchSize,
	}
}

func (s *RunService) Validator() *validation.Validator { return s.validator }

func (s *RunService) StartRun(req *domain.RunRequest) (*domain.Run, error) {
	s.logger.Debugw("start_run.request_received", map[string]any{
		"scenario_id":         req.ScenarioID,
		"target_id":           req.TargetID,
		"mode":                req.Mode,
		"has_inline_scenario": req.Scenario != nil,
		"has_inline_target":   req.Target != nil,
	})
	if err := s.validator.ValidateRunRequest(req); err != nil {
		s.logger.Warnw("start_run.validation_failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	scenario, err := s.loadScenario(req)
	if err != nil {
		return nil, err
	}
	target, err := s.loadTarget(req)
	if err != nil {
		return nil, err
	}
	target = resolveTargetForRun(target, req.TargetDatabase)

	if err := s.validator.ValidateScenario(scenario); err != nil {
		return nil, err
	}
	if err := s.validator.ValidateTarget(target); err != nil {
		return nil, err
	}

	seed := s.resolveSeed(req, scenario)
	mode := req.Mode

	plan, resolvedScenario, err := s.buildPlanAndResolvedScenario(scenario, req)
	if err != nil {
		return nil, err
	}

	cfgHash, err := hashing.HashRunConfig(resolvedScenario, target, mode, plan.Scale, plan.ResolvedCounts, seed)
	if err != nil {
		return nil, err
	}

	rcJSON, _ := json.Marshal(plan.ResolvedCounts)
	eoJSON, _ := json.Marshal(plan.ExecutionOrder)
	wJSON, _ := json.Marshal(plan.Warnings)

	run := &domain.Run{
		ID:                    uuid.NewString(),
		ScenarioID:            scenario.ID,
		ScenarioName:          scenario.Name,
		ScenarioVersion:       scenario.Version,
		TargetID:              target.ID,
		TargetName:            target.Name,
		TargetKind:            target.Kind,
		Seed:                  seed,
		Mode:                  mode,
		Scale:                 &plan.Scale,
		ResolvedCounts:        json.RawMessage(rcJSON),
		ExecutionOrder:        json.RawMessage(eoJSON),
		Warnings:              json.RawMessage(wJSON),
		ConfigHash:            cfgHash,
		Status:                domain.RunStatusRunning,
		StartedAt:             time.Now().UTC(),
		ProgressRowsTotal:     sumCounts(plan.ResolvedCounts),
		ProgressEntitiesTotal: len(plan.ExecutionOrder),
	}

	if err := s.runRepo.Create(run); err != nil {
		s.logger.Errorw("start_run.persist_failed", map[string]any{"run_id": run.ID, "error": err.Error()})
		return nil, err
	}
	s.logger.Infow("start_run.accepted", map[string]any{
		"run_id":         run.ID,
		"scenario_id":    run.ScenarioID,
		"target_kind":    run.TargetKind,
		"mode":           run.Mode,
		"resolved_count": len(plan.ResolvedCounts),
		"warning_count":  len(plan.Warnings),
	})

	go s.executeRun(run, resolvedScenario, target, mode)
	return run, nil
}

func (s *RunService) PlanRun(req *domain.RunRequest) (*domain.RunPlan, error) {
	if err := s.validator.ValidateRunRequest(req); err != nil {
		s.logger.Warnw("plan_run.validation_failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	scenario, err := s.loadScenario(req)
	if err != nil {
		return nil, err
	}
	if err := s.validator.ValidateScenario(scenario); err != nil {
		return nil, err
	}

	target, err := s.loadTarget(req)
	if err != nil {
		return nil, err
	}
	target = resolveTargetForRun(target, req.TargetDatabase)
	if err := s.validator.ValidateTarget(target); err != nil {
		return nil, err
	}

	plan, _, err := s.buildPlanAndResolvedScenario(scenario, req)
	if err != nil {
		s.logger.Warnw("plan_run.failed", map[string]any{"error": err.Error()})
		return nil, err
	}
	s.logger.Infow("plan_run.completed", map[string]any{
		"scenario_id":     scenario.ID,
		"target_kind":     target.Kind,
		"entity_count":    len(plan.ResolvedCounts),
		"execution_steps": len(plan.ExecutionOrder),
		"warning_count":   len(plan.Warnings),
	})
	return plan, nil
}

func (s *RunService) TestTarget(targetID string) (*domain.TargetCheck, error) {
	s.logger.Debugw("target_test.started", map[string]any{"target_id": targetID})
	t, err := s.targetRepo.Get(targetID)
	if err != nil {
		s.logger.Warnw("target_test.load_failed", map[string]any{"target_id": targetID, "error": err.Error()})
		return nil, err
	}
	if err := s.validator.ValidateTarget(t); err != nil {
		c := &domain.TargetCheck{
			ID:        uuid.NewString(),
			TargetID:  t.ID,
			CheckedAt: time.Now().UTC(),
			OK:        false,
			Error:     err.Error(),
		}
		_ = s.targetRepo.RecordCheck(c)
		return c, err
	}

	check, err := CheckTarget(t)
	if check != nil {
		_ = s.targetRepo.RecordCheck(check)
	}
	if err != nil {
		s.logger.Warnw("target_test.failed", map[string]any{"target_id": targetID, "error": err.Error()})
	} else if check != nil {
		s.logger.Infow("target_test.completed", map[string]any{
			"target_id":    targetID,
			"ok":           check.OK,
			"latency_ms":   check.LatencyMS,
			"server_ver":   check.ServerVer,
			"can_create":   check.Capabilities.CanCreate,
			"can_insert":   check.Capabilities.CanInsert,
			"can_truncate": check.Capabilities.CanTruncate,
		})
	}
	return check, err
}

func (s *RunService) ListRuns(limit int) ([]*domain.Run, error) {
	return s.runRepo.List(limit, "")
}

func (s *RunService) GetRun(id string) (*domain.Run, error) {
	return s.runRepo.Get(id)
}

func (s *RunService) ListRunLogs(id string, limit int) ([]*domain.RunLog, error) {
	return s.runRepo.ListRunLogs(id, limit)
}

func (s *RunService) loadScenario(req *domain.RunRequest) (*domain.Scenario, error) {
	if req.Scenario != nil {
		return req.Scenario, nil
	}
	return s.scenarioRepo.Get(req.ScenarioID)
}

func (s *RunService) loadTarget(req *domain.RunRequest) (*domain.TargetConfig, error) {
	if req.Target != nil {
		return req.Target, nil
	}
	return s.targetRepo.Get(req.TargetID)
}

func (s *RunService) resolveSeed(req *domain.RunRequest, scenario *domain.Scenario) int64 {
	if req.Seed != nil {
		return *req.Seed
	}
	if scenario.Seed != nil {
		return *scenario.Seed
	}
	return time.Now().UnixNano()
}

func (s *RunService) buildPlanAndResolvedScenario(scenario *domain.Scenario, req *domain.RunRequest) (*domain.RunPlan, *domain.Scenario, error) {
	scale := 1.0
	if req.Scale != nil {
		scale = *req.Scale
	}

	warnings := make([]string, 0)
	includeSet := make(map[string]struct{}, len(req.IncludeEntities))
	excludeSet := make(map[string]struct{}, len(req.ExcludeEntities))
	for _, name := range req.IncludeEntities {
		includeSet[name] = struct{}{}
	}
	for _, name := range req.ExcludeEntities {
		excludeSet[name] = struct{}{}
	}

	// Copy scenario so we never mutate the file-backed source.
	resolved := *scenario
	resolved.Entities = make([]domain.Entity, 0, len(scenario.Entities))
	present := make(map[string]struct{}, len(scenario.Entities))
	for _, entity := range scenario.Entities {
		present[entity.Name] = struct{}{}
		if len(includeSet) > 0 {
			if _, ok := includeSet[entity.Name]; !ok {
				continue
			}
		}
		if _, ok := excludeSet[entity.Name]; ok {
			warnings = append(warnings, fmt.Sprintf("entity %q was excluded from this run", entity.Name))
			continue
		}
		resolved.Entities = append(resolved.Entities, entity)
	}
	for name := range includeSet {
		if _, ok := present[name]; !ok {
			warnings = append(warnings, fmt.Sprintf("include_entities references unknown entity %q", name))
		}
	}
	for name := range excludeSet {
		if _, ok := present[name]; !ok {
			warnings = append(warnings, fmt.Sprintf("exclude_entities references unknown entity %q", name))
		}
	}
	if len(resolved.Entities) == 0 {
		return nil, nil, errors.New("no entities selected for run after applying include/exclude filters")
	}

	resolvedCounts := make(map[string]int64, len(resolved.Entities))

	for i := range resolved.Entities {
		e := resolved.Entities[i]

		base := float64(e.Rows)
		scaled := int64(math.Round(base * scale))
		if scaled < 1 {
			scaled = 1
			warnings = append(warnings, fmt.Sprintf("entity %q scaled below 1 row; clamped to 1", e.Name))
		}
		if req.EntityScales != nil {
			if entityScale, ok := req.EntityScales[e.Name]; ok {
				scaled = int64(math.Round(float64(scaled) * entityScale))
				if scaled < 1 {
					scaled = 1
					warnings = append(warnings, fmt.Sprintf("entity %q entity_scale produced below 1 row; clamped to 1", e.Name))
				}
			}
		}

		count := scaled
		if req.EntityCounts != nil {
			if ov, ok := req.EntityCounts[e.Name]; ok {
				count = ov
			}
		}

		resolved.Entities[i].Rows = count
		resolvedCounts[e.Name] = count
	}
	if req.EntityCounts != nil {
		for name := range req.EntityCounts {
			if _, ok := resolvedCounts[name]; !ok {
				warnings = append(warnings, fmt.Sprintf("entity_count override for unknown entity %q ignored", name))
			}
		}
	}
	if req.EntityScales != nil {
		for name := range req.EntityScales {
			if _, ok := resolvedCounts[name]; !ok {
				warnings = append(warnings, fmt.Sprintf("entity_scale override for unknown or excluded entity %q ignored", name))
			}
		}
	}
	if err := s.validator.ValidateScenario(&resolved); err != nil {
		return nil, nil, err
	}

	executionOrder, err := validation.TopologicalSort(&resolved)
	if err != nil {
		return nil, nil, err
	}

	plan := &domain.RunPlan{
		Scale:          scale,
		ResolvedCounts: resolvedCounts,
		ExecutionOrder: executionOrder,
		Warnings:       warnings,
	}

	return plan, &resolved, nil
}

func (s *RunService) executeRun(run *domain.Run, scenario *domain.Scenario, targetCfg *domain.TargetConfig, mode string) {
	started := time.Now()
	s.logger.Infow("run_execution.started", map[string]any{
		"run_id":      run.ID,
		"target_kind": targetCfg.Kind,
		"mode":        mode,
		"entities":    len(scenario.Entities),
	})
	// Build concrete exec.Target
	var tgt exec.Target
	switch targetCfg.Kind {
	case "sqlite":
		tgt = sqliteTarget.NewSQLiteTarget(targetCfg.DSN)
	case "postgres":
		schema := targetCfg.Schema
		if schema == "" {
			schema = "public"
		}
		tgt = pgTarget.NewPostgresTarget(targetCfg.DSN, schema)
	case "elasticsearch":
		tgt = esTarget.NewElasticsearchTarget(targetCfg.DSN)
	default:
		_ = s.runRepo.UpdateStatus(run.ID, domain.RunStatusFailed, fmt.Sprintf("unsupported target kind: %s", targetCfg.Kind), nil)
		s.logger.Errorw("run_execution.failed", map[string]any{"run_id": run.ID, "error": fmt.Sprintf("unsupported target kind: %s", targetCfg.Kind)})
		return
	}

	executor := exec.NewExecutor(s.genRegistry, s.batchSize)

	rowsGenerated := int64(0)
	entitiesDone := 0
	_ = s.runRepo.AppendRunLog(run.ID, "info", "run started")
	stats, err := executor.Execute(scenario, tgt, run.Seed, mode, func(ev exec.ProgressEvent) {
		if ev.EntityStarted {
			_ = s.runRepo.AppendRunLog(run.ID, "info", fmt.Sprintf("entity %s started", ev.EntityName))
			_ = s.runRepo.UpdateProgress(run.ID, rowsGenerated, run.ProgressRowsTotal, entitiesDone, run.ProgressEntitiesTotal, ev.EntityName)
		}
		if ev.RowsDelta > 0 {
			rowsGenerated += ev.RowsDelta
			_ = s.runRepo.UpdateProgress(run.ID, rowsGenerated, run.ProgressRowsTotal, entitiesDone, run.ProgressEntitiesTotal, ev.EntityName)
		}
		if ev.EntityCompleted {
			entitiesDone = ev.EntitiesDone
			_ = s.runRepo.AppendRunLog(run.ID, "info", fmt.Sprintf("entity %s completed", ev.EntityName))
			_ = s.runRepo.UpdateProgress(run.ID, rowsGenerated, run.ProgressRowsTotal, entitiesDone, run.ProgressEntitiesTotal, "")
		}
	})
	if err != nil {
		_ = s.runRepo.UpdateStatus(run.ID, domain.RunStatusFailed, err.Error(), nil)
		_ = s.runRepo.AppendRunLog(run.ID, "error", fmt.Sprintf("run failed: %s", err.Error()))
		s.logger.Errorw("run_execution.failed", map[string]any{"run_id": run.ID, "error": err.Error()})
		return
	}

	_ = s.runRepo.UpdateStatus(run.ID, domain.RunStatusSuccess, "", stats)
	_ = s.runRepo.UpdateProgress(run.ID, run.ProgressRowsTotal, run.ProgressRowsTotal, run.ProgressEntitiesTotal, run.ProgressEntitiesTotal, "")
	_ = s.runRepo.AppendRunLog(run.ID, "info", fmt.Sprintf("run completed successfully: total_rows=%d", stats.TotalRows))
	s.logger.Infow("run_execution.completed", map[string]any{
		"run_id":      run.ID,
		"total_rows":  stats.TotalRows,
		"entities":    stats.EntitiesGenerated,
		"duration_ms": time.Since(started).Milliseconds(),
	})
}

func sumCounts(m map[string]int64) int64 {
	var n int64
	for _, v := range m {
		n += v
	}
	return n
}
