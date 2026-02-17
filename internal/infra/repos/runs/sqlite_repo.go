package runs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mmrzaf/sdgen/internal/domain"
)

type SQLiteRepository struct {
	dbPath string
	db     *sql.DB
}

func NewSQLiteRepository(path string) *SQLiteRepository {
	return &SQLiteRepository{dbPath: path}
}

func (r *SQLiteRepository) Init() error {
	if err := ensureDBDirectory(r.dbPath); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", r.dbPath)
	if err != nil {
		return err
	}
	r.db = db
	return r.applyMigrations()
}

func ensureDBDirectory(dbPath string) error {
	trimmed := strings.TrimSpace(dbPath)
	if trimmed == "" || trimmed == ":memory:" || strings.HasPrefix(trimmed, "file::memory:") {
		return nil
	}
	if strings.HasPrefix(trimmed, "file:") {
		trimmed = strings.TrimPrefix(trimmed, "file:")
		if i := strings.Index(trimmed, "?"); i >= 0 {
			trimmed = trimmed[:i]
		}
	}
	dir := filepath.Dir(trimmed)
	if dir == "." || dir == "/" || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func (r *SQLiteRepository) DB() *sql.DB { return r.db }

func (r *SQLiteRepository) applyMigrations() error {
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
		{1, migrateV1Runs},
		{2, migrateV2Targets},
		{3, migrateV3TargetChecks},
		{4, migrateV4TargetDatabase},
		{5, migrateV5RunProgress},
		{6, migrateV6RunLogs},
	}

	for _, m := range migs {
		if cur >= m.v {
			continue
		}
		if err := m.up(r.db); err != nil {
			return fmt.Errorf("migration %d failed: %w", m.v, err)
		}
		if _, err := r.db.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, m.v); err != nil {
			return err
		}
		cur = m.v
	}
	return nil
}

func hasColumn(db *sql.DB, table, col string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == col {
			return true, nil
		}
	}
	return false, rows.Err()
}

