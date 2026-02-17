package validation

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mmrzaf/sdgen/internal/domain"
	"github.com/mmrzaf/sdgen/internal/registry"
)

type Validator struct {
	genRegistry *registry.GeneratorRegistry
}

func NewValidator(genRegistry *registry.GeneratorRegistry) *Validator {
	return &Validator{genRegistry: genRegistry}
}

// identifier validation: allow simple SQL identifiers only (prevents injection via table/column names).
var (
	identRe       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	reservedWords = map[string]struct{}{
		"add": {}, "all": {}, "alter": {}, "and": {}, "any": {}, "as": {},
		"asc": {}, "between": {}, "by": {}, "case": {}, "check": {},
		"column": {}, "constraint": {}, "create": {}, "cross": {}, "current_date": {},
		"current_time": {}, "current_timestamp": {}, "database": {}, "default": {}, "delete": {},
		"desc": {}, "distinct": {}, "do": {}, "drop": {}, "else": {},
		"end": {}, "except": {}, "exists": {}, "false": {}, "for": {},
		"foreign": {}, "from": {}, "full": {}, "grant": {}, "group": {},
		"having": {}, "in": {}, "index": {}, "inner": {}, "insert": {},
		"intersect": {}, "into": {}, "is": {}, "join": {}, "key": {},
		"left": {}, "like": {}, "limit": {}, "natural": {}, "not": {},
		"null": {}, "offset": {}, "on": {}, "or": {}, "order": {},
		"outer": {}, "primary": {}, "references": {}, "returning": {}, "revoke": {},
		"right": {}, "schema": {}, "select": {}, "set": {}, "table": {},
		"then": {}, "to": {}, "true": {}, "truncate": {}, "union": {},
		"unique": {}, "update": {}, "user": {}, "using": {}, "values": {},
		"view": {}, "when": {}, "where": {}, "with": {},
	}
)

func IsValidIdentifier(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if !identRe.MatchString(s) {
		return false
	}
	if _, ok := reservedWords[strings.ToLower(s)]; ok {
		return false
	}
	return true
}

func (v *Validator) ValidateScenario(scenario *domain.Scenario) error {
	if scenario.Name == "" {
		return errors.New("scenario name is required")
	}

	if len(scenario.Entities) == 0 {
		return errors.New("scenario must have at least one entity")
	}

	entityNames := make(map[string]bool)
	for _, entity := range scenario.Entities {
		if err := v.validateEntity(&entity, entityNames); err != nil {
			return fmt.Errorf("entity '%s': %w", entity.Name, err)
		}
	}

	if err := v.validateDependencies(scenario); err != nil {
		return fmt.Errorf("dependency validation failed: %w", err)
	}

	return nil
}

func (v *Validator) validateEntity(entity *domain.Entity, entityNames map[string]bool) error {
	if entity.Name == "" {
		return errors.New("entity name is required")
	}
	if !IsValidIdentifier(entity.Name) {
		return fmt.Errorf("invalid entity identifier: %s", entity.Name)
	}

	if entityNames[entity.Name] {
		return fmt.Errorf("duplicate entity name: %s", entity.Name)
	}
	entityNames[entity.Name] = true

	if entity.TargetTable == "" {
		return errors.New("target_table is required")
	}
	if !IsValidIdentifier(entity.TargetTable) {
		return fmt.Errorf("invalid target_table identifier: %s", entity.TargetTable)
	}

	if entity.Rows <= 0 {
		return fmt.Errorf("rows must be > 0, got %d", entity.Rows)
	}

	if len(entity.Columns) == 0 {
		return errors.New("entity must have at least one column")
	}

	columnNames := make(map[string]bool)
	for _, col := range entity.Columns {
		if err := v.validateColumn(&col, columnNames); err != nil {
			return fmt.Errorf("column '%s': %w", col.Name, err)
		}
	}

	return nil
}

