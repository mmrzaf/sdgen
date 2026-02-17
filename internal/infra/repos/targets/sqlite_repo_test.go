package targets

import (
	"os"
	"testing"

	"github.com/mmrzaf/sdgen/internal/domain"
	"github.com/mmrzaf/sdgen/internal/infra/repos/runs"
)

func TestSQLiteTargetsCRUD(t *testing.T) {
	f, err := os.CreateTemp("", "sdgen_runs_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	_ = f.Close()

	runRepo := runs.NewSQLiteRepository(f.Name())
	if err := runRepo.Init(); err != nil {
		t.Fatal(err)
	}
	repo := NewSQLiteRepository(runRepo.DB())

	tgt := &domain.TargetConfig{
		Name:     "t1",
		Kind:     "postgres",
		DSN:      "postgres://u:p@localhost:5432/postgres?sslmode=disable",
		Database: "appdb",
		Schema:   "public",
	}
	if err := repo.Create(tgt); err != nil {
		t.Fatal(err)
	}
	if tgt.ID == "" {
		t.Fatal("expected id")
	}

	got, err := repo.Get(tgt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "t1" || got.Kind != "postgres" {
		t.Fatalf("unexpected: %#v", got)
	}
	if got.DSN != "postgres://u:p@localhost:5432/postgres?sslmode=disable" {
		t.Fatalf("expected raw DSN in DB-backed read, got %q", got.DSN)
	}
	if got.Database != "appdb" {
		t.Fatalf("expected database field, got %#v", got)
	}
	if RedactTarget(got).DSN == got.DSN {
		t.Fatalf("expected redacted DSN to differ from raw")
	}

	got.Name = "t1b"
	if err := repo.Update(got); err != nil {
		t.Fatal(err)
	}

	list, err := repo.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "t1b" {
		t.Fatalf("unexpected list: %#v", list)
	}

	if err := repo.Delete(tgt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(tgt.ID); err == nil {
		t.Fatal("expected not found")
	}
}
