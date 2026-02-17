package generators

import (
	"errors"
	"math/rand"

	"github.com/mmrzaf/sdgen/internal/domain"
)

type UniformFloatGenerator struct{}

func (g *UniformFloatGenerator) Generate(rng *rand.Rand, ctx GeneratorContext) (interface{}, error) {
	return nil, errors.New("uniform_float generator requires params")
}

func (g *UniformFloatGenerator) Validate(spec domain.GeneratorSpec, columnType domain.ColumnType) error {
	if spec.Params == nil {
		return errors.New("uniform_float requires 'min' and 'max' params")
	}
	_, hasMin := spec.Params["min"]
	_, hasMax := spec.Params["max"]
	if !hasMin || !hasMax {
		return errors.New("uniform_float requires 'min' and 'max' params")
	}
	return nil
}

func (g *UniformFloatGenerator) GenerateWithParams(rng *rand.Rand, params map[string]interface{}) (interface{}, error) {
	minVal, ok := params["min"]
	if !ok {
		return nil, errors.New("missing 'min' param")
	}
	maxVal, ok := params["max"]
	if !ok {
		return nil, errors.New("missing 'max' param")
	}

	min := toFloat64(minVal)
	max := toFloat64(maxVal)

	return min + rng.Float64()*(max-min), nil
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0.0
	}
}