func (v *Validator) validateColumn(col *domain.Column, columnNames map[string]bool) error {
	if col.Name == "" {
		return errors.New("column name is required")
	}
	if !IsValidIdentifier(col.Name) {
		return fmt.Errorf("invalid column identifier: %s", col.Name)
	}

	if columnNames[col.Name] {
		return fmt.Errorf("duplicate column name: %s", col.Name)
	}
	columnNames[col.Name] = true

	if col.Type == "" {
		return errors.New("column type is required")
	}

	if !isValidColumnType(col.Type) {
		return fmt.Errorf("invalid column type: %s", col.Type)
	}

	if col.Generator.Type == "" {
		return errors.New("generator type is required")
	}

	gen, err := v.genRegistry.Get(col.Generator.Type)
	if err != nil {
		return fmt.Errorf("generator not found: %s", col.Generator.Type)
	}

	if err := gen.Validate(col.Generator, col.Type); err != nil {
		return fmt.Errorf("generator validation failed: %w", err)
	}

	// Optional FK metadata should be safe identifiers if present.
	if col.FK != nil {
		if col.FK.Entity == "" || col.FK.Column == "" {
			return errors.New("fk must include entity and column")
		}
		if !IsValidIdentifier(col.FK.Entity) {
			return fmt.Errorf("invalid fk entity identifier: %s", col.FK.Entity)
		}
		if !IsValidIdentifier(col.FK.Column) {
			return fmt.Errorf("invalid fk column identifier: %s", col.FK.Column)
		}
	}

	return nil
}

func isValidColumnType(t domain.ColumnType) bool {
	switch t {
	case domain.ColumnTypeInt, domain.ColumnTypeBigInt, domain.ColumnTypeFloat,
		domain.ColumnTypeDouble, domain.ColumnTypeString, domain.ColumnTypeText,
		domain.ColumnTypeBool, domain.ColumnTypeTimestamp, domain.ColumnTypeDate,
		domain.ColumnTypeUUID:
		return true
	default:
		return false
	}
}

func (v *Validator) validateDependencies(scenario *domain.Scenario) error {
	entityMap := make(map[string]*domain.Entity)
	for i := range scenario.Entities {
		entityMap[scenario.Entities[i].Name] = &scenario.Entities[i]
	}

	graph := make(map[string][]string)
	for _, entity := range scenario.Entities {
		deps := make([]string, 0)
		for _, col := range entity.Columns {
			if col.Generator.Type == "fk" {
				refEntity, ok := col.Generator.Params["entity"].(string)
				if !ok {
					return fmt.Errorf("entity '%s', column '%s': fk 'entity' param must be a string", entity.Name, col.Name)
				}
				refColumn, ok := col.Generator.Params["column"].(string)
				if !ok {
					return fmt.Errorf("entity '%s', column '%s': fk 'column' param must be a string", entity.Name, col.Name)
				}

				refEnt, exists := entityMap[refEntity]
				if !exists {
					return fmt.Errorf("entity '%s', column '%s': referenced entity '%s' not found", entity.Name, col.Name, refEntity)
				}

				columnExists := false
				for _, refCol := range refEnt.Columns {
					if refCol.Name == refColumn {
						columnExists = true
						break
					}
				}
				if !columnExists {
					return fmt.Errorf("entity '%s', column '%s': referenced column '%s.%s' not found", entity.Name, col.Name, refEntity, refColumn)
				}

				deps = append(deps, refEntity)
			}
		}
		graph[entity.Name] = deps
	}

	if hasCycle(graph) {
		return errors.New("cyclic dependencies detected")
	}

	return nil
}

func hasCycle(graph map[string][]string) bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for node := range graph {
		if !visited[node] {
			if hasCycleDFS(node, graph, visited, recStack) {
				return true
			}
		}
	}
	return false
}

func hasCycleDFS(node string, graph map[string][]string, visited, recStack map[string]bool) bool {
	visited[node] = true
	recStack[node] = true

	for _, neighbor := range graph[node] {
		if !visited[neighbor] {
			if hasCycleDFS(neighbor, graph, visited, recStack) {
				return true
			}
		} else if recStack[neighbor] {
			return true
		}
	}

	recStack[node] = false
	return false
}
func (v *Validator) ValidateTarget(t *domain.TargetConfig) error {
	if t.Name == "" {
		return errors.New("target name is required")
	}
	if t.Kind == "" {
		return errors.New("target kind is required")
	}
	if t.DSN == "" {
		return errors.New("target dsn is required")
	}
	if t.Database != "" && !IsValidIdentifier(t.Database) {
		return fmt.Errorf("invalid target database identifier: %s", t.Database)
	}

	switch t.Kind {
	case "postgres":
		if t.Schema != "" && !IsValidIdentifier(t.Schema) {
			return fmt.Errorf("invalid target schema identifier: %s", t.Schema)
		}
	case "elasticsearch":
		if t.Schema != "" {
			return fmt.Errorf("%s targets must not set schema", t.Kind)
		}
		if t.Database != "" {
			return errors.New("elasticsearch targets must not set database")
		}
	default:
		return fmt.Errorf("unsupported target kind: %s", t.Kind)
	}

	return nil
}

