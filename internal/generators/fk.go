package generators

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/mmrzaf/sdgen/internal/domain"
)

type FKGenerator struct{}

func (g *FKGenerator) Generate(rng *rand.Rand, ctx GeneratorContext) (interface{}, error) {
	return nil, errors.New("fk generator requires params")
}

func (g *FKGenerator) Validate(spec domain.GeneratorSpec, columnType domain.ColumnType) error {
	if spec.Params == nil {
		return errors.New("fk requires 'entity' and 'column' params")
	}
	_, hasEntity := spec.Params["entity"]
	_, hasColumn := spec.Params["column"]
	if !hasEntity || !hasColumn {
		return errors.New("fk requires 'entity' and 'column' params")
	}
	return nil
}

func (g *FKGenerator) GenerateWithContext(rng *rand.Rand, params map[string]interface{}, ctx GeneratorContext) (interface{}, error) {
	entityName, ok := params["entity"].(string)
	if !ok {
		return nil, errors.New("'entity' must be a string")
	}

	columnName, ok := params["column"].(string)
	if !ok {
		return nil, errors.New("'column' must be a string")
	}

	key := entityName + "." + columnName
	values, ok := ctx.EntityValues[key]
	if !ok {
		return nil, fmt.Errorf("no values found for FK reference: %s", key)
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("empty values for FK reference: %s", key)
	}

	return values[rng.Intn(len(values))], nil
}
