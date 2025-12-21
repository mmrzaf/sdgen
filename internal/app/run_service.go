package app

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mmrzaf/sdgen/internal/domain"
	"github.com/mmrzaf/sdgen/internal/exec"
	"github.com/mmrzaf/sdgen/internal/hashing"
	"github.com/mmrzaf/sdgen/internal/infra/repos/runs"
	"github.com/mmrzaf/sdgen/internal/infra/repos/scenarios"
	"github.com/mmrzaf/sdgen/internal/infra/repos/targets"
	"github.com/mmrzaf/sdgen/internal/infra/targets/postgres"
	"github.com/mmrzaf/sdgen/internal/infra/targets/sqlite"
	"github.com/mmrzaf/sdgen/internal/logging"
	"github.com/mmrzaf/sdgen/internal/registry"
	"github.com/mmrzaf/sdgen/internal/validation"
)

type RunService struct {
	scenarioRepo *scenarios.FileRepository
	targetRepo   *targets.FileRepository
	runRepo      runs.Repository
	validator    *validation.Validator
	executor     *exec.Executor
	logger       *logging.Logger
}

func NewRunService(
	scenarioRepo *scenarios.FileRepository,
	targetRepo *targets.FileRepository,
	runRepo runs.Repository,
	genRegistry *registry.GeneratorRegistry,
	logger *logging.Logger,
) *RunService {
	return &RunService{
		scenarioRepo: scenarioRepo,
		targetRepo:   targetRepo,
		runRepo:      runRepo,
		validator:    validation.NewValidator(genRegistry),
		executor:     exec.NewExecutor(genRegistry),
		logger:       logger,
	}
}

func (s *RunService) StartRun(req *domain.RunRequest) (*domain.Run, error) {
	if err := s.validator.ValidateRunRequest(req); err != nil {
		return nil, fmt.Errorf("invalid run request: %w", err)
	}

	var scenario *domain.Scenario
	var err error

	if req.ScenarioID != "" {
		scenario, err = s.scenarioRepo.Get(req.ScenarioID)
		if err != nil {
			return nil, fmt.Errorf("failed to load scenario: %w", err)
		}
	} else {
		scenario = req.Scenario
	}

	if req.RowOverrides != nil {
		for i := range scenario.Entities {
			if newRows, ok := req.RowOverrides[scenario.Entities[i].Name]; ok {
				scenario.Entities[i].Rows = newRows
			}
		}
	}

	if err := s.validator.ValidateScenario(scenario); err != nil {
		return nil, fmt.Errorf("scenario validation failed: %w", err)
	}

	var targetCfg *domain.TargetConfig
	if req.TargetID != "" {
		targetCfg, err = s.targetRepo.Get(req.TargetID)
		if err != nil {
			return nil, fmt.Errorf("failed to load target: %w", err)
		}
	} else {
		targetCfg = req.Target
	}

	if err := s.validator.ValidateTarget(targetCfg); err != nil {
		return nil, fmt.Errorf("target validation failed: %w", err)
	}

	seed := int64(0)
	if req.Seed != nil {
		seed = *req.Seed
	} else if scenario.Seed != nil {
		seed = *scenario.Seed
	} else {
		seed = generateSeed()
	}

	configHash, err := hashing.HashScenario(scenario)
	if err != nil {
		return nil, fmt.Errorf("failed to hash scenario: %w", err)
	}

	run := &domain.Run{
		ScenarioID:      scenario.ID,
		ScenarioName:    scenario.Name,
		ScenarioVersion: scenario.Version,
		TargetID:        targetCfg.ID,
		TargetName:      targetCfg.Name,
		TargetKind:      targetCfg.Kind,
		Seed:            seed,
		ConfigHash:      configHash,
		Status:          domain.RunStatusRunning,
		StartedAt:       time.Now(),
	}

	if err := s.runRepo.Create(run); err != nil {
		return nil, fmt.Errorf("failed to create run: %w", err)
	}

	s.logger.Info("Starting run %s: scenario=%s, target=%s, seed=%d", run.ID, scenario.Name, targetCfg.Name, seed)

	go s.executeRun(run, scenario, targetCfg, seed, req.Mode)

	return run, nil
}

func (s *RunService) executeRun(run *domain.Run, scenario *domain.Scenario, targetCfg *domain.TargetConfig, seed int64, mode string) {
	var target exec.Target
	switch targetCfg.Kind {
	case "postgres":
		schema := targetCfg.Schema
		if schema == "" {
			schema = "public"
		}
		target = postgres.NewPostgresTarget(targetCfg.DSN, schema)
	case "sqlite":
		target = sqlite.NewSQLiteTarget(targetCfg.DSN)
	default:
		s.updateRunFailed(run, fmt.Sprintf("unsupported target kind: %s", targetCfg.Kind))
		return
	}

	if mode == "" {
		mode = domain.TableModeCreateIfMissing
	}

	stats, err := s.executor.Execute(scenario, target, seed, mode)
	if err != nil {
		s.logger.Error("Run %s failed: %v", run.ID, err)
		s.updateRunFailed(run, err.Error())
		return
	}

	now := time.Now()
	duration := now.Sub(run.StartedAt)
	stats.DurationSeconds = duration.Seconds()

	statsJSON, _ := json.Marshal(stats)
	run.Stats = statsJSON
	run.Status = domain.RunStatusSuccess
	run.CompletedAt = &now

	if err := s.runRepo.Update(run); err != nil {
		s.logger.Error("Failed to update run %s: %v", run.ID, err)
	}

	s.logger.Info("Run %s completed: %d entities, %d total rows, %.2fs",
		run.ID, stats.EntitiesGenerated, stats.TotalRows, stats.DurationSeconds)
}

func (s *RunService) updateRunFailed(run *domain.Run, errorMsg string) {
	now := time.Now()
	run.Status = domain.RunStatusFailed
	run.Error = errorMsg
	run.CompletedAt = &now
	if err := s.runRepo.Update(run); err != nil {
		s.logger.Error("Failed to update run %s: %v", run.ID, err)
	}
}

func (s *RunService) GetRun(id string) (*domain.Run, error) {
	return s.runRepo.Get(id)
}

func (s *RunService) ListRuns(limit int, status string) ([]*domain.Run, error) {
	return s.runRepo.List(limit, status)
}

func generateSeed() int64 {
	var b [8]byte
	rand.Read(b[:])
	return int64(binary.LittleEndian.Uint64(b[:]))
}