func (v *Validator) ValidateRunRequest(req *domain.RunRequest) error {
	hasScenarioID := req.ScenarioID != ""
	hasScenario := req.Scenario != nil

	if !hasScenarioID && !hasScenario {
		return errors.New("either scenario_id or scenario must be provided")
	}

	if hasScenarioID && hasScenario {
		return errors.New("only one of scenario_id or scenario must be provided")
	}

	hasTargetID := req.TargetID != ""
	hasTarget := req.Target != nil

	if !hasTargetID && !hasTarget {
		return errors.New("either target_id or target must be provided")
	}

	if hasTargetID && hasTarget {
		return errors.New("only one of target_id or target must be provided")
	}

	if req.Mode == "" {
		return errors.New("mode is required")
	}
	if !IsValidMode(req.Mode) {
		return fmt.Errorf("invalid mode: %s", req.Mode)
	}

	if req.Scale != nil && *req.Scale <= 0 {
		return fmt.Errorf("scale must be > 0, got %v", *req.Scale)
	}
	if req.TargetDatabase != "" && !IsValidIdentifier(req.TargetDatabase) {
		return fmt.Errorf("invalid target_database identifier: %s", req.TargetDatabase)
	}
	if req.EntityScales != nil {
		for k, v := range req.EntityScales {
			if !IsValidIdentifier(k) {
				return fmt.Errorf("invalid entity name in entity_scales: %s", k)
			}
			if v <= 0 {
				return fmt.Errorf("entity_scales[%s] must be > 0, got %v", k, v)
			}
		}
	}

	// entity_counts: must be >0 and valid identifiers
	if req.EntityCounts != nil {
		for k, v := range req.EntityCounts {
			if !IsValidIdentifier(k) {
				return fmt.Errorf("invalid entity name in entity_counts: %s", k)
			}
			if v <= 0 {
				return fmt.Errorf("entity_counts[%s] must be > 0, got %d", k, v)
			}
		}
	}
	for _, name := range req.IncludeEntities {
		if !IsValidIdentifier(name) {
			return fmt.Errorf("invalid entity name in include_entities: %s", name)
		}
	}
	for _, name := range req.ExcludeEntities {
		if !IsValidIdentifier(name) {
			return fmt.Errorf("invalid entity name in exclude_entities: %s", name)
		}
	}

	if req.Scenario != nil {
		if err := v.ValidateScenario(req.Scenario); err != nil {
			return fmt.Errorf("scenario validation failed: %w", err)
		}
	}

	if req.Target != nil {
		if err := v.ValidateTarget(req.Target); err != nil {
			return fmt.Errorf("target validation failed: %w", err)
		}
	}

	return nil
}

func TopologicalSort(scenario *domain.Scenario) ([]string, error) {
	graph := make(map[string][]string) // dependency -> dependents
	inDegree := make(map[string]int)

	for _, entity := range scenario.Entities {
		if _, ok := inDegree[entity.Name]; !ok {
			inDegree[entity.Name] = 0
		}
		for _, col := range entity.Columns {
			if col.Generator.Type == "fk" {
				v, ok := col.Generator.Params["entity"]
				if !ok {
					return nil, fmt.Errorf("entity '%s', column '%s': fk missing entity", entity.Name, col.Name)
				}
				refEntity, ok := v.(string)
				if !ok {
					return nil, fmt.Errorf("entity '%s', column '%s': fk entity must be string", entity.Name, col.Name)
				}
				graph[refEntity] = append(graph[refEntity], entity.Name)
				inDegree[entity.Name]++
			}
		}
		if _, ok := graph[entity.Name]; !ok {
			graph[entity.Name] = []string{}
		}
	}

	queue := make([]string, 0)
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	result := make([]string, 0)
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for _, dependent := range graph[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
		sort.Strings(queue)
	}

	if len(result) != len(scenario.Entities) {
		return nil, errors.New("cycle detected in entity dependencies")
	}

	return result, nil
}

func IsValidMode(mode string) bool {
	switch mode {
	case domain.TableModeCreate, domain.TableModeTruncate, domain.TableModeAppend:
		return true
	default:
		return false
	}
}
