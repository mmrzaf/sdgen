package generators

import (
	"errors"
	"math/rand"

	"github.com/mmrzaf/sdgen/internal/domain"
)

type ConstGenerator struct{}

func (g *ConstGenerator) Generate(rng *rand.Rand, ctx GeneratorContext) (interface{}, error) {
	return nil, errors.New("const generator requires params at validation time")
}

func (g *ConstGenerator) Validate(spec domain.GeneratorSpec, columnType domain.ColumnType) error {
	if spec.Params == nil {
		return errors.New("const generator requires 'value' param")
	}
	_, ok := spec.Params["value"]
	if !ok {
		return errors.New("const generator requires 'value' param")
	}
	return nil
}

func (g *ConstGenerator) GenerateValue(spec domain.GeneratorSpec) interface{} {
	return spec.Params["value"]
}
