package hashing

import (
	"testing"

	"github.com/mmrzaf/sdgen/internal/domain"
)

func TestHashRunConfig_IncludesModeSeedAndResolvedCounts(t *testing.T) {
	sc := &domain.Scenario{
		ID:      "s1",
		Name:    "scenario",
		Version: "1.0.0",
		Entities: []domain.Entity{
			{
				Name:        "users",
				TargetTable: "users",
				Rows:        10,
				Columns: []domain.Column{
					{Name: "id", Type: domain.ColumnTypeInt, Generator: domain.GeneratorSpec{Type: "uniform_int", Params: map[string]interface{}{"min": 1, "max": 10}}},
				},
			},
		},
	}
	tg := &domain.TargetConfig{Kind: "postgres", DSN: "postgres://localhost:5432/app?sslmode=disable"}

	h1, err := HashRunConfig(sc, tg, "create", 1.0, map[string]int64{"users": 10}, 11)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HashRunConfig(sc, tg, "truncate", 1.0, map[string]int64{"users": 10}, 11)
	if err != nil {
		t.Fatal(err)
	}
	h3, err := HashRunConfig(sc, tg, "create", 1.0, map[string]int64{"users": 20}, 11)
	if err != nil {
		t.Fatal(err)
	}
	h4, err := HashRunConfig(sc, tg, "create", 1.0, map[string]int64{"users": 10}, 12)
	if err != nil {
		t.Fatal(err)
	}

	if h1 == h2 {
		t.Fatal("expected mode to affect hash")
	}
	if h1 == h3 {
		t.Fatal("expected resolved counts to affect hash")
	}
	if h1 == h4 {
		t.Fatal("expected seed to affect hash")
	}
}
