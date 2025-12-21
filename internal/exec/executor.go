package exec

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/mmrzaf/sdgen/internal/domain"
	"github.com/mmrzaf/sdgen/internal/generators"
	"github.com/mmrzaf/sdgen/internal/registry"
	"github.com/mmrzaf/sdgen/internal/validation"
)

type Target interface {
	Connect() error
	Close() error
	CreateTableIfNotExists(entity *domain.Entity) error
	TruncateTable(tableName string) error
	InsertBatch(tableName string, columns []string, rows [][]interface{}) error
}

type Executor struct {
	genRegistry *registry.GeneratorRegistry
}

func NewExecutor(genRegistry *registry.GeneratorRegistry) *Executor {
	return &Executor{genRegistry: genRegistry}
}

func (e *Executor) Execute(scenario *domain.Scenario, target Target, seed int64, mode string) (*domain.RunStats, error) {
	if err := target.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to target: %w", err)
	}
	defer target.Close()

	order, err := validation.TopologicalSort(scenario)
	if err != nil {
		return nil, fmt.Errorf("failed to sort entities: %w", err)
	}

	entityMap := make(map[string]*domain.Entity)
	for i := range scenario.Entities {
		entityMap[scenario.Entities[i].Name] = &scenario.Entities[i]
	}

	entityValues := make(map[string][]interface{})
	stats := &domain.RunStats{
		EntityStats: make([]domain.EntityRunStats, 0),
	}

	for _, entityName := range order {
		entity := entityMap[entityName]
		startTime := time.Now()

		entitySeed := seed + int64(len(entityName))
		rng := rand.New(rand.NewSource(entitySeed))

		entityMode := mode
		if entity.TableMode != "" {
			entityMode = entity.TableMode
		}

		if entityMode == "" {
			entityMode = domain.TableModeCreateIfMissing
		}

		switch entityMode {
		case domain.TableModeCreateIfMissing:
			if err := target.CreateTableIfNotExists(entity); err != nil {
				return nil, fmt.Errorf("failed to create table for entity '%s': %w", entity.Name, err)
			}
		case domain.TableModeTruncateThenInsert:
			if err := target.CreateTableIfNotExists(entity); err != nil {
				return nil, fmt.Errorf("failed to create table for entity '%s': %w", entity.Name, err)
			}
			if err := target.TruncateTable(entity.TargetTable); err != nil {
				return nil, fmt.Errorf("failed to truncate table for entity '%s': %w", entity.Name, err)
			}
		case domain.TableModeAppendOnly:
		default:
			return nil, fmt.Errorf("unknown table mode: %s", entityMode)
		}

		columnNames := make([]string, len(entity.Columns))
		for i, col := range entity.Columns {
			columnNames[i] = col.Name
		}

		fkColumnIndices := make(map[int]bool)
		for i, col := range entity.Columns {
			if col.Generator.Type == "fk" {
				refEntity := col.Generator.Params["entity"].(string)
				refColumn := col.Generator.Params["column"].(string)
				key := refEntity + "." + refColumn
				if _, exists := entityValues[key]; !exists {
					return nil, fmt.Errorf("FK reference %s not yet generated", key)
				}
				fkColumnIndices[i] = true
			}
		}

		const batchSize = 1000
		batch := make([][]interface{}, 0, batchSize)

		for rowIdx := int64(0); rowIdx < entity.Rows; rowIdx++ {
			row := make([]interface{}, len(entity.Columns))
			ctx := generators.GeneratorContext{
				RowIndex:     rowIdx,
				EntityValues: entityValues,
			}

			for colIdx, col := range entity.Columns {
				val, err := e.generateValue(rng, col, ctx)
				if err != nil {
					return nil, fmt.Errorf("entity '%s', column '%s', row %d: %w", entity.Name, col.Name, rowIdx, err)
				}
				row[colIdx] = val

				if !fkColumnIndices[colIdx] {
					key := entity.Name + "." + col.Name
					entityValues[key] = append(entityValues[key], val)
				}
			}

			batch = append(batch, row)

			if len(batch) >= batchSize {
				if err := target.InsertBatch(entity.TargetTable, columnNames, batch); err != nil {
					return nil, fmt.Errorf("failed to insert batch for entity '%s': %w", entity.Name, err)
				}
				batch = batch[:0]
			}
		}

		if len(batch) > 0 {
			if err := target.InsertBatch(entity.TargetTable, columnNames, batch); err != nil {
				return nil, fmt.Errorf("failed to insert final batch for entity '%s': %w", entity.Name, err)
			}
		}

		duration := time.Since(startTime)
		stats.EntityStats = append(stats.EntityStats, domain.EntityRunStats{
			EntityName:      entity.Name,
			RowsGenerated:   entity.Rows,
			DurationSeconds: duration.Seconds(),
		})
		stats.TotalRows += entity.Rows
	}

	stats.EntitiesGenerated = len(order)
	return stats, nil
}

func (e *Executor) generateValue(rng *rand.Rand, col domain.Column, ctx generators.GeneratorContext) (interface{}, error) {
	gen, err := e.genRegistry.Get(col.Generator.Type)
	if err != nil {
		return nil, err
	}

	switch col.Generator.Type {
	case "const":
		constGen := gen.(*generators.ConstGenerator)
		return constGen.GenerateValue(col.Generator), nil
	case "uniform_int":
		uintGen := gen.(*generators.UniformIntGenerator)
		return uintGen.GenerateWithParams(rng, col.Generator.Params)
	case "uniform_float":
		ufloatGen := gen.(*generators.UniformFloatGenerator)
		return ufloatGen.GenerateWithParams(rng, col.Generator.Params)
	case "normal":
		normalGen := gen.(*generators.NormalGenerator)
		return normalGen.GenerateWithParams(rng, col.Generator.Params)
	case "choice":
		choiceGen := gen.(*generators.ChoiceGenerator)
		return choiceGen.GenerateWithParams(rng, col.Generator.Params)
	case "time_series":
		tsGen := gen.(*generators.TimeSeriesGenerator)
		return tsGen.GenerateWithParams(rng, col.Generator.Params, ctx.RowIndex)
	case "fk":
		fkGen := gen.(*generators.FKGenerator)
		return fkGen.GenerateWithContext(rng, col.Generator.Params, ctx)
	default:
		return gen.Generate(rng, ctx)
	}
}
