package registry

import (
	"fmt"
	"sync"

	"github.com/mmrzaf/sdgen/internal/generators"
)

type GeneratorRegistry struct {
	mu         sync.RWMutex
	generators map[string]generators.Generator
}

func NewGeneratorRegistry() *GeneratorRegistry {
	return &GeneratorRegistry{
		generators: make(map[string]generators.Generator),
	}
}

func (r *GeneratorRegistry) Register(name string, gen generators.Generator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.generators[name] = gen
}

func (r *GeneratorRegistry) Get(name string) (generators.Generator, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	gen, ok := r.generators[name]
	if !ok {
		return nil, fmt.Errorf("generator not found: %s", name)
	}
	return gen, nil
}

func (r *GeneratorRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.generators))
	for name := range r.generators {
		names = append(names, name)
	}
	return names
}

func DefaultGeneratorRegistry() *GeneratorRegistry {
	r := NewGeneratorRegistry()
	r.Register("const", &generators.ConstGenerator{})
	r.Register("uuid4", &generators.UUID4Generator{})
	r.Register("uniform_int", &generators.UniformIntGenerator{})
	r.Register("uniform_float", &generators.UniformFloatGenerator{})
	r.Register("normal", &generators.NormalGenerator{})
	r.Register("choice", &generators.ChoiceGenerator{})
	r.Register("faker_name", &generators.FakerNameGenerator{})
	r.Register("faker_city", &generators.FakerCityGenerator{})
	r.Register("faker_device_name", &generators.FakerDeviceNameGenerator{})
	r.Register("time_series", &generators.TimeSeriesGenerator{})
	r.Register("fk", &generators.FKGenerator{})
	return r
}
