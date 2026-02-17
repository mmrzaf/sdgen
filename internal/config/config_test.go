package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ReadsDotEnvForSDGENDB(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(cwd) }()

	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, ".env"), []byte("SDGEN_DB=postgres://u:p@localhost:5432/sdgen?sslmode=disable\nSDGEN_LOG_LEVEL=debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}

	oldDB, hadDB := os.LookupEnv("SDGEN_DB")
	oldLog, hadLog := os.LookupEnv("SDGEN_LOG_LEVEL")
	_ = os.Unsetenv("SDGEN_DB")
	_ = os.Unsetenv("SDGEN_LOG_LEVEL")
	t.Cleanup(func() {
		if hadDB {
			_ = os.Setenv("SDGEN_DB", oldDB)
		} else {
			_ = os.Unsetenv("SDGEN_DB")
		}
		if hadLog {
			_ = os.Setenv("SDGEN_LOG_LEVEL", oldLog)
		} else {
			_ = os.Unsetenv("SDGEN_LOG_LEVEL")
		}
	})

	cfg := Load()
	if cfg.SDGenDBDSN != "postgres://u:p@localhost:5432/sdgen?sslmode=disable" {
		t.Fatalf("expected SDGEN_DB from .env, got %q", cfg.SDGenDBDSN)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("expected SDGEN_LOG_LEVEL from .env, got %q", cfg.LogLevel)
	}
}
