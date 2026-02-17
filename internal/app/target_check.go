package app

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mmrzaf/sdgen/internal/domain"
	"github.com/mmrzaf/sdgen/internal/exec"
	esTarget "github.com/mmrzaf/sdgen/internal/infra/targets/elasticsearch"
	pgTarget "github.com/mmrzaf/sdgen/internal/infra/targets/postgres"
	sqliteTarget "github.com/mmrzaf/sdgen/internal/infra/targets/sqlite"
	"github.com/mmrzaf/sdgen/internal/validation"
)

func CheckTarget(t *domain.TargetConfig) (*domain.TargetCheck, error) {
	check := &domain.TargetCheck{
		ID:        uuid.NewString(),
		TargetID:  t.ID,
		CheckedAt: time.Now().UTC(),
	}

	// schema identifier validated here too
	val := validation.NewValidator(nil)
	if err := val.ValidateTarget(t); err != nil {
		check.OK = false
		check.Error = err.Error()
		return check, err
	}

	start := time.Now()
	effective := resolveTargetForRun(t, "")
	tgt, verFn, err := buildCheckTarget(effective)
	if err != nil {
		check.OK = false
		check.Error = "unsupported target kind"
		return check, err
	}
	if err := tgt.Connect(); err != nil {
		check.OK = false
		check.Error = err.Error()
		check.LatencyMS = time.Since(start).Milliseconds()
		return check, err
	}
	defer tgt.Close()

	check.OK = true
	check.LatencyMS = time.Since(start).Milliseconds()
	if verFn != nil {
		if ver, verErr := verFn(); verErr == nil {
			check.ServerVer = ver
		}
	}
	check.Capabilities = probeCapabilities(tgt)
	return check, nil
}

func buildCheckTarget(t *domain.TargetConfig) (exec.Target, func() (string, error), error) {
	switch t.Kind {
	case "postgres":
		schema := t.Schema
		if schema == "" {
			schema = "public"
		}
		return pgTarget.NewPostgresTarget(t.DSN, schema), func() (string, error) {
			return queryServerVersion("postgres", t.DSN, "SHOW server_version")
		}, nil
	case "sqlite":
		return sqliteTarget.NewSQLiteTarget(t.DSN), func() (string, error) {
			return queryServerVersion("sqlite3", t.DSN, "SELECT sqlite_version()")
		}, nil
	case "elasticsearch":
		return esTarget.NewElasticsearchTarget(t.DSN), func() (string, error) {
			return esTarget.GetServerVersion(t.DSN)
		}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported target kind: %s", t.Kind)
	}
}

func queryServerVersion(driver, dsn, query string) (string, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return "", err
	}
	defer db.Close()
	var version string
	if err := db.QueryRow(query).Scan(&version); err != nil {
		return "", err
	}
	return version, nil
}

func probeCapabilities(tgt exec.Target) domain.TargetCapabilities {
	entity := &domain.Entity{
		Name:        "sdgen_check",
		TargetTable: fmt.Sprintf("sdgen_check_%d", time.Now().UnixNano()),
		Rows:        1,
		Columns: []domain.Column{
			{Name: "id", Type: domain.ColumnTypeInt},
		},
	}

	var caps domain.TargetCapabilities
	if err := tgt.CreateTableIfNotExists(entity); err != nil {
		return caps
	}
	caps.CanCreate = true

	if err := tgt.InsertBatch(entity.TargetTable, []string{"id"}, [][]interface{}{{int64(1)}}); err != nil {
		return caps
	}
	caps.CanInsert = true

	if err := tgt.TruncateTable(entity.TargetTable); err != nil {
		return caps
	}
	caps.CanTruncate = true
	return caps
}
