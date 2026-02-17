package main

import (
	"flag"
	"net/http"
	"os"
	"time"

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
	sdgenDB := flag.String("db", cfg.SDGenDBDSN, "sdgen metadata database DSN (PostgreSQL)")
	bindAddr := flag.String("bind", cfg.BindAddr, "Bind address")
	logLevel := flag.String("log-level", cfg.LogLevel, "Log level")
	batchSize := flag.Int("batch-size", cfg.BatchSize, "Default insert batch size")
	flag.Parse()

	logger := logging.NewLogger(*logLevel).WithComponent("api_main")

	scenarioRepo := scenarios.NewFileRepository(*scenariosDir)

	runRepo := runs.NewPostgresRepository(*sdgenDB)
	if err := runRepo.Init(); err != nil {
		logger.Errorw("startup.failed", map[string]any{"error": err.Error(), "stage": "init_run_repo"})
		os.Exit(1)
	}
	targetRepo := targets.NewPostgresRepository(runRepo.DB())

	genRegistry := registry.DefaultGeneratorRegistry()
	runService := app.NewRunService(scenarioRepo, targetRepo, runRepo, genRegistry, logger, *batchSize)

	handler := api.NewHandler(scenarioRepo, targetRepo, runService)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", web.IndexHandler)
	mux.HandleFunc("GET /targets", web.TargetsHandler)
	mux.HandleFunc("GET /runs/{id}", web.RunDetailHandler)

	mux.HandleFunc("GET /api/v1/scenarios", handler.ListScenarios)
	mux.HandleFunc("GET /api/v1/scenarios/{id}", handler.GetScenario)

	mux.HandleFunc("GET /api/v1/targets", handler.ListTargets)
	mux.HandleFunc("POST /api/v1/targets", handler.CreateTarget)
	mux.HandleFunc("GET /api/v1/targets/{id}", handler.GetTarget)
	mux.HandleFunc("PUT /api/v1/targets/{id}", handler.UpdateTarget)
	mux.HandleFunc("DELETE /api/v1/targets/{id}", handler.DeleteTarget)
	mux.HandleFunc("POST /api/v1/targets/{id}/test", handler.TestTarget)

	mux.HandleFunc("POST /api/v1/runs", handler.CreateRun)
	mux.HandleFunc("POST /api/v1/runs/plan", handler.PlanRun)
	mux.HandleFunc("GET /api/v1/runs", handler.ListRuns)
	mux.HandleFunc("GET /api/v1/runs/{id}", handler.GetRun)
	mux.HandleFunc("GET /api/v1/runs/{id}/logs", handler.GetRunLogs)

	logger.Infow("startup.listening", map[string]any{"bind": *bindAddr})
	if err := http.ListenAndServe(*bindAddr, loggingMiddleware(logger.WithComponent("http"), mux)); err != nil {
		logger.Errorw("startup.failed", map[string]any{"error": err.Error(), "stage": "listen"})
		os.Exit(1)
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(logger *logging.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		fields := map[string]any{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      sw.status,
			"duration_ms": time.Since(started).Milliseconds(),
			"remote":      r.RemoteAddr,
		}
		if sw.status >= 500 {
			logger.Errorw("request.completed", fields)
			return
		}
		if sw.status >= 400 {
			logger.Warnw("request.completed", fields)
			return
		}
		logger.Infow("request.completed", fields)
	})
}
