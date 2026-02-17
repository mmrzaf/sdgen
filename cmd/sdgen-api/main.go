package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/mmrzaf/sdgen/internal/api"
	"github.com/mmrzaf/sdgen/internal/app"
	"github.com/mmrzaf/sdgen/internal/config"
	"github.com/mmrzaf/sdgen/internal/infra/repos/runs"
	"github.com/mmrzaf/sdgen/internal/infra/repos/scenarios"
	"github.com/mmrzaf/sdgen/internal/infra/repos/targets"
	"github.com/mmrzaf/sdgen/internal/logging"
	"github.com/mmrzaf/sdgen/internal/registry"
	"github.com/mmrzaf/sdgen/internal/web"
)

func main() {
	cfg := config.Load()

	scenariosDir := flag.String("scenarios-dir", cfg.ScenariosDir, "Scenarios directory")
	targetsDir := flag.String("targets-dir", cfg.TargetsDir, "Targets directory")
	runsDB := flag.String("runs-db", cfg.RunsDBPath, "Runs database path")
	bindAddr := flag.String("bind", cfg.BindAddr, "Bind address")
	logLevel := flag.String("log-level", cfg.LogLevel, "Log level")
	flag.Parse()

	logger := logging.NewLogger(*logLevel)
	logger.Info("Starting sdgen API server")

	scenarioRepo := scenarios.NewFileRepository(*scenariosDir)
	targetRepo := targets.NewFileRepository(*targetsDir)

	runRepo := runs.NewSQLiteRepository(*runsDB)
	if err := runRepo.Init(); err != nil {
		log.Fatalf("Failed to initialize run repository: %v", err)
	}

	genRegistry := registry.DefaultGeneratorRegistry()
	runService := app.NewRunService(scenarioRepo, targetRepo, runRepo, genRegistry, logger)

	handler := api.NewHandler(scenarioRepo, targetRepo, runService)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", web.IndexHandler)
	mux.HandleFunc("GET /runs/{id}", web.RunDetailHandler)

	mux.HandleFunc("GET /api/v1/scenarios", handler.ListScenarios)
	mux.HandleFunc("GET /api/v1/scenarios/{id}", handler.GetScenario)
	mux.HandleFunc("GET /api/v1/targets", handler.ListTargets)
	mux.HandleFunc("GET /api/v1/targets/{id}", handler.GetTarget)
	mux.HandleFunc("POST /api/v1/runs", handler.CreateRun)
	mux.HandleFunc("GET /api/v1/runs", handler.ListRuns)
	mux.HandleFunc("GET /api/v1/runs/{id}", handler.GetRun)

	logger.Info("Listening on %s", *bindAddr)
	if err := http.ListenAndServe(*bindAddr, mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
