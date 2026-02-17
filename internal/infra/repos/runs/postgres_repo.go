package runs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/mmrzaf/sdgen/internal/domain"
)

type PostgresRepository struct {
	dsn string
	db  *sql.DB
}

func NewPostgresRepository(dsn string) *PostgresRepository {
	return &PostgresRepository{dsn: strings.TrimSpace(dsn)}
}

func (r *PostgresRepository) Init() error {
	if r.dsn == "" {
		return fmt.Errorf("sdgen db dsn is required")
	}
	db, err := sql.Open("postgres", r.dsn)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return err
	}
	r.db = db
	return r.applyMigrations()
}

func (r *PostgresRepository) DB() *sql.DB { return r.db }

func (r *PostgresRepository) applyMigrations() error {
	if _, err := r.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return err
	}
	var cur int
	if err := r.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&cur); err != nil {
		return err
	}

	type mig struct {
		v  int
		up func(*sql.DB) error
	}
	migs := []mig{
		{1, migrateV1RunsPG},
		{2, migrateV2TargetsPG},
		{3, migrateV3TargetChecksPG},
		{4, migrateV4TargetDatabasePG},
		{5, migrateV5RunProgressPG},
		{6, migrateV6RunLogsPG},
	}

	for _, m := range migs {
		if cur >= m.v {
			continue
		}
		if err := m.up(r.db); err != nil {
			return fmt.Errorf("migration %d failed: %w", m.v, err)
		}
		if _, err := r.db.Exec(`INSERT INTO schema_migrations(version) VALUES ($1)`, m.v); err != nil {
			return err
		}
		cur = m.v
	}
	return nil
}

