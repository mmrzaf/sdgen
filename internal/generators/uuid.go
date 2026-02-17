package generators

import (
	"math/rand"

	"github.com/google/uuid"
	"github.com/mmrzaf/sdgen/internal/domain"
)

type UUID4Generator struct{}

func (g *UUID4Generator) Generate(rng *rand.Rand, ctx GeneratorContext) (interface{}, error) {
	uuidBytes := make([]byte, 16)
	rng.Read(uuidBytes)
	uuidBytes[6] = (uuidBytes[6] & 0x0f) | 0x40
	uuidBytes[8] = (uuidBytes[8] & 0x3f) | 0x80
	u, err := uuid.FromBytes(uuidBytes)
	if err != nil {
		return nil, err
	}
	return u.String(), nil
}

func (g *UUID4Generator) Validate(spec domain.GeneratorSpec, columnType domain.ColumnType) error {
	return nil
}
