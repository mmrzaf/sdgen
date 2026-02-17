package hashing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/mmrzaf/sdgen/internal/domain"
)

func HashScenario(scenario *domain.Scenario) (string, error) {
	canonical := canonicalizeScenario(scenario)
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func canonicalizeScenario(scenario *domain.Scenario) map[string]interface{} {
	entities := make([]map[string]interface{}, len(scenario.Entities))
	for i, entity := range scenario.Entities {
		columns := make([]map[string]interface{}, len(entity.Columns))
		for j, col := range entity.Columns {
			colMap := map[string]interface{}{
				"name":      col.Name,
				"type":      col.Type,
				"nullable":  col.Nullable,
				"generator": canonicalizeGeneratorSpec(col.Generator),
			}
			if col.FK != nil {
				colMap["fk"] = map[string]interface{}{
					"entity": col.FK.Entity,
					"column": col.FK.Column,
				}
			}
			columns[j] = colMap
		}

		entities[i] = map[string]interface{}{
			"name":         entity.Name,
			"target_table": entity.TargetTable,
			"rows":         entity.Rows,
			"columns":      columns,
		}
	}

	result := map[string]interface{}{
		"name":     scenario.Name,
		"entities": entities,
	}
	if scenario.ID != "" {
		result["id"] = scenario.ID
	}
	if scenario.Version != "" {
		result["version"] = scenario.Version
	}
	if scenario.Description != "" {
		result["description"] = scenario.Description
	}

	return result
}

func canonicalizeGeneratorSpec(spec domain.GeneratorSpec) map[string]interface{} {
	result := map[string]interface{}{
		"type": spec.Type,
	}
	if spec.Params != nil && len(spec.Params) > 0 {
		result["params"] = canonicalizeParams(spec.Params)
	}
	return result
}

func canonicalizeParams(params map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := params[k]
		switch val := v.(type) {
		case map[string]interface{}:
			result[k] = canonicalizeParams(val)
		case []interface{}:
			result[k] = val
		default:
			result[k] = val
		}
	}
	return result
}
