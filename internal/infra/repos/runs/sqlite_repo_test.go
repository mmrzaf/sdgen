package runs

import (
	"path/filepath"
	"testing"
)

func TestInitCreatesParentDirectory(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "nested", "deeper", "runs.db")
	repo := NewSQLiteRepository(dbPath)

	if err := repo.Init(); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if repo.DB() == nil {
		t.Fatal("expected db handle to be initialized")
	}
	t.Cleanup(func() {
		_ = repo.DB().Close()
	})
}

