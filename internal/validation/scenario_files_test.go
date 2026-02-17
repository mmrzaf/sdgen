package validation

import (
	"testing"

	"github.com/mmrzaf/sdgen/internal/infra/repos/scenarios"
	"github.com/mmrzaf/sdgen/internal/registry"
)

func TestRepositoryScenariosValidate(t *testing.T) {
	repo := scenarios.NewFileRepository("../../scenarios")
	list, err := repo.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("expected scenario files")
	}

	v := NewValidator(registry.DefaultGeneratorRegistry())
	for _, sc := range list {
		if err := v.ValidateScenario(sc); err != nil {
			t.Fatalf("scenario %q failed validation: %v", sc.ID, err)
		}
	}
}