func migrateV1Runs(db *sql.DB) error {
	// Create new schema (also supports fresh DB)
	if _, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS runs (
		id TEXT PRIMARY KEY,
		scenario_id TEXT,
		scenario_name TEXT,
		scenario_version TEXT,
		target_id TEXT,
		target_name TEXT,
		target_kind TEXT,
		seed INTEGER,
		mode TEXT,
		scale REAL,
		resolved_counts TEXT,
		execution_order TEXT,
		warnings TEXT,
		config_hash TEXT,
		status TEXT,
		started_at DATETIME,
		completed_at DATETIME,
		stats TEXT,
		error TEXT
	)`); err != nil {
		return err
	}

	// Backfill columns for existing DBs
	addCols := []struct {
		name string
		ddl  string
	}{
		{"mode", `ALTER TABLE runs ADD COLUMN mode TEXT`},
		{"scale", `ALTER TABLE runs ADD COLUMN scale REAL`},
		{"resolved_counts", `ALTER TABLE runs ADD COLUMN resolved_counts TEXT`},
		{"execution_order", `ALTER TABLE runs ADD COLUMN execution_order TEXT`},
		{"warnings", `ALTER TABLE runs ADD COLUMN warnings TEXT`},
	}
	for _, c := range addCols {
		ok, err := hasColumn(db, "runs", c.name)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if _, err := db.Exec(c.ddl); err != nil {
			return err
		}
	}
	return nil
}

func migrateV2Targets(db *sql.DB) error {
	if _, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS targets (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		kind TEXT NOT NULL,
		dsn TEXT NOT NULL,
		schema TEXT,
		options_json TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`); err != nil {
		return err
	}
	_, _ = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_targets_name ON targets(name)`)
	return nil
}

func migrateV4TargetDatabase(db *sql.DB) error {
	ok, err := hasColumn(db, "targets", "database")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	_, err = db.Exec(`ALTER TABLE targets ADD COLUMN database TEXT`)
	return err
}

func migrateV5RunProgress(db *sql.DB) error {
	addCols := []struct {
		name string
		ddl  string
	}{
		{"progress_rows_generated", `ALTER TABLE runs ADD COLUMN progress_rows_generated INTEGER`},
		{"progress_rows_total", `ALTER TABLE runs ADD COLUMN progress_rows_total INTEGER`},
		{"progress_entities_done", `ALTER TABLE runs ADD COLUMN progress_entities_done INTEGER`},
		{"progress_entities_total", `ALTER TABLE runs ADD COLUMN progress_entities_total INTEGER`},
		{"progress_current_entity", `ALTER TABLE runs ADD COLUMN progress_current_entity TEXT`},
	}
	for _, c := range addCols {
		ok, err := hasColumn(db, "runs", c.name)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if _, err := db.Exec(c.ddl); err != nil {
			return err
		}
	}
	return nil
}

func migrateV6RunLogs(db *sql.DB) error {
	if _, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS run_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		level TEXT NOT NULL,
		message TEXT NOT NULL
	)`); err != nil {
		return err
	}
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_run_logs_run_time ON run_logs(run_id, id DESC)`)
	return nil
}

func migrateV3TargetChecks(db *sql.DB) error {
	if _, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS target_checks (
		id TEXT PRIMARY KEY,
		target_id TEXT NOT NULL,
		checked_at DATETIME NOT NULL,
		ok INTEGER NOT NULL,
		latency_ms INTEGER NOT NULL,
		server_version TEXT,
		capabilities_json TEXT,
		error TEXT
	)`); err != nil {
		return err
	}
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_target_checks_target_time ON target_checks(target_id, checked_at DESC)`)
	return nil
}

func (r *SQLiteRepository) Create(run *domain.Run) error {
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
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.ScenarioID, run.ScenarioName, run.ScenarioVersion,
		run.TargetID, run.TargetName, run.TargetKind,
		run.Seed, run.Mode, run.Scale, string(run.ResolvedCounts), string(run.ExecutionOrder), string(run.Warnings),
		run.ConfigHash, run.Status, run.StartedAt, string(statsJSON),
		run.ProgressRowsGenerated, run.ProgressRowsTotal, run.ProgressEntitiesDone, run.ProgressEntitiesTotal, run.ProgressCurrentEntity,
	)
	return err
}

func (r *SQLiteRepository) Update(run *domain.Run) error {
	statsJSON, err := json.Marshal(run.Stats)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`
	UPDATE runs SET
		status = ?, completed_at = ?, stats = ?, error = ?
	WHERE id = ?`,
		run.Status, run.CompletedAt, string(statsJSON), run.Error, run.ID,
	)
	return err
}

func (r *SQLiteRepository) Get(id string) (*domain.Run, error) {
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
	FROM runs WHERE id = ?`, id).Scan(
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

func (r *SQLiteRepository) List(limit int, status string) ([]*domain.Run, error) {
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
		WHERE status = ?
		ORDER BY started_at DESC
		LIMIT ?`, status, limit)
	} else {
		rows, err = r.db.Query(`
		SELECT id, scenario_id, scenario_name, scenario_version,
			target_id, target_name, target_kind,
			seed, mode, scale, resolved_counts, execution_order, warnings,
			config_hash, status, started_at, completed_at, stats, error,
			progress_rows_generated, progress_rows_total, progress_entities_done, progress_entities_total, progress_current_entity
		FROM runs
		ORDER BY started_at DESC
		LIMIT ?`, limit)
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

func (r *SQLiteRepository) UpdateStatus(id string, status domain.RunStatus, errMsg string, stats *domain.RunStats) error {
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
		SET status = ?, completed_at = ?, stats = COALESCE(?, stats), error = ?
		WHERE id = ?`,
		status, now, statsJSON, errMsg, id,
	)
	return err
}

func (r *SQLiteRepository) UpdateProgress(id string, rowsGenerated, rowsTotal int64, entitiesDone, entitiesTotal int, currentEntity string) error {
	_, err := r.db.Exec(`
		UPDATE runs
		SET progress_rows_generated = ?, progress_rows_total = ?, progress_entities_done = ?, progress_entities_total = ?, progress_current_entity = ?
		WHERE id = ?`,
		rowsGenerated, rowsTotal, entitiesDone, entitiesTotal, currentEntity, id,
	)
	return err
}

func (r *SQLiteRepository) AppendRunLog(runID, level, message string) error {
	_, err := r.db.Exec(`
		INSERT INTO run_logs (run_id, created_at, level, message)
		VALUES (?, ?, ?, ?)`,
		runID, time.Now().UTC(), level, message,
	)
	return err
}

func (r *SQLiteRepository) ListRunLogs(runID string, limit int) ([]*domain.RunLog, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.db.Query(`
		SELECT id, run_id, created_at, level, message
		FROM run_logs
		WHERE run_id = ?
		ORDER BY id DESC
		LIMIT ?`, runID, limit)
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
