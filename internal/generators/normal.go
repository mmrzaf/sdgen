package generators

import (
	"errors"
	"math/rand"

	"github.com/mmrzaf/sdgen/internal/domain"
)

type NormalGenerator struct{}

func (g *NormalGenerator) Generate(rng *rand.Rand, ctx GeneratorContext) (interface{}, error) {
	return nil, errors.New("normal generator requires params")
}

func (g *NormalGenerator) Validate(spec domain.GeneratorSpec, columnType domain.ColumnType) error {
	if spec.Params == nil {
		return errors.New("normal requires 'mean' and 'std' params")
	}
	_, hasMean := spec.Params["mean"]
	_, hasStd := spec.Params["std"]
	if !hasMean || !hasStd {
		return errors.New("normal requires 'mean' and 'std' params")
	}
	return nil
}

func (g *NormalGenerator) GenerateWithParams(rng *rand.Rand, params map[string]interface{}) (interface{}, error) {
	meanVal, ok := params["mean"]
	if !ok {
		return nil, errors.New("missing 'mean' param")
	}
	stdVal, ok := params["std"]
	if !ok {
		return nil, errors.New("missing 'std' param")
	}

	mean := toFloat64(meanVal)
	std := toFloat64(stdVal)

	return rng.NormFloat64()*std + mean, nil
}
