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
		return nil
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
