package hashing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/mmrzaf/sdgen/internal/domain"
)

type runConfigHashPayload struct {
	ScenarioHash   string           `json:"scenario_hash"`
	TargetKind     string           `json:"target_kind"`
	TargetSchema   string           `json:"target_schema,omitempty"`
	TargetDSN      string           `json:"target_dsn"`
	Mode           string           `json:"mode"`
	Scale          float64          `json:"scale"`
	ResolvedCounts map[string]int64 `json:"resolved_counts"`
	Seed           int64            `json:"seed"`
}

func HashRunConfig(scenario *domain.Scenario, target *domain.TargetConfig, mode string, scale float64, resolvedCounts map[string]int64, seed int64) (string, error) {
	sh, err := HashScenario(scenario)
	if err != nil {
		return "", err
	}

	canon := make(map[string]int64, len(resolvedCounts))
	if resolvedCounts != nil {
		keys := make([]string, 0, len(resolvedCounts))
		for k := range resolvedCounts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			canon[k] = resolvedCounts[k]
		}
	}

	p := runConfigHashPayload{
		ScenarioHash:   sh,
		TargetKind:     target.Kind,
		TargetSchema:   target.Schema,
		TargetDSN:      target.DSN,
		Mode:           mode,
		Scale:          scale,
		ResolvedCounts: canon,
		Seed:           seed,
	}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
