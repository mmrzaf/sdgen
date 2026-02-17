package targets

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mmrzaf/sdgen/internal/domain"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) List() ([]*domain.TargetConfig, error) {
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

func (r *PostgresRepository) Get(id string) (*domain.TargetConfig, error) {
	var t domain.TargetConfig
	var database sql.NullString
	var schema sql.NullString
	var opt sql.NullString
	err := r.db.QueryRow(`
		SELECT id, name, kind, dsn, database, schema, options_json
		FROM targets
		WHERE id = $1`, id).Scan(&t.ID, &t.Name, &t.Kind, &t.DSN, &database, &schema, &opt)
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

func (r *PostgresRepository) Create(t *domain.TargetConfig) error {
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		t.ID, t.Name, t.Kind, t.DSN, nullIfEmpty(t.Database), nullIfEmpty(t.Schema), nullIfEmpty(opt), now, now)
	return err
}

func (r *PostgresRepository) Update(t *domain.TargetConfig) error {
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
		SET name = $1, kind = $2, dsn = $3, database = $4, schema = $5, options_json = $6, updated_at = $7
		WHERE id = $8`,
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

func (r *PostgresRepository) Delete(id string) error {
	res, err := r.db.Exec(`DELETE FROM targets WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PostgresRepository) RecordCheck(c *domain.TargetCheck) error {
	if c == nil {
		return errors.New("nil check")
	}
	b, _ := json.Marshal(c.Capabilities)

	_, err := r.db.Exec(`
		INSERT INTO target_checks (id, target_id, checked_at, ok, latency_ms, server_version, capabilities_json, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		c.ID, c.TargetID, c.CheckedAt, c.OK, c.LatencyMS, nullIfEmpty(c.ServerVer), nullIfEmpty(string(b)), nullIfEmpty(c.Error))
	return err
}

func (r *PostgresRepository) ListChecks(targetID string, limit int) ([]*domain.TargetCheck, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(`
		SELECT id, target_id, checked_at, ok, latency_ms, server_version, capabilities_json, error
		FROM target_checks
		WHERE target_id = $1
		ORDER BY checked_at DESC
		LIMIT $2`, targetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.TargetCheck
	for rows.Next() {
		var c domain.TargetCheck
		var ok bool
		var server sql.NullString
		var caps sql.NullString
		var errStr sql.NullString
		if err := rows.Scan(&c.ID, &c.TargetID, &c.CheckedAt, &ok, &c.LatencyMS, &server, &caps, &errStr); err != nil {
			return nil, err
		}
		c.OK = ok
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
