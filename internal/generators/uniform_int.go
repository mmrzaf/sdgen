package generators

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/mmrzaf/sdgen/internal/domain"
)

type UniformIntGenerator struct{}

func (g *UniformIntGenerator) Generate(rng *rand.Rand, ctx GeneratorContext) (interface{}, error) {
	return nil, errors.New("uniform_int generator requires params")
}

func (g *UniformIntGenerator) Validate(spec domain.GeneratorSpec, columnType domain.ColumnType) error {
	if spec.Params == nil {
		return errors.New("uniform_int requires 'min' and 'max' params")
	}
	_, hasMin := spec.Params["min"]
	_, hasMax := spec.Params["max"]
	if !hasMin || !hasMax {
		return errors.New("uniform_int requires 'min' and 'max' params")
	}
	return nil
}

func (g *UniformIntGenerator) GenerateWithParams(rng *rand.Rand, params map[string]interface{}) (interface{}, error) {
	minVal, ok := params["min"]
	if !ok {
		return nil, errors.New("missing 'min' param")
	}
	maxVal, ok := params["max"]
	if !ok {
		return nil, errors.New("missing 'max' param")
	}

	min := toInt64(minVal)
	max := toInt64(maxVal)

	if max <= min {
		return nil, fmt.Errorf("max (%d) must be greater than min (%d)", max, min)
	}

	return min + rng.Int63n(max-min), nil
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case float64:
		return int64(val)
	default:
		return 0
	}
}
