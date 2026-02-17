package generators

import (
	"math/rand"

	"github.com/mmrzaf/sdgen/internal/domain"
)

type Generator interface {
	Generate(rng *rand.Rand, ctx GeneratorContext) (interface{}, error)
	Validate(spec domain.GeneratorSpec, columnType domain.ColumnType) error
}

type GeneratorContext struct {
	RowIndex     int64
	EntityValues map[string][]interface{}
}
