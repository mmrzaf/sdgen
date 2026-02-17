package generators

import (
	"math/rand"

	"github.com/go-faker/faker/v4"
	"github.com/mmrzaf/sdgen/internal/domain"
)

type FakerNameGenerator struct{}

func (g *FakerNameGenerator) Generate(rng *rand.Rand, ctx GeneratorContext) (interface{}, error) {
	return faker.Name(), nil
}

func (g *FakerNameGenerator) Validate(spec domain.GeneratorSpec, columnType domain.ColumnType) error {
	return nil
}

type FakerCityGenerator struct{}

func (g *FakerCityGenerator) Generate(rng *rand.Rand, ctx GeneratorContext) (interface{}, error) {
	cities := []string{
		"New York", "Los Angeles", "Chicago", "Houston", "Phoenix",
		"Philadelphia", "San Antonio", "San Diego", "Dallas", "San Jose",
		"Austin", "Jacksonville", "Fort Worth", "Columbus", "Charlotte",
		"San Francisco", "Indianapolis", "Seattle", "Denver", "Washington",
		"Boston", "Nashville", "Detroit", "Portland", "Las Vegas",
		"London", "Paris", "Tokyo", "Berlin", "Madrid",
		"Rome", "Amsterdam", "Vienna", "Prague", "Barcelona",
		"Munich", "Milan", "Stockholm", "Copenhagen", "Oslo",
	}
	return cities[rng.Intn(len(cities))], nil
}

func (g *FakerCityGenerator) Validate(spec domain.GeneratorSpec, columnType domain.ColumnType) error {
	return nil
}

type FakerDeviceNameGenerator struct{}

func (g *FakerDeviceNameGenerator) Generate(rng *rand.Rand, ctx GeneratorContext) (interface{}, error) {
	prefixes := []string{"Sensor", "Device", "Meter", "Gauge", "Monitor", "Detector", "Reader", "Tracker"}
	suffixes := []string{"Alpha", "Beta", "Gamma", "Delta", "Prime", "Pro", "Max", "Plus"}

	prefix := prefixes[rng.Intn(len(prefixes))]
	suffix := suffixes[rng.Intn(len(suffixes))]
	number := rng.Intn(9999)

	return faker.Username() + "-" + prefix + "-" + suffix + "-" + faker.Word() + "-" + string(rune('0'+number%10)), nil
}

func (g *FakerDeviceNameGenerator) Validate(spec domain.GeneratorSpec, columnType domain.ColumnType) error {
	return nil
}
