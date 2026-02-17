package domain

import (
	"encoding/json"
	"time"
)

type Scenario struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Version     string   `json:"version" yaml:"version"`
	Description string   `json:"description" yaml:"description"`
	Seed        *int64   `json:"seed,omitempty" yaml:"seed,omitempty"`
	Entities    []Entity `json:"entities" yaml:"entities"`
}

type Entity struct {
	Name        string   `json:"name" yaml:"name"`
	TargetTable string   `json:"target_table" yaml:"target_table"`
	Rows        int64    `json:"rows" yaml:"rows"`
	Columns     []Column `json:"columns" yaml:"columns"`
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
	ID       string            `json:"id" yaml:"id"`
	Name     string            `json:"name" yaml:"name"`
	Kind     string            `json:"kind" yaml:"kind"`
	DSN      string            `json:"dsn" yaml:"dsn"`
	Database string            `json:"database,omitempty" yaml:"database,omitempty"`
	Schema   string            `json:"schema,omitempty" yaml:"schema,omitempty"`
	Options  map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
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

	// Extensions (safe: older DB rows won’t populate these unless you migrate/scan them)
	Mode           string          `json:"mode,omitempty"`
	Scale          *float64        `json:"scale,omitempty"`
	ResolvedCounts json.RawMessage `json:"resolved_counts,omitempty"`
	ExecutionOrder json.RawMessage `json:"execution_order,omitempty"`
	Warnings       json.RawMessage `json:"warnings,omitempty"`

	ProgressRowsGenerated   int64  `json:"progress_rows_generated,omitempty"`
	ProgressRowsTotal       int64  `json:"progress_rows_total,omitempty"`
	ProgressEntitiesDone    int    `json:"progress_entities_done,omitempty"`
	ProgressEntitiesTotal   int    `json:"progress_entities_total,omitempty"`
	ProgressCurrentEntity   string `json:"progress_current_entity,omitempty"`
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

type TargetCapabilities struct {
	CanCreate   bool `json:"can_create"`
	CanTruncate bool `json:"can_truncate"`
	CanInsert   bool `json:"can_insert"`
}

type TargetCheck struct {
	ID           string             `json:"id"`
	TargetID     string             `json:"target_id"`
	CheckedAt    time.Time          `json:"checked_at"`
	OK           bool               `json:"ok"`
	LatencyMS    int64              `json:"latency_ms"`
	ServerVer    string             `json:"server_version,omitempty"`
	Capabilities TargetCapabilities `json:"capabilities"`
	Error        string             `json:"error,omitempty"`
}

type RunLog struct {
	ID        int64     `json:"id"`
	RunID     string    `json:"run_id"`
	CreatedAt time.Time `json:"created_at"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

type RunPlan struct {
	ExecutionOrder []string         `json:"execution_order"`
	ResolvedCounts map[string]int64 `json:"resolved_counts"`
	Scale          float64          `json:"scale"`
	Warnings       []string         `json:"warnings,omitempty"`
}

type RunRequest struct {
	ScenarioID      string             `json:"scenario_id,omitempty"`
	Scenario        *Scenario          `json:"scenario,omitempty"`
	TargetID        string             `json:"target_id,omitempty"`
	Target          *TargetConfig      `json:"target,omitempty"`
	TargetDatabase  string             `json:"target_database,omitempty"`
	Seed            *int64             `json:"seed,omitempty"`
	Scale           *float64           `json:"scale,omitempty"`
	EntityScales    map[string]float64 `json:"entity_scales,omitempty"`
	EntityCounts    map[string]int64   `json:"entity_counts,omitempty"`
	IncludeEntities []string           `json:"include_entities,omitempty"`
	ExcludeEntities []string           `json:"exclude_entities,omitempty"`
	Mode            string             `json:"mode,omitempty"`
}

const (
	TableModeCreate   = "create"
	TableModeTruncate = "truncate"
	TableModeAppend   = "append"
)
