package validation

import (
	"errors"
	"fmt"

	"github.com/mmrzaf/sdgen/internal/domain"
	"github.com/mmrzaf/sdgen/internal/registry"
)

type Validator struct {
	genRegistry *registry.GeneratorRegistry
}

func NewValidator(genRegistry *registry.GeneratorRegistry) *Validator {
	return &Validator{genRegistry: genRegistry}
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

	if entityNames[entity.Name] {
		return fmt.Errorf("duplicate entity name: %s", entity.Name)
	}
	entityNames[entity.Name] = true

	if entity.TargetTable == "" {
		return errors.New("target_table is required")
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

func (v *Validator) ValidateTarget(target *domain.TargetConfig) error {
	if target.Name == "" {
		return errors.New("target name is required")
	}

	if target.Kind == "" {
		return errors.New("target kind is required")
	}

	if target.Kind != "postgres" && target.Kind != "sqlite" {
		return fmt.Errorf("unsupported target kind: %s", target.Kind)
	}

	if target.DSN == "" {
		return errors.New("target dsn is required")
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
	entityMap := make(map[string]*domain.Entity)
	for i := range scenario.Entities {
		entityMap[scenario.Entities[i].Name] = &scenario.Entities[i]
	}

	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, entity := range scenario.Entities {
		if _, ok := inDegree[entity.Name]; !ok {
			inDegree[entity.Name] = 0
		}
		deps := make([]string, 0)
		for _, col := range entity.Columns {
			if col.Generator.Type == "fk" {
				refEntity := col.Generator.Params["entity"].(string)
				deps = append(deps, refEntity)
				inDegree[entity.Name]++
			}
		}
		graph[entity.Name] = deps
	}

	queue := make([]string, 0)
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	result := make([]string, 0)
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for name, deps := range graph {
			for _, dep := range deps {
				if dep == node {
					inDegree[name]--
					if inDegree[name] == 0 {
						queue = append(queue, name)
					}
				}
			}
		}
	}

	if len(result) != len(scenario.Entities) {
		return nil, errors.New("cycle detected in entity dependencies")
	}

	return result, nil
}
