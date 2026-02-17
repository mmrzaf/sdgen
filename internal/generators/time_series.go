package generators

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/mmrzaf/sdgen/internal/domain"
	"github.com/mmrzaf/sdgen/internal/timeutil"
)

type TimeSeriesGenerator struct{}

func (g *TimeSeriesGenerator) Generate(rng *rand.Rand, ctx GeneratorContext) (interface{}, error) {
	return nil, errors.New("time_series generator requires params")
}

func (g *TimeSeriesGenerator) Validate(spec domain.GeneratorSpec, columnType domain.ColumnType) error {
	if spec.Params == nil {
		return errors.New("time_series requires 'start' and 'step' params")
	}
	_, hasStart := spec.Params["start"]
	_, hasStep := spec.Params["step"]
	if !hasStart || !hasStep {
		return errors.New("time_series requires 'start' and 'step' params")
	}
	return nil
}

func (g *TimeSeriesGenerator) GenerateWithParams(rng *rand.Rand, params map[string]interface{}, rowIndex int64) (interface{}, error) {
	startRaw, ok := params["start"]
	if !ok {
		return nil, errors.New("missing 'start' param")
	}
	stepRaw, ok := params["step"]
	if !ok {
		return nil, errors.New("missing 'step' param")
	}

	startStr, ok := startRaw.(string)
	if !ok {
		return nil, errors.New("'start' must be a string")
	}

	stepStr, ok := stepRaw.(string)
	if !ok {
		return nil, errors.New("'step' must be a string")
	}

	now := time.Now()
	startTime, err := timeutil.ParseRelativeTime(startStr, now)
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}

	stepDuration, err := timeutil.ParseDuration(stepStr)
	if err != nil {
		return nil, fmt.Errorf("invalid step duration: %w", err)
	}

	timestamp := startTime.Add(time.Duration(rowIndex) * stepDuration)

	if jitterRaw, hasJitter := params["jitter_seconds"]; hasJitter {
		jitterSeconds := toInt64(jitterRaw)
		if jitterSeconds > 0 {
			jitterRange := jitterSeconds * 2
			jitter := rng.Int63n(jitterRange) - jitterSeconds
			timestamp = timestamp.Add(time.Duration(jitter) * time.Second)
		}
	}

	return timestamp, nil
}