func migrateV1RunsPG(db *sql.DB) error {
	if _, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS runs (
		id TEXT PRIMARY KEY,
		scenario_id TEXT,
		scenario_name TEXT,
		scenario_version TEXT,
		target_id TEXT,
		target_name TEXT,
		target_kind TEXT,
		seed BIGINT,
		mode TEXT,
		scale DOUBLE PRECISION,
		resolved_counts TEXT,
		execution_order TEXT,
		warnings TEXT,
		config_hash TEXT,
		status TEXT,
		started_at TIMESTAMPTZ,
		completed_at TIMESTAMPTZ,
		stats TEXT,
		error TEXT
	)`); err != nil {
		return err
	}
	return nil
}

func migrateV2TargetsPG(db *sql.DB) error {
	if _, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS targets (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		kind TEXT NOT NULL,
		dsn TEXT NOT NULL,
		schema TEXT,
		options_json TEXT,
		created_at TIMESTAMPTZ NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL
	)`); err != nil {
		return err
	}
	_, _ = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_targets_name ON targets(name)`)
	return nil
}

func migrateV3TargetChecksPG(db *sql.DB) error {
	if _, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS target_checks (
		id TEXT PRIMARY KEY,
		target_id TEXT NOT NULL,
		checked_at TIMESTAMPTZ NOT NULL,
		ok BOOLEAN NOT NULL,
		latency_ms BIGINT NOT NULL,
		server_version TEXT,
		capabilities_json TEXT,
		error TEXT
	)`); err != nil {
		return err
	}
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_target_checks_target_time ON target_checks(target_id, checked_at DESC)`)
	return nil
}

func migrateV4TargetDatabasePG(db *sql.DB) error {
	_, err := db.Exec(`ALTER TABLE targets ADD COLUMN IF NOT EXISTS database TEXT`)
	return err
}

func migrateV5RunProgressPG(db *sql.DB) error {
	ddls := []string{
		`ALTER TABLE runs ADD COLUMN IF NOT EXISTS progress_rows_generated BIGINT`,
		`ALTER TABLE runs ADD COLUMN IF NOT EXISTS progress_rows_total BIGINT`,
		`ALTER TABLE runs ADD COLUMN IF NOT EXISTS progress_entities_done INTEGER`,
		`ALTER TABLE runs ADD COLUMN IF NOT EXISTS progress_entities_total INTEGER`,
		`ALTER TABLE runs ADD COLUMN IF NOT EXISTS progress_current_entity TEXT`,
	}
	for _, ddl := range ddls {
		if _, err := db.Exec(ddl); err != nil {
			return err
		}
	}
	return nil
}

func migrateV6RunLogsPG(db *sql.DB) error {
	if _, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS run_logs (
		id BIGSERIAL PRIMARY KEY,
		run_id TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL,
		level TEXT NOT NULL,
		message TEXT NOT NULL
	)`); err != nil {
		return err
	}
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_run_logs_run_time ON run_logs(run_id, id DESC)`)
	return nil
}

func (r *PostgresRepository) Create(run *domain.Run) error {
	statsJSON, err := json.Marshal(run.Stats)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`
	INSERT INTO runs (
		id, scenario_id, scenario_name, scenario_version,
		target_id, target_name, target_kind,
		seed, mode, scale, resolved_counts, execution_order, warnings,
		config_hash, status, started_at, stats,
		progress_rows_generated, progress_rows_total, progress_entities_done, progress_entities_total, progress_current_entity
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`,
		run.ID, run.ScenarioID, run.ScenarioName, run.ScenarioVersion,
		run.TargetID, run.TargetName, run.TargetKind,
		run.Seed, run.Mode, run.Scale, string(run.ResolvedCounts), string(run.ExecutionOrder), string(run.Warnings),
		run.ConfigHash, run.Status, run.StartedAt, string(statsJSON),
		run.ProgressRowsGenerated, run.ProgressRowsTotal, run.ProgressEntitiesDone, run.ProgressEntitiesTotal, run.ProgressCurrentEntity,
	)
	return err
}

func (r *PostgresRepository) Update(run *domain.Run) error {
	statsJSON, err := json.Marshal(run.Stats)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`
	UPDATE runs SET
		status = $1, completed_at = $2, stats = $3, error = $4
	WHERE id = $5`,
		run.Status, run.CompletedAt, string(statsJSON), run.Error, run.ID,
	)
	return err
}

func (r *PostgresRepository) Get(id string) (*domain.Run, error) {
	var run domain.Run
	var completedAt sql.NullTime
	var statsStr sql.NullString
	var rc sql.NullString
	var eo sql.NullString
	var w sql.NullString
	var errStr sql.NullString
	var prgRows sql.NullInt64
	var prgTotal sql.NullInt64
	var prgEntDone sql.NullInt64
	var prgEntTotal sql.NullInt64
	var prgCurrent sql.NullString

	err := r.db.QueryRow(`
	SELECT id, scenario_id, scenario_name, scenario_version,
		target_id, target_name, target_kind,
		seed, mode, scale, resolved_counts, execution_order, warnings,
		config_hash, status, started_at, completed_at, stats, error,
		progress_rows_generated, progress_rows_total, progress_entities_done, progress_entities_total, progress_current_entity
	FROM runs WHERE id = $1`, id).Scan(
		&run.ID, &run.ScenarioID, &run.ScenarioName, &run.ScenarioVersion,
		&run.TargetID, &run.TargetName, &run.TargetKind,
		&run.Seed, &run.Mode, &run.Scale, &rc, &eo, &w,
		&run.ConfigHash, &run.Status, &run.StartedAt, &completedAt, &statsStr, &errStr,
		&prgRows, &prgTotal, &prgEntDone, &prgEntTotal, &prgCurrent,
	)
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		run.CompletedAt = &completedAt.Time
	}
	if statsStr.Valid {
		run.Stats = json.RawMessage(statsStr.String)
	}
	if rc.Valid {
		run.ResolvedCounts = json.RawMessage(rc.String)
	}
	if eo.Valid {
		run.ExecutionOrder = json.RawMessage(eo.String)
	}
	if w.Valid {
		run.Warnings = json.RawMessage(w.String)
	}
	if errStr.Valid {
		run.Error = errStr.String
	}
	if prgRows.Valid {
		run.ProgressRowsGenerated = prgRows.Int64
	}
	if prgTotal.Valid {
		run.ProgressRowsTotal = prgTotal.Int64
	}
	if prgEntDone.Valid {
		run.ProgressEntitiesDone = int(prgEntDone.Int64)
	}
	if prgEntTotal.Valid {
		run.ProgressEntitiesTotal = int(prgEntTotal.Int64)
	}
	if prgCurrent.Valid {
		run.ProgressCurrentEntity = prgCurrent.String
	}
	return &run, nil
}

func (r *PostgresRepository) List(limit int, status string) ([]*domain.Run, error) {
	if limit <= 0 {
		limit = 50
	}

	var (
		rows *sql.Rows
		err  error
	)
	if status != "" {
		rows, err = r.db.Query(`
		SELECT id, scenario_id, scenario_name, scenario_version,
			target_id, target_name, target_kind,
			seed, mode, scale, resolved_counts, execution_order, warnings,
			config_hash, status, started_at, completed_at, stats, error,
			progress_rows_generated, progress_rows_total, progress_entities_done, progress_entities_total, progress_current_entity
		FROM runs
		WHERE status = $1
		ORDER BY started_at DESC
		LIMIT $2`, status, limit)
	} else {
		rows, err = r.db.Query(`
		SELECT id, scenario_id, scenario_name, scenario_version,
			target_id, target_name, target_kind,
			seed, mode, scale, resolved_counts, execution_order, warnings,
			config_hash, status, started_at, completed_at, stats, error,
			progress_rows_generated, progress_rows_total, progress_entities_done, progress_entities_total, progress_current_entity
		FROM runs
		ORDER BY started_at DESC
		LIMIT $1`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Run
	for rows.Next() {
		var run domain.Run
		var completedAt sql.NullTime
		var statsStr sql.NullString
		var rc sql.NullString
		var eo sql.NullString
		var w sql.NullString
		var errStr sql.NullString
		var prgRows sql.NullInt64
		var prgTotal sql.NullInt64
		var prgEntDone sql.NullInt64
		var prgEntTotal sql.NullInt64
		var prgCurrent sql.NullString

		if err := rows.Scan(
			&run.ID, &run.ScenarioID, &run.ScenarioName, &run.ScenarioVersion,
			&run.TargetID, &run.TargetName, &run.TargetKind,
			&run.Seed, &run.Mode, &run.Scale, &rc, &eo, &w,
			&run.ConfigHash, &run.Status, &run.StartedAt, &completedAt, &statsStr, &errStr,
			&prgRows, &prgTotal, &prgEntDone, &prgEntTotal, &prgCurrent,
		); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			run.CompletedAt = &completedAt.Time
		}
		if statsStr.Valid {
			run.Stats = json.RawMessage(statsStr.String)
		}
		if rc.Valid {
			run.ResolvedCounts = json.RawMessage(rc.String)
		}
		if eo.Valid {
			run.ExecutionOrder = json.RawMessage(eo.String)
		}
		if w.Valid {
			run.Warnings = json.RawMessage(w.String)
		}
		if errStr.Valid {
			run.Error = errStr.String
		}
		if prgRows.Valid {
			run.ProgressRowsGenerated = prgRows.Int64
		}
		if prgTotal.Valid {
			run.ProgressRowsTotal = prgTotal.Int64
		}
		if prgEntDone.Valid {
			run.ProgressEntitiesDone = int(prgEntDone.Int64)
		}
		if prgEntTotal.Valid {
			run.ProgressEntitiesTotal = int(prgEntTotal.Int64)
		}
		if prgCurrent.Valid {
			run.ProgressCurrentEntity = prgCurrent.String
		}
		out = append(out, &run)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) UpdateStatus(id string, status domain.RunStatus, errMsg string, stats *domain.RunStats) error {
	now := sql.NullTime{}
	if status == domain.RunStatusSuccess || status == domain.RunStatusFailed {
		now = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	}

	var statsJSON interface{}
	if stats != nil {
		b, err := json.Marshal(stats)
		if err != nil {
			return err
		}
		statsJSON = string(b)
	}

	_, err := r.db.Exec(`
		UPDATE runs
		SET status = $1, completed_at = $2, stats = COALESCE($3, stats), error = $4
		WHERE id = $5`,
		status, now, statsJSON, errMsg, id,
	)
	return err
}

func (r *PostgresRepository) UpdateProgress(id string, rowsGenerated, rowsTotal int64, entitiesDone, entitiesTotal int, currentEntity string) error {
	_, err := r.db.Exec(`
		UPDATE runs
		SET progress_rows_generated = $1, progress_rows_total = $2, progress_entities_done = $3, progress_entities_total = $4, progress_current_entity = $5
		WHERE id = $6`,
		rowsGenerated, rowsTotal, entitiesDone, entitiesTotal, currentEntity, id,
	)
	return err
}

func (r *PostgresRepository) AppendRunLog(runID, level, message string) error {
	_, err := r.db.Exec(`
		INSERT INTO run_logs (run_id, created_at, level, message)
		VALUES ($1, $2, $3, $4)`,
		runID, time.Now().UTC(), level, message,
	)
	return err
}

func (r *PostgresRepository) ListRunLogs(runID string, limit int) ([]*domain.RunLog, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.db.Query(`
		SELECT id, run_id, created_at, level, message
		FROM run_logs
		WHERE run_id = $1
		ORDER BY id DESC
		LIMIT $2`, runID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*domain.RunLog, 0, limit)
	for rows.Next() {
		var rl domain.RunLog
		if err := rows.Scan(&rl.ID, &rl.RunID, &rl.CreatedAt, &rl.Level, &rl.Message); err != nil {
			return nil, err
		}
		out = append(out, &rl)
	}
	return out, rows.Err()
}
