package postgres

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/mmrzaf/sdgen/internal/domain"
)

type PostgresTarget struct {
	dsn    string
	schema string
	db     *sql.DB
}

func NewPostgresTarget(dsn, schema string) *PostgresTarget {
	if schema == "" {
		schema = "public"
	}
	return &PostgresTarget{
		dsn:    dsn,
		schema: schema,
	}
}

func (t *PostgresTarget) Connect() error {
	db, err := sql.Open("postgres", t.dsn)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}
	t.db = db
	return nil
}

func (t *PostgresTarget) Close() error {
	if t.db != nil {
		return t.db.Close()
	}
	return nil
}

func (t *PostgresTarget) CreateTableIfNotExists(entity *domain.Entity) error {
	var exists bool
	query := `SELECT EXISTS (
		SELECT FROM information_schema.tables 
		WHERE table_schema = $1 AND table_name = $2
	)`
	err := t.db.QueryRow(query, t.schema, entity.TargetTable).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		return t.validateExistingTable(entity)
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

	createSQL := fmt.Sprintf("CREATE TABLE %s.%s (%s)",
		t.schema, entity.TargetTable, strings.Join(columnDefs, ", "))

	_, err = t.db.Exec(createSQL)
	return err
}

func (t *PostgresTarget) validateExistingTable(entity *domain.Entity) error {
	rows, err := t.db.Query(`
		SELECT column_name, data_type
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2`, t.schema, entity.TargetTable)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := map[string]string{}
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return err
		}
		existing[name] = strings.ToLower(strings.TrimSpace(typ))
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, col := range entity.Columns {
		got, ok := existing[col.Name]
		if !ok {
			return fmt.Errorf("existing table %s.%s missing column %s", t.schema, entity.TargetTable, col.Name)
		}
		expected := strings.ToLower(strings.TrimSpace(t.mapColumnType(col.Type)))
		if !postgresTypeCompatible(expected, got) {
			return fmt.Errorf("existing table %s.%s column %s type mismatch: expected %s, got %s", t.schema, entity.TargetTable, col.Name, expected, got)
		}
	}
	return nil
}

func (t *PostgresTarget) mapColumnType(colType domain.ColumnType) string {
	switch colType {
	case domain.ColumnTypeInt:
		return "INTEGER"
	case domain.ColumnTypeBigInt:
		return "BIGINT"
	case domain.ColumnTypeFloat:
		return "REAL"
	case domain.ColumnTypeDouble:
		return "DOUBLE PRECISION"
	case domain.ColumnTypeString:
		return "VARCHAR(255)"
	case domain.ColumnTypeText:
		return "TEXT"
	case domain.ColumnTypeBool:
		return "BOOLEAN"
	case domain.ColumnTypeTimestamp:
		return "TIMESTAMP"
	case domain.ColumnTypeDate:
		return "DATE"
	case domain.ColumnTypeUUID:
		return "UUID"
	default:
		return "TEXT"
	}
}

func postgresTypeCompatible(expected, actual string) bool {
	if expected == actual {
		return true
	}
	switch expected {
	case "varchar(255)":
		return actual == "character varying" || actual == "text"
	case "double precision":
		return actual == "double precision" || actual == "real"
	case "integer":
		return actual == "integer" || actual == "smallint"
	case "timestamp":
		return actual == "timestamp without time zone" || actual == "timestamp with time zone" || actual == "timestamp"
	default:
		return false
	}
}

func (t *PostgresTarget) TruncateTable(tableName string) error {
	_, err := t.db.Exec(fmt.Sprintf("TRUNCATE TABLE %s.%s", t.schema, tableName))
	return err
}

func (t *PostgresTarget) InsertBatch(tableName string, columns []string, rows [][]interface{}) error {
	if len(rows) == 0 {
		return nil
	}

	quotedCols := make([]string, len(columns))
	for i, col := range columns {
		quotedCols[i] = col
	}

	placeholders := make([]string, len(rows))
	args := make([]interface{}, 0, len(rows)*len(columns))

	for i, row := range rows {
		rowPlaceholders := make([]string, len(columns))
		for j := range columns {
			paramNum := i*len(columns) + j + 1
			rowPlaceholders[j] = fmt.Sprintf("$%d", paramNum)

			val := row[j]
			if t, ok := val.(time.Time); ok {
				args = append(args, t)
			} else {
				args = append(args, val)
			}
		}
		placeholders[i] = "(" + strings.Join(rowPlaceholders, ", ") + ")"
	}

	insertSQL := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES %s",
		t.schema, tableName, strings.Join(quotedCols, ", "), strings.Join(placeholders, ", "))

	_, err := t.db.Exec(insertSQL, args...)
	return err
}
