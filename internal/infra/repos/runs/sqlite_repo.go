package runs

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mmrzaf/sdgen/internal/domain"
)

type Repository interface {
	Init() error
	Create(run *domain.Run) error
	Update(run *domain.Run) error
	Get(id string) (*domain.Run, error)
	List(limit int, status string) ([]*domain.Run, error)
}

type SQLiteRepository struct {
	dbPath string
	db     *sql.DB
}

func NewSQLiteRepository(dbPath string) *SQLiteRepository {
	return &SQLiteRepository{dbPath: dbPath}
}

func (r *SQLiteRepository) Init() error {
	db, err := sql.Open("sqlite3", r.dbPath)
	if err != nil {
		return err
	}
	r.db = db

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS runs (
		id TEXT PRIMARY KEY,
		scenario_id TEXT NOT NULL,
		scenario_name TEXT NOT NULL,
		scenario_version TEXT,
		target_id TEXT NOT NULL,
		target_name TEXT NOT NULL,
		target_kind TEXT NOT NULL,
		seed INTEGER NOT NULL,
		config_hash TEXT NOT NULL,
		status TEXT NOT NULL,
		started_at TIMESTAMP NOT NULL,
		completed_at TIMESTAMP,
		stats TEXT,
		error TEXT
	)`

	_, err = r.db.Exec(createTableSQL)
	return err
}

func (r *SQLiteRepository) Create(run *domain.Run) error {
	if run.ID == "" {
		run.ID = uuid.New().String()
	}

	statsJSON, err := json.Marshal(run.Stats)
	if err != nil {
		return err
	}

	var completedAt interface{}
	if run.CompletedAt != nil {
		completedAt = run.CompletedAt.Format(time.RFC3339)
	}

	query := `
		INSERT INTO runs (
			id, scenario_id, scenario_name, scenario_version,
			target_id, target_name, target_kind,
			seed, config_hash, status, started_at, completed_at, stats, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.Exec(query,
		run.ID, run.ScenarioID, run.ScenarioName, run.ScenarioVersion,
		run.TargetID, run.TargetName, run.TargetKind,
		run.Seed, run.ConfigHash, run.Status,
		run.StartedAt.Format(time.RFC3339), completedAt,
		string(statsJSON), run.Error,
	)
	return err
}

func (r *SQLiteRepository) Update(run *domain.Run) error {
	statsJSON, err := json.Marshal(run.Stats)
	if err != nil {
		return err
	}

	var completedAt interface{}
	if run.CompletedAt != nil {
		completedAt = run.CompletedAt.Format(time.RFC3339)
	}

	query := `
		UPDATE runs SET
			status = ?, completed_at = ?, stats = ?, error = ?
		WHERE id = ?
	`

	_, err = r.db.Exec(query, run.Status, completedAt, string(statsJSON), run.Error, run.ID)
	return err
}

func (r *SQLiteRepository) Get(id string) (*domain.Run, error) {
	query := `
		SELECT id, scenario_id, scenario_name, scenario_version,
		       target_id, target_name, target_kind,
		       seed, config_hash, status, started_at, completed_at, stats, error
		FROM runs WHERE id = ?
	`

	var run domain.Run
	var startedAtStr string
	var completedAtStr sql.NullString
	var statsStr sql.NullString
	var errorStr sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&run.ID, &run.ScenarioID, &run.ScenarioName, &run.ScenarioVersion,
		&run.TargetID, &run.TargetName, &run.TargetKind,
		&run.Seed, &run.ConfigHash, &run.Status,
		&startedAtStr, &completedAtStr, &statsStr, &errorStr,
	)
	if err != nil {
		return nil, err
	}

	run.StartedAt, _ = time.Parse(time.RFC3339, startedAtStr)
	if completedAtStr.Valid {
		t, _ := time.Parse(time.RFC3339, completedAtStr.String)
		run.CompletedAt = &t
	}
	if statsStr.Valid {
		run.Stats = json.RawMessage(statsStr.String)
	}
	if errorStr.Valid {
		run.Error = errorStr.String
	}

	return &run, nil
}

func (r *SQLiteRepository) List(limit int, status string) ([]*domain.Run, error) {
	query := `
		SELECT id, scenario_id, scenario_name, scenario_version,
		       target_id, target_name, target_kind,
		       seed, config_hash, status, started_at, completed_at, stats, error
		FROM runs
	`

	args := make([]interface{}, 0)
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}

	query += " ORDER BY started_at DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := make([]*domain.Run, 0)
	for rows.Next() {
		var run domain.Run
		var startedAtStr string
		var completedAtStr sql.NullString
		var statsStr sql.NullString
		var errorStr sql.NullString

		err := rows.Scan(
			&run.ID, &run.ScenarioID, &run.ScenarioName, &run.ScenarioVersion,
			&run.TargetID, &run.TargetName, &run.TargetKind,
			&run.Seed, &run.ConfigHash, &run.Status,
			&startedAtStr, &completedAtStr, &statsStr, &errorStr,
		)
		if err != nil {
			return nil, err
		}

		run.StartedAt, _ = time.Parse(time.RFC3339, startedAtStr)
		if completedAtStr.Valid {
			t, _ := time.Parse(time.RFC3339, completedAtStr.String)
			run.CompletedAt = &t
		}
		if statsStr.Valid {
			run.Stats = json.RawMessage(statsStr.String)
		}
		if errorStr.Valid {
			run.Error = errorStr.String
		}

		runs = append(runs, &run)
	}

	return runs, rows.Err()
}

func (r *SQLiteRepository) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}
