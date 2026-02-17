package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
	runsDBPath   string
	logLevel     string
	batchSize    int
)

func main() {
	cfg := config.Load()

	root := &cobra.Command{Use: "sdgen"}
	root.PersistentFlags().StringVar(&scenariosDir, "scenarios-dir", cfg.ScenariosDir, "Scenarios directory")
	root.PersistentFlags().StringVar(&runsDBPath, "runs-db", cfg.RunsDBPath, "Runs database path")
	root.PersistentFlags().StringVar(&logLevel, "log-level", cfg.LogLevel, "Log level")
	root.PersistentFlags().IntVar(&batchSize, "batch-size", cfg.BatchSize, "Default insert batch size")

	root.AddCommand(scenarioCmd())
	root.AddCommand(targetCmd())
	root.AddCommand(runCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func scenarioCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "scenario", Short: "Manage scenarios (read-only)"}
	var format string

	list := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := scenarios.NewFileRepository(scenariosDir)
			list, err := repo.List()
			if err != nil {
				return err
			}
			if format == "json" {
				b, _ := json.MarshalIndent(list, "", "  ")
				fmt.Println(string(b))
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tVERSION")
			for _, s := range list {
				fmt.Fprintf(w, "%s\t%s\t%s\n", s.ID, s.Name, s.Version)
			}
			return w.Flush()
		},
	}
	list.Flags().StringVar(&format, "format", "table", "Output format (table|json)")

	show := &cobra.Command{
		Use:  "show <id>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := scenarios.NewFileRepository(scenariosDir)
			sc, err := repo.Get(args[0])
			if err != nil {
				return err
			}
			b, _ := yaml.Marshal(sc)
			fmt.Println(string(b))
			return nil
		},
	}

	validate := &cobra.Command{
		Use:  "validate <id|path>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := scenarios.NewFileRepository(scenariosDir)
			arg := args[0]
			var (
				sc  *domain.Scenario
				err error
			)
			if st, statErr := os.Stat(arg); statErr == nil && !st.IsDir() {
				sc, err = repo.GetByPath(arg)
			} else {
				sc, err = repo.Get(arg)
			}
			if err != nil {
				return err
			}
			val := validation.NewValidator(registry.DefaultGeneratorRegistry())
			return val.ValidateScenario(sc)
		},
	}

	cmd.AddCommand(list, show, validate)
	return cmd
}

func targetCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "target", Short: "Manage targets (DB-backed)"}
	var format string

	openRepos := func() (*runs.SQLiteRepository, *targets.SQLiteRepository, error) {
		runRepo := runs.NewSQLiteRepository(runsDBPath)
		if err := runRepo.Init(); err != nil {
			return nil, nil, err
		}
		return runRepo, targets.NewSQLiteRepository(runRepo.DB()), nil
	}

	list := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, repo, err := openRepos()
			if err != nil {
				return err
			}
			list, err := repo.List()
			if err != nil {
				return err
			}
			if format == "json" {
				b, _ := json.MarshalIndent(targets.RedactTargets(list), "", "  ")
				fmt.Println(string(b))
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tKIND\tDATABASE\tSCHEMA\tDSN")
			for _, t := range list {
				rt := targets.RedactTarget(t)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", rt.ID, rt.Name, rt.Kind, rt.Database, rt.Schema, rt.DSN)
			}
			return w.Flush()
		},
	}
	list.Flags().StringVar(&format, "format", "table", "Output format (table|json)")

	show := &cobra.Command{
		Use:  "show <id>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, repo, err := openRepos()
			if err != nil {
				return err
			}
			t, err := repo.Get(args[0])
			if err != nil {
				return err
			}
			b, _ := yaml.Marshal(targets.RedactTarget(t))
			fmt.Println(string(b))
			return nil
		},
	}

	var (
		id       string
		name     string
		kind     string
		dsn      string
		database string
		schema   string
		host     string
		port     int
		user     string
		password string
		sslmode  string
		scheme   string
	)

	add := &cobra.Command{
		Use: "add",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, repo, err := openRepos()
			if err != nil {
				return err
			}
			val := validation.NewValidator(registry.DefaultGeneratorRegistry())
			resolvedDSN := dsn
			if strings.TrimSpace(resolvedDSN) == "" {
				var err error
				resolvedDSN, err = buildDSNFromParts(kind, host, port, user, password, database, sslmode, scheme)
				if err != nil {
					return err
				}
			}
			t := &domain.TargetConfig{ID: id, Name: name, Kind: kind, DSN: resolvedDSN, Database: database, Schema: schema}
			if err := val.ValidateTarget(t); err != nil {
				return err
			}
			if err := repo.Create(t); err != nil {
				return err
			}
			fmt.Println(t.ID)
			return nil
		},
	}
	add.Flags().StringVar(&id, "id", "", "Target id (optional)")
	add.Flags().StringVar(&name, "name", "", "Target name")
	add.Flags().StringVar(&kind, "kind", "", "Target kind (postgres|sqlite|elasticsearch)")
	add.Flags().StringVar(&dsn, "dsn", "", "Target DSN")
	add.Flags().StringVar(&database, "database", "", "Default database name for postgres targets")
	add.Flags().StringVar(&schema, "schema", "", "Schema (postgres)")
	add.Flags().StringVar(&host, "host", "", "Host used to build DSN when --dsn is omitted")
	add.Flags().IntVar(&port, "port", 0, "Port used to build DSN when --dsn is omitted")
	add.Flags().StringVar(&user, "user", "", "Username used to build DSN when --dsn is omitted")
	add.Flags().StringVar(&password, "password", "", "Password used to build DSN when --dsn is omitted")
	add.Flags().StringVar(&sslmode, "sslmode", "disable", "Postgres sslmode used to build DSN")
	add.Flags().StringVar(&scheme, "scheme", "", "Scheme used to build DSN (http/https for elasticsearch)")
	_ = add.MarkFlagRequired("name")
	_ = add.MarkFlagRequired("kind")

	update := &cobra.Command{
		Use:  "update <id>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, repo, err := openRepos()
			if err != nil {
				return err
			}
			val := validation.NewValidator(registry.DefaultGeneratorRegistry())
			resolvedDSN := dsn
			if strings.TrimSpace(resolvedDSN) == "" {
				var err error
				resolvedDSN, err = buildDSNFromParts(kind, host, port, user, password, database, sslmode, scheme)
				if err != nil {
					return err
				}
			}
			t := &domain.TargetConfig{ID: args[0], Name: name, Kind: kind, DSN: resolvedDSN, Database: database, Schema: schema}
			if err := val.ValidateTarget(t); err != nil {
				return err
			}
			return repo.Update(t)
		},
	}
	update.Flags().StringVar(&name, "name", "", "Target name")
	update.Flags().StringVar(&kind, "kind", "", "Target kind (postgres|sqlite|elasticsearch)")
	update.Flags().StringVar(&dsn, "dsn", "", "Target DSN")
	update.Flags().StringVar(&database, "database", "", "Default database name for postgres targets")
	update.Flags().StringVar(&schema, "schema", "", "Schema (postgres)")
	update.Flags().StringVar(&host, "host", "", "Host used to build DSN when --dsn is omitted")
	update.Flags().IntVar(&port, "port", 0, "Port used to build DSN when --dsn is omitted")
	update.Flags().StringVar(&user, "user", "", "Username used to build DSN when --dsn is omitted")
	update.Flags().StringVar(&password, "password", "", "Password used to build DSN when --dsn is omitted")
	update.Flags().StringVar(&sslmode, "sslmode", "disable", "Postgres sslmode used to build DSN")
	update.Flags().StringVar(&scheme, "scheme", "", "Scheme used to build DSN (http/https for elasticsearch)")
	_ = update.MarkFlagRequired("name")
	_ = update.MarkFlagRequired("kind")

	rm := &cobra.Command{
		Use:  "rm <id>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, repo, err := openRepos()
			if err != nil {
				return err
			}
			return repo.Delete(args[0])
		},
	}

	test := &cobra.Command{
		Use:  "test <id>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.NewLogger(logLevel)
			runRepo := runs.NewSQLiteRepository(runsDBPath)
			if err := runRepo.Init(); err != nil {
				return err
			}
			targetRepo := targets.NewSQLiteRepository(runRepo.DB())
			scRepo := scenarios.NewFileRepository(scenariosDir)
			svc := app.NewRunService(scRepo, targetRepo, runRepo, registry.DefaultGeneratorRegistry(), logger, batchSize)

			out, err := svc.TestTarget(args[0])
			if out != nil {
				b, _ := json.MarshalIndent(out, "", "  ")
				fmt.Println(string(b))
			}
			return err
		},
	}

	cmd.AddCommand(list, show, add, update, rm, test)
	return cmd
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "run"}

	var (
		scenario string

		targetID     string
		targetDSN    string
		targetKind   string
		targetDB     string
		targetSchema string

		mode    string
		scale   float64
		ecList  []string
		esList  []string
		include []string
		exclude []string

		seed     int64
		hasSeed  bool
		hasScale bool

		doPlan bool
		wait   bool
	)

	start := &cobra.Command{
		Use: "start",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.NewLogger(logLevel)

			scRepo := scenarios.NewFileRepository(scenariosDir)

			runRepo := runs.NewSQLiteRepository(runsDBPath)
			if err := runRepo.Init(); err != nil {
				return err
			}
			targetRepo := targets.NewSQLiteRepository(runRepo.DB())
			svc := app.NewRunService(scRepo, targetRepo, runRepo, registry.DefaultGeneratorRegistry(), logger, batchSize)

			req := &domain.RunRequest{Mode: mode}

			if scenario == "" {
				return fmt.Errorf("--scenario is required")
			}
			if st, statErr := os.Stat(scenario); statErr == nil && !st.IsDir() {
				sc, err := scRepo.GetByPath(scenario)
				if err != nil {
					return err
				}
				req.Scenario = sc
			} else {
				req.ScenarioID = scenario
			}

			if targetDSN != "" {
				if targetKind == "" {
					return fmt.Errorf("--target-kind required with --target")
				}
				req.Target = &domain.TargetConfig{
					Name:   "inline-target",
					Kind:   targetKind,
					DSN:    targetDSN,
					Schema: targetSchema,
				}
			} else if targetID != "" {
				req.TargetID = targetID
			} else {
				return fmt.Errorf("either --target-id or --target required")
			}

			if hasSeed {
				req.Seed = &seed
			}
			if targetDB != "" {
				req.TargetDatabase = targetDB
			}
			if hasScale {
				req.Scale = &scale
			}

			if len(ecList) > 0 {
				req.EntityCounts = map[string]int64{}
				for _, kv := range ecList {
					parts := strings.SplitN(kv, "=", 2)
					if len(parts) != 2 {
						return fmt.Errorf("invalid --entity-count: %s", kv)
					}
					n, err := strconv.ParseInt(parts[1], 10, 64)
					if err != nil {
						return fmt.Errorf("invalid --entity-count value: %s", parts[1])
					}
					req.EntityCounts[parts[0]] = n
				}
			}
			if len(esList) > 0 {
				req.EntityScales = map[string]float64{}
				for _, kv := range esList {
					parts := strings.SplitN(kv, "=", 2)
					if len(parts) != 2 {
						return fmt.Errorf("invalid --entity-scale: %s", kv)
					}
					f, err := strconv.ParseFloat(parts[1], 64)
					if err != nil {
						return fmt.Errorf("invalid --entity-scale value: %s", parts[1])
					}
					req.EntityScales[parts[0]] = f
				}
			}
			if len(include) > 0 {
				req.IncludeEntities = append([]string(nil), include...)
			}
			if len(exclude) > 0 {
				req.ExcludeEntities = append([]string(nil), exclude...)
			}

			if doPlan {
				plan, err := svc.PlanRun(req)
				if plan != nil {
					b, _ := json.MarshalIndent(plan, "", "  ")
					fmt.Println(string(b))
				}
				return err
			}

			run, err := svc.StartRun(req)
			if err != nil {
				return err
			}
			if !wait {
				b, _ := json.MarshalIndent(run, "", "  ")
				fmt.Println(string(b))
				return nil
			}
			for {
				cur, err := svc.GetRun(run.ID)
				if err != nil {
					return err
				}
				if cur.Status == domain.RunStatusSuccess {
					b, _ := json.MarshalIndent(cur, "", "  ")
					fmt.Println(string(b))
					return nil
				}
				if cur.Status == domain.RunStatusFailed {
					b, _ := json.MarshalIndent(cur, "", "  ")
					fmt.Println(string(b))
					if cur.Error != "" {
						return errors.New(cur.Error)
					}
					return fmt.Errorf("run %s failed", cur.ID)
				}
				time.Sleep(200 * time.Millisecond)
			}
		},
	}

	start.Flags().StringVar(&scenario, "scenario", "", "Scenario ID or file path (inside scenarios dir)")

	start.Flags().StringVar(&targetID, "target-id", "", "Target ID")
	start.Flags().StringVar(&targetDSN, "target", "", "Inline target DSN (not stored)")
	start.Flags().StringVar(&targetKind, "target-kind", "", "Inline target kind (postgres|sqlite|elasticsearch)")
	start.Flags().StringVar(&targetDB, "target-db", "", "Target database override for this run (postgres)")
	start.Flags().StringVar(&targetSchema, "target-schema", "", "Inline target schema (postgres)")

	start.Flags().StringVar(&mode, "mode", "", "Mode (create|truncate|append)")
	start.Flags().Float64Var(&scale, "scale", 1.0, "Scale factor")
	start.Flags().StringSliceVar(&ecList, "entity-count", nil, "Override entity count entity=N (repeatable)")
	start.Flags().StringSliceVar(&esList, "entity-scale", nil, "Per-entity scale entity=F (repeatable)")
	start.Flags().StringSliceVar(&include, "include-entity", nil, "Include only these entities (repeatable)")
	start.Flags().StringSliceVar(&exclude, "exclude-entity", nil, "Exclude these entities (repeatable)")
	start.Flags().BoolVar(&doPlan, "plan", false, "Plan only (do not execute)")
	start.Flags().BoolVar(&wait, "wait", true, "Wait for terminal run status before returning")

	start.Flags().Int64Var(&seed, "seed", 0, "Seed for RNG")
	start.Flags().Lookup("seed").NoOptDefVal = "0"
	start.PreRun = func(cmd *cobra.Command, args []string) {
		hasSeed = cmd.Flags().Changed("seed")
		hasScale = cmd.Flags().Changed("scale")
	}
	_ = start.MarkFlagRequired("mode")

	var limit int
	list := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.NewLogger(logLevel)
			scRepo := scenarios.NewFileRepository(scenariosDir)
			runRepo := runs.NewSQLiteRepository(runsDBPath)
			if err := runRepo.Init(); err != nil {
				return err
			}
			targetRepo := targets.NewSQLiteRepository(runRepo.DB())
			svc := app.NewRunService(scRepo, targetRepo, runRepo, registry.DefaultGeneratorRegistry(), logger, batchSize)
			runsList, err := svc.ListRuns(limit)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSTATUS\tSCENARIO\tTARGET\tSTARTED_AT")
			for _, r := range runsList {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Status, r.ScenarioName, r.TargetName, r.StartedAt.Format("2006-01-02T15:04:05Z07:00"))
			}
			return w.Flush()
		},
	}
	list.Flags().IntVar(&limit, "limit", 50, "Maximum runs to list")

	show := &cobra.Command{
		Use:  "show <run_id>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.NewLogger(logLevel)
			scRepo := scenarios.NewFileRepository(scenariosDir)
			runRepo := runs.NewSQLiteRepository(runsDBPath)
			if err := runRepo.Init(); err != nil {
				return err
			}
			targetRepo := targets.NewSQLiteRepository(runRepo.DB())
			svc := app.NewRunService(scRepo, targetRepo, runRepo, registry.DefaultGeneratorRegistry(), logger, batchSize)
			run, err := svc.GetRun(args[0])
			if err != nil {
				return err
			}
			b, _ := json.MarshalIndent(run, "", "  ")
			fmt.Println(string(b))
			return nil
		},
	}

	cmd.AddCommand(start, list, show)
	return cmd
}

