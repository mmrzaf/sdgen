package targets

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mmrzaf/sdgen/internal/domain"
)

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) List() ([]*domain.TargetConfig, error) {
	rows, err := r.db.Query(`
		SELECT id, name, kind, dsn, database, schema, options_json
		FROM targets
		ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.TargetConfig
	for rows.Next() {
		var t domain.TargetConfig
		var database sql.NullString
		var schema sql.NullString
		var opt sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.Kind, &t.DSN, &database, &schema, &opt); err != nil {
			return nil, err
		}
		if database.Valid {
			t.Database = database.String
		}
		if schema.Valid {
			t.Schema = schema.String
		}
		if opt.Valid && opt.String != "" {
			_ = json.Unmarshal([]byte(opt.String), &t.Options)
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) Get(id string) (*domain.TargetConfig, error) {
	var t domain.TargetConfig
	var database sql.NullString
	var schema sql.NullString
	var opt sql.NullString
	err := r.db.QueryRow(`
		SELECT id, name, kind, dsn, database, schema, options_json
		FROM targets
		WHERE id = ?`, id).Scan(&t.ID, &t.Name, &t.Kind, &t.DSN, &database, &schema, &opt)
	if err != nil {
		return nil, err
	}
	if database.Valid {
		t.Database = database.String
	}
	if schema.Valid {
		t.Schema = schema.String
	}
	if opt.Valid && opt.String != "" {
		_ = json.Unmarshal([]byte(opt.String), &t.Options)
	}
	return &t, nil
}

func (r *SQLiteRepository) Create(t *domain.TargetConfig) error {
	if t == nil {
		return errors.New("nil target")
	}
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	now := time.Now().UTC()

	var opt string
	if t.Options != nil {
		b, _ := json.Marshal(t.Options)
		opt = string(b)
	}

	_, err := r.db.Exec(`
		INSERT INTO targets (id, name, kind, dsn, database, schema, options_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.Kind, t.DSN, nullIfEmpty(t.Database), nullIfEmpty(t.Schema), nullIfEmpty(opt), now, now)
	return err
}

func (r *SQLiteRepository) Update(t *domain.TargetConfig) error {
	if t == nil {
		return errors.New("nil target")
	}
	if t.ID == "" {
		return errors.New("missing target id")
	}
	now := time.Now().UTC()

	var opt string
	if t.Options != nil {
		b, _ := json.Marshal(t.Options)
		opt = string(b)
	}

	res, err := r.db.Exec(`
		UPDATE targets
		SET name = ?, kind = ?, dsn = ?, database = ?, schema = ?, options_json = ?, updated_at = ?
		WHERE id = ?`,
		t.Name, t.Kind, t.DSN, nullIfEmpty(t.Database), nullIfEmpty(t.Schema), nullIfEmpty(opt), now, t.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *SQLiteRepository) Delete(id string) error {
	res, err := r.db.Exec(`DELETE FROM targets WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *SQLiteRepository) RecordCheck(c *domain.TargetCheck) error {
	if c == nil {
		return errors.New("nil check")
	}
	b, _ := json.Marshal(c.Capabilities)

	_, err := r.db.Exec(`
		INSERT INTO target_checks (id, target_id, checked_at, ok, latency_ms, server_version, capabilities_json, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.TargetID, c.CheckedAt, boolToInt(c.OK), c.LatencyMS, nullIfEmpty(c.ServerVer), nullIfEmpty(string(b)), nullIfEmpty(c.Error))
	return err
}

func (r *SQLiteRepository) ListChecks(targetID string, limit int) ([]*domain.TargetCheck, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(`
		SELECT id, target_id, checked_at, ok, latency_ms, server_version, capabilities_json, error
		FROM target_checks
		WHERE target_id = ?
		ORDER BY checked_at DESC
		LIMIT ?`, targetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.TargetCheck
	for rows.Next() {
		var c domain.TargetCheck
		var okInt int
		var server sql.NullString
		var caps sql.NullString
		var errStr sql.NullString
		if err := rows.Scan(&c.ID, &c.TargetID, &c.CheckedAt, &okInt, &c.LatencyMS, &server, &caps, &errStr); err != nil {
			return nil, err
		}
		c.OK = okInt == 1
		if server.Valid {
			c.ServerVer = server.String
		}
		if caps.Valid && caps.String != "" {
			_ = json.Unmarshal([]byte(caps.String), &c.Capabilities)
		}
		if errStr.Valid {
			c.Error = errStr.String
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
