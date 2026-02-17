package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mmrzaf/sdgen/internal/domain"
)

type SQLiteTarget struct {
	path string
	db   *sql.DB
}

func NewSQLiteTarget(path string) *SQLiteTarget {
	return &SQLiteTarget{path: path}
}

func (t *SQLiteTarget) Connect() error {
	db, err := sql.Open("sqlite3", t.path)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}
	t.db = db
	return nil
}

func (t *SQLiteTarget) Close() error {
	if t.db != nil {
		return t.db.Close()
	}
	return nil
}

func (t *SQLiteTarget) CreateTableIfNotExists(entity *domain.Entity) error {
	query := `SELECT name FROM sqlite_master WHERE type='table' AND name=?`
	var name string
	err := t.db.QueryRow(query, entity.TargetTable).Scan(&name)
	if err == nil {
		return t.validateExistingTable(entity)
	}
	if err != sql.ErrNoRows {
		return err
	}

	columnDefs := make([]string, len(entity.Columns))
	for i, col := range entity.Columns {
		colType := t.mapColumnType(col.Type)
		nullable := ""
		if !col.Nullable {
			nullable = " NOT NULL"
		}
		columnDefs[i] = fmt.Sprintf("%s %s%s", col.Name, colType, nullable)
	}

	createSQL := fmt.Sprintf("CREATE TABLE %s (%s)",
		entity.TargetTable, strings.Join(columnDefs, ", "))

	_, err = t.db.Exec(createSQL)
	return err
}

func (t *SQLiteTarget) validateExistingTable(entity *domain.Entity) error {
	rows, err := t.db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, entity.TargetTable))
	if err != nil {
		return err
	}
	defer rows.Close()

	type colInfo struct {
		name string
		typ  string
	}
	existing := map[string]colInfo{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return err
		}
		existing[name] = colInfo{name: name, typ: strings.ToUpper(strings.TrimSpace(typ))}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, col := range entity.Columns {
		got, ok := existing[col.Name]
		if !ok {
			return fmt.Errorf("existing table %s missing column %s", entity.TargetTable, col.Name)
		}
		expectedType := t.mapColumnType(col.Type)
		if !sqliteTypeCompatible(expectedType, got.typ) {
			return fmt.Errorf("existing table %s column %s type mismatch: expected %s, got %s", entity.TargetTable, col.Name, expectedType, got.typ)
		}
	}
	return nil
}

func (t *SQLiteTarget) mapColumnType(colType domain.ColumnType) string {
	switch colType {
	case domain.ColumnTypeInt:
		return "INTEGER"
	case domain.ColumnTypeBigInt:
		return "INTEGER"
	case domain.ColumnTypeFloat, domain.ColumnTypeDouble:
		return "REAL"
	case domain.ColumnTypeString, domain.ColumnTypeText:
		return "TEXT"
	case domain.ColumnTypeBool:
		return "INTEGER"
	case domain.ColumnTypeTimestamp, domain.ColumnTypeDate:
		return "TEXT"
	case domain.ColumnTypeUUID:
		return "TEXT"
	default:
		return "TEXT"
	}
}

func sqliteTypeCompatible(expected, actual string) bool {
	expected = strings.ToUpper(expected)
	actual = strings.ToUpper(actual)
	if expected == actual {
		return true
	}
	// SQLite type affinity allows INTEGER variants to interoperate.
	if expected == "INTEGER" && strings.Contains(actual, "INT") {
		return true
	}
	if expected == "REAL" && (strings.Contains(actual, "REAL") || strings.Contains(actual, "FLOAT") || strings.Contains(actual, "DOUBLE")) {
		return true
	}
	if expected == "TEXT" && (strings.Contains(actual, "TEXT") || strings.Contains(actual, "CHAR") || strings.Contains(actual, "CLOB") || actual == "") {
		return true
	}
	return false
}

func (t *SQLiteTarget) TruncateTable(tableName string) error {
	_, err := t.db.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
	return err
}

func (t *SQLiteTarget) InsertBatch(tableName string, columns []string, rows [][]interface{}) error {
	if len(rows) == 0 {
		return nil
	}

	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = "?"
	}

	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, row := range rows {
		args := make([]interface{}, len(row))
		for i, val := range row {
			if t, ok := val.(time.Time); ok {
				args[i] = t.Format(time.RFC3339)
			} else if b, ok := val.(bool); ok {
				if b {
					args[i] = 1
				} else {
					args[i] = 0
				}
			} else {
				args[i] = val
			}
		}
		if _, err := stmt.Exec(args...); err != nil {
			return err
		}
	}

	return tx.Commit()
}
