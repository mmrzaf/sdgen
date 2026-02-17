package scenarios

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetByPath_RejectsPathTraversal(t *testing.T) {
	base := t.TempDir()
	repo := NewFileRepository(base)

	inside := filepath.Join(base, "ok.yaml")
	if err := os.WriteFile(inside, []byte("id: ok\nname: ok\nentities:\n  - name: e\n    target_table: e\n    rows: 1\n    columns:\n      - name: id\n        type: int\n        generator:\n          type: uniform_int\n          params:\n            min: 1\n            max: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByPath("ok.yaml"); err != nil {
		t.Fatalf("expected scenario load inside base dir, got %v", err)
	}

	outsideFile := filepath.Join(t.TempDir(), "outside.yaml")
	if err := os.WriteFile(outsideFile, []byte("id: bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByPath(outsideFile); err == nil {
		t.Fatal("expected traversal rejection for outside absolute path")
	}
	if _, err := repo.GetByPath("../outside.yaml"); err == nil {
		t.Fatal("expected traversal rejection for relative path escape")
	}
}
