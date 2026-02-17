package validation

import (
	"testing"

	"github.com/mmrzaf/sdgen/internal/domain"
	"github.com/mmrzaf/sdgen/internal/registry"
)

func TestValidateRunRequest_RequiresMode(t *testing.T) {
	v := NewValidator(registry.DefaultGeneratorRegistry())
	req := &domain.RunRequest{
		ScenarioID: "s1",
		TargetID:   "t1",
	}
	if err := v.ValidateRunRequest(req); err == nil {
		t.Fatal("expected mode validation error")
	}
}

func TestValidateRunRequest_CustomizationFields(t *testing.T) {
	v := NewValidator(registry.DefaultGeneratorRegistry())
	scale := 1.2
	req := &domain.RunRequest{
		ScenarioID: "s1",
		TargetID:   "t1",
		Mode:       "create",
		Scale:      &scale,
		EntityScales: map[string]float64{
			"users":  2.0,
			"events": 0.5,
		},
		EntityCounts: map[string]int64{
			"users": 100,
		},
		IncludeEntities: []string{"users", "events"},
		ExcludeEntities: []string{"events_archive"},
		TargetDatabase:  "tenant_a",
	}
	if err := v.ValidateRunRequest(req); err != nil {
		t.Fatalf("expected valid request, got %v", err)
	}
}

func TestValidateRunRequest_RejectsInvalidCustomizationFields(t *testing.T) {
	v := NewValidator(registry.DefaultGeneratorRegistry())
	scale := 1.0
	req := &domain.RunRequest{
		ScenarioID: "s1",
		TargetID:   "t1",
		Mode:       "create",
		Scale:      &scale,
		EntityScales: map[string]float64{
			"bad-name": 1.2,
		},
	}
	if err := v.ValidateRunRequest(req); err == nil {
		t.Fatal("expected invalid entity_scales key error")
	}

	req2 := &domain.RunRequest{
		ScenarioID:     "s1",
		TargetID:       "t1",
		Mode:           "create",
		TargetDatabase: "bad-name",
	}
	if err := v.ValidateRunRequest(req2); err == nil {
		t.Fatal("expected invalid target_database error")
	}
}

func TestValidateTarget_NewKinds(t *testing.T) {
	v := NewValidator(registry.DefaultGeneratorRegistry())
	es := &domain.TargetConfig{Name: "e1", Kind: "elasticsearch", DSN: "http://localhost:9200"}
	if err := v.ValidateTarget(es); err != nil {
		t.Fatalf("expected elasticsearch target valid, got %v", err)
	}

	sqliteBad := &domain.TargetConfig{Name: "s1", Kind: "sqlite", DSN: "/tmp/x.db", Database: "not_allowed"}
	if err := v.ValidateTarget(sqliteBad); err == nil {
		t.Fatal("expected sqlite database field to be rejected")
	}
}
