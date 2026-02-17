package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mmrzaf/sdgen/internal/app"
	"github.com/mmrzaf/sdgen/internal/config"
	"github.com/mmrzaf/sdgen/internal/domain"
	"github.com/mmrzaf/sdgen/internal/infra/repos/runs"
	"github.com/mmrzaf/sdgen/internal/infra/repos/scenarios"
	"github.com/mmrzaf/sdgen/internal/infra/repos/targets"
	"github.com/mmrzaf/sdgen/internal/logging"
	"github.com/mmrzaf/sdgen/internal/registry"
	"github.com/mmrzaf/sdgen/internal/validation"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	scenariosDir string
	targetsDir   string
	runsDBPath   string
	logLevel     string
)

func main() {
	cfg := config.Load()

	rootCmd := &cobra.Command{
		Use:   "sdgen",
		Short: "Synthetic data generator",
	}

	rootCmd.PersistentFlags().StringVar(&scenariosDir, "scenarios-dir", cfg.ScenariosDir, "Scenarios directory")
	rootCmd.PersistentFlags().StringVar(&targetsDir, "targets-dir", cfg.TargetsDir, "Targets directory")
	rootCmd.PersistentFlags().StringVar(&runsDBPath, "runs-db", cfg.RunsDBPath, "Runs database path")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", cfg.LogLevel, "Log level")

	rootCmd.AddCommand(scenarioCmd())
	rootCmd.AddCommand(targetCmd())
	rootCmd.AddCommand(runCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func scenarioCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scenario",
		Short: "Manage scenarios",
	}

	var format string

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List scenarios",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := scenarios.NewFileRepository(scenariosDir)
			list, err := repo.List()
			if err != nil {
				return err
			}

			if format == "json" {
				data, _ := json.MarshalIndent(list, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tVERSION\tENTITIES")
			for _, s := range list {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", s.ID, s.Name, s.Version, len(s.Entities))
			}
			w.Flush()
			return nil
		},
	}
	listCmd.Flags().StringVar(&format, "format", "table", "Output format (table|json)")

	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show scenario details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := scenarios.NewFileRepository(scenariosDir)
			scenario, err := repo.Get(args[0])
			if err != nil {
				return err
			}

			data, _ := yaml.Marshal(scenario)
			fmt.Println(string(data))
			return nil
		},
	}

	validateCmd := &cobra.Command{
		Use:   "validate <id|path>",
		Short: "Validate a scenario",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := scenarios.NewFileRepository(scenariosDir)
			var scenario *domain.Scenario
			var err error

			if strings.Contains(args[0], "/") || strings.HasSuffix(args[0], ".yaml") || strings.HasSuffix(args[0], ".yml") {
				scenario, err = repo.GetByPath(args[0])
			} else {
				scenario, err = repo.Get(args[0])
			}

			if err != nil {
				return err
			}

			genRegistry := registry.DefaultGeneratorRegistry()
			validator := validation.NewValidator(genRegistry)

			if err := validator.ValidateScenario(scenario); err != nil {
				fmt.Printf("Validation failed: %v\n", err)
				return err
			}

			fmt.Printf("Scenario '%s' is valid\n", scenario.Name)
			return nil
		},
	}

	cmd.AddCommand(listCmd, showCmd, validateCmd)
	return cmd
}

func targetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "target",
		Short: "Manage targets",
	}

	var format string

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := targets.NewFileRepository(targetsDir)
			list, err := repo.List()
			if err != nil {
				return err
			}

			if format == "json" {
				data, _ := json.MarshalIndent(list, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tKIND\tDSN")
			for _, t := range list {
				dsn := t.DSN
				if len(dsn) > 50 {
					dsn = dsn[:47] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.ID, t.Name, t.Kind, dsn)
			}
			w.Flush()
			return nil
		},
	}
	listCmd.Flags().StringVar(&format, "format", "table", "Output format (table|json)")

	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show target details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := targets.NewFileRepository(targetsDir)
			target, err := repo.Get(args[0])
			if err != nil {
				return err
			}

			data, _ := yaml.Marshal(target)
			fmt.Println(string(data))
			return nil
		},
	}

	validateCmd := &cobra.Command{
		Use:   "validate <id|path>",
		Short: "Validate a target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := targets.NewFileRepository(targetsDir)
			var target *domain.TargetConfig
			var err error

			if strings.Contains(args[0], "/") || strings.HasSuffix(args[0], ".yaml") || strings.HasSuffix(args[0], ".yml") {
				target, err = repo.GetByPath(args[0])
			} else {
				target, err = repo.Get(args[0])
			}

			if err != nil {
				return err
			}

			genRegistry := registry.DefaultGeneratorRegistry()
			validator := validation.NewValidator(genRegistry)

			if err := validator.ValidateTarget(target); err != nil {
				fmt.Printf("Validation failed: %v\n", err)
				return err
			}

			fmt.Printf("Target '%s' is valid\n", target.Name)
			return nil
		},
	}

	cmd.AddCommand(listCmd, showCmd, validateCmd)
	return cmd
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Manage runs",
	}

	var (
		scenarioID    string
		scenarioPath  string
		targetID      string
		targetDSN     string
		targetKind    string
		seed          int64
		rowsOverride  []string
		mode          string
		hasSeed       bool
	)

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start a run",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.NewLogger(logLevel)

			scenarioRepo := scenarios.NewFileRepository(scenariosDir)
			targetRepo := targets.NewFileRepository(targetsDir)

			runRepo := runs.NewSQLiteRepository(runsDBPath)
			if err := runRepo.Init(); err != nil {
				return err
			}

			genRegistry := registry.DefaultGeneratorRegistry()
			runService := app.NewRunService(scenarioRepo, targetRepo, runRepo, genRegistry, logger)

			req := &domain.RunRequest{}

			if scenarioPath != "" {
				scenario, err := scenarioRepo.GetByPath(scenarioPath)
				if err != nil {
					return err
				}
				req.Scenario = scenario
			} else if scenarioID != "" {
				req.ScenarioID = scenarioID
			} else {
				return fmt.Errorf("either --scenario or --scenario-path required")
			}

			if targetDSN != "" {
				if targetKind == "" {
					return fmt.Errorf("--target-kind required when using --target DSN")
				}
				req.Target = &domain.TargetConfig{
					Name: "inline-target",
					Kind: targetKind,
					DSN:  targetDSN,
				}
			} else if targetID != "" {
				req.TargetID = targetID
			} else {
				return fmt.Errorf("either --target-id or --target required")
			}

			if hasSeed {
				req.Seed = &seed
			}

			if len(rowsOverride) > 0 {
				req.RowOverrides = make(map[string]int64)
				for _, override := range rowsOverride {
					parts := strings.SplitN(override, "=", 2)
					if len(parts) != 2 {
						return fmt.Errorf("invalid rows override format: %s", override)
					}
					rows, err := strconv.ParseInt(parts[1], 10, 64)
					if err != nil {
						return fmt.Errorf("invalid rows value: %s", parts[1])
					}
					req.RowOverrides[parts[0]] = rows
				}
			}

			req.Mode = mode

			run, err := runService.StartRun(req)
			if err != nil {
				return err
			}

			fmt.Printf("Run started: %s\n", run.ID)
			fmt.Println("Waiting for completion...")

			for {
				time.Sleep(1 * time.Second)
				updated, err := runService.GetRun(run.ID)
				if err != nil {
					return err
				}

				if updated.Status == domain.RunStatusSuccess || updated.Status == domain.RunStatusFailed {
					if updated.Status == domain.RunStatusSuccess {
						fmt.Printf("Run completed successfully\n")
						if updated.Stats != nil {
							var stats domain.RunStats
							json.Unmarshal(updated.Stats, &stats)
							fmt.Printf("Total rows: %d\n", stats.TotalRows)
							fmt.Printf("Duration: %.2fs\n", stats.DurationSeconds)
						}
					} else {
						fmt.Printf("Run failed: %s\n", updated.Error)
						return fmt.Errorf("run failed")
					}
					break
				}
			}

			return nil
		},
	}

	startCmd.Flags().StringVar(&scenarioID, "scenario", "", "Scenario ID")
	startCmd.Flags().StringVar(&scenarioPath, "scenario-path", "", "Scenario file path")
	startCmd.Flags().StringVar(&targetID, "target-id", "", "Target ID")
	startCmd.Flags().StringVar(&targetDSN, "target", "", "Target DSN")
	startCmd.Flags().StringVar(&targetKind, "target-kind", "", "Target kind (required with --target)")
	startCmd.Flags().Int64VarP(&seed, "seed", "s", 0, "Seed for RNG")
	startCmd.Flags().StringSliceVar(&rowsOverride, "rows-override", nil, "Row overrides (entity=rows)")
	startCmd.Flags().StringVar(&mode, "mode", "create_if_missing", "Table mode")
	startCmd.Flags().Lookup("seed").NoOptDefVal = "0"
	startCmd.PreRun = func(cmd *cobra.Command, args []string) {
		hasSeed = cmd.Flags().Changed("seed")
	}

	var limit int
	var status string
	var format string

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			runRepo := runs.NewSQLiteRepository(runsDBPath)
			if err := runRepo.Init(); err != nil {
				return err
			}

			list, err := runRepo.List(limit, status)
			if err != nil {
				return err
			}

			if format == "json" {
				data, _ := json.MarshalIndent(list, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSCENARIO\tTARGET\tSTATUS\tSTARTED")
			for _, r := range list {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					r.ID[:8], r.ScenarioName, r.TargetName, r.Status, r.StartedAt.Format("2006-01-02 15:04"))
			}
			w.Flush()
			return nil
		},
	}
	listCmd.Flags().IntVar(&limit, "limit", 20, "Limit results")
	listCmd.Flags().StringVar(&status, "status", "", "Filter by status")
	listCmd.Flags().StringVar(&format, "format", "table", "Output format (table|json)")

	showCmd := &cobra.Command{
		Use:   "show <run_id>",
		Short: "Show run details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runRepo := runs.NewSQLiteRepository(runsDBPath)
			if err := runRepo.Init(); err != nil {
				return err
			}

			run, err := runRepo.Get(args[0])
			if err != nil {
				return err
			}

			data, _ := yaml.Marshal(run)
			fmt.Println(string(data))
			return nil
		},
	}

	cmd.AddCommand(startCmd, listCmd, showCmd)
	return cmd
}