func buildDSNFromParts(kind, host string, port int, user, password, database, sslmode, scheme string) (string, error) {
	kind = strings.TrimSpace(kind)
	host = strings.TrimSpace(host)
	user = strings.TrimSpace(user)
	password = strings.TrimSpace(password)
	database = strings.TrimSpace(database)
	sslmode = strings.TrimSpace(sslmode)
	scheme = strings.TrimSpace(scheme)

	switch kind {
	case "postgres":
		if host == "" {
			host = "localhost"
		}
		if port == 0 {
			port = 5432
		}
		if database == "" {
			return "", fmt.Errorf("postgres DSN builder requires --database")
		}
		if user == "" {
			return "", fmt.Errorf("postgres DSN builder requires --user")
		}
		if sslmode == "" {
			sslmode = "disable"
		}
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			url.QueryEscape(user), url.QueryEscape(password), host, port, url.PathEscape(database), url.QueryEscape(sslmode)), nil
	case "elasticsearch":
		if host == "" {
			host = "localhost"
		}
		if port == 0 {
			port = 9200
		}
		if scheme == "" {
			scheme = "http"
		}
		auth := ""
		if user != "" {
			auth = url.QueryEscape(user)
			if password != "" {
				auth += ":" + url.QueryEscape(password)
			}
			auth += "@"
		}
		return fmt.Sprintf("%s://%s%s:%d", scheme, auth, host, port), nil
	case "sqlite":
		if database == "" {
			return "", fmt.Errorf("sqlite DSN builder requires --database with sqlite file path")
		}
		return database, nil
	default:
		return "", fmt.Errorf("unsupported target kind for DSN builder: %s", kind)
	}
}
