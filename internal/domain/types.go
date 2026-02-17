package domain

import (
	"encoding/json"
	"time"
)

type Scenario struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	Version     string    `json:"version" yaml:"version"`
	Description string    `json:"description" yaml:"description"`
	Seed        *int64    `json:"seed,omitempty" yaml:"seed,omitempty"`
	Entities    []Entity  `json:"entities" yaml:"entities"`
}

type Entity struct {
	Name        string   `json:"name" yaml:"name"`
	TargetTable string   `json:"target_table" yaml:"target_table"`
	Rows        int64    `json:"rows" yaml:"rows"`
	Columns     []Column `json:"columns" yaml:"columns"`
	TableMode   string   `json:"table_mode,omitempty" yaml:"table_mode,omitempty"`
}

type Column struct {
	Name      string        `json:"name" yaml:"name"`
	Type      ColumnType    `json:"type" yaml:"type"`
	Nullable  bool          `json:"nullable,omitempty" yaml:"nullable,omitempty"`
	Generator GeneratorSpec `json:"generator" yaml:"generator"`
	FK        *ForeignKey   `json:"fk,omitempty" yaml:"fk,omitempty"`
}

type ColumnType string

const (
	ColumnTypeInt       ColumnType = "int"
	ColumnTypeBigInt    ColumnType = "bigint"
	ColumnTypeFloat     ColumnType = "float"
	ColumnTypeDouble    ColumnType = "double"
	ColumnTypeString    ColumnType = "string"
	ColumnTypeText      ColumnType = "text"
	ColumnTypeBool      ColumnType = "bool"
	ColumnTypeTimestamp ColumnType = "timestamp"
	ColumnTypeDate      ColumnType = "date"
	ColumnTypeUUID      ColumnType = "uuid"
)

type GeneratorSpec struct {
	Type   string                 `json:"type" yaml:"type"`
	Params map[string]interface{} `json:"params,omitempty" yaml:"params,omitempty"`
}

type ForeignKey struct {
	Entity string `json:"entity" yaml:"entity"`
	Column string `json:"column" yaml:"column"`
}

type TargetConfig struct {
	ID      string            `json:"id" yaml:"id"`
	Name    string            `json:"name" yaml:"name"`
	Kind    string            `json:"kind" yaml:"kind"`
	DSN     string            `json:"dsn" yaml:"dsn"`
	Schema  string            `json:"schema,omitempty" yaml:"schema,omitempty"`
	Options map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
}

type Run struct {
	ID              string          `json:"id"`
	ScenarioID      string          `json:"scenario_id"`
	ScenarioName    string          `json:"scenario_name"`
	ScenarioVersion string          `json:"scenario_version"`
	TargetID        string          `json:"target_id"`
	TargetName      string          `json:"target_name"`
	TargetKind      string          `json:"target_kind"`
	Seed            int64           `json:"seed"`
	ConfigHash      string          `json:"config_hash"`
	Status          RunStatus       `json:"status"`
	StartedAt       time.Time       `json:"started_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
	Stats           json.RawMessage `json:"stats,omitempty"`
	Error           string          `json:"error,omitempty"`
}

type RunStatus string

const (
	RunStatusPending RunStatus = "pending"
	RunStatusRunning RunStatus = "running"
	RunStatusSuccess RunStatus = "success"
	RunStatusFailed  RunStatus = "failed"
)

type RunStats struct {
	EntitiesGenerated int              `json:"entities_generated"`
	TotalRows         int64            `json:"total_rows"`
	DurationSeconds   float64          `json:"duration_seconds"`
	EntityStats       []EntityRunStats `json:"entity_stats"`
}

type EntityRunStats struct {
	EntityName      string  `json:"entity_name"`
	RowsGenerated   int64   `json:"rows_generated"`
	DurationSeconds float64 `json:"duration_seconds"`
}

type RunRequest struct {
	ScenarioID   string            `json:"scenario_id,omitempty"`
	Scenario     *Scenario         `json:"scenario,omitempty"`
	TargetID     string            `json:"target_id,omitempty"`
	Target       *TargetConfig     `json:"target,omitempty"`
	Seed         *int64            `json:"seed,omitempty"`
	RowOverrides map[string]int64  `json:"row_overrides,omitempty"`
	Mode         string            `json:"mode,omitempty"`
}

const (
	TableModeCreateIfMissing  = "create_if_missing"
	TableModeTruncateThenInsert = "truncate_then_insert"
	TableModeAppendOnly       = "append_only"
)
