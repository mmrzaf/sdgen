package generators

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/mmrzaf/sdgen/internal/domain"
)

type ChoiceGenerator struct{}

func (g *ChoiceGenerator) Generate(rng *rand.Rand, ctx GeneratorContext) (interface{}, error) {
	return nil, errors.New("choice generator requires params")
}

func (g *ChoiceGenerator) Validate(spec domain.GeneratorSpec, columnType domain.ColumnType) error {
	if spec.Params == nil {
		return errors.New("choice requires 'values' param")
	}
	valuesRaw, ok := spec.Params["values"]
	if !ok {
		return errors.New("choice requires 'values' param")
	}

	values, ok := valuesRaw.([]interface{})
	if !ok {
		return errors.New("'values' must be a list")
	}

	if len(values) == 0 {
		return errors.New("'values' cannot be empty")
	}

	if weightsRaw, hasWeights := spec.Params["weights"]; hasWeights {
		weights, ok := weightsRaw.([]interface{})
		if !ok {
			return errors.New("'weights' must be a list")
		}
		if len(weights) != len(values) {
			return errors.New("'weights' and 'values' must have the same length")
		}
	}

	return nil
}

func (g *ChoiceGenerator) GenerateWithParams(rng *rand.Rand, params map[string]interface{}) (interface{}, error) {
	valuesRaw, ok := params["values"]
	if !ok {
		return nil, errors.New("missing 'values' param")
	}

	values, ok := valuesRaw.([]interface{})
	if !ok {
		return nil, errors.New("'values' must be a list")
	}

	if len(values) == 0 {
		return nil, errors.New("'values' cannot be empty")
	}

	weightsRaw, hasWeights := params["weights"]
	if !hasWeights {
		return values[rng.Intn(len(values))], nil
	}

	weights, ok := weightsRaw.([]interface{})
	if !ok {
		return nil, errors.New("'weights' must be a list")
	}

	if len(weights) != len(values) {
		return nil, errors.New("'weights' and 'values' must have the same length")
	}

	totalWeight := 0.0
	for _, w := range weights {
		weight := toFloat64(w)
		if weight < 0 {
			return nil, fmt.Errorf("negative weight: %v", w)
		}
		totalWeight += weight
	}

	if totalWeight == 0 {
		return nil, errors.New("total weight is zero")
	}

	r := rng.Float64() * totalWeight
	cumWeight := 0.0
	for i, w := range weights {
		cumWeight += toFloat64(w)
		if r < cumWeight {
			return values[i], nil
		}
	}

	return values[len(values)-1], nil
}
