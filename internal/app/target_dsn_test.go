package app

import (
	"strings"
	"testing"

	"github.com/mmrzaf/sdgen/internal/domain"
)

func TestResolveTargetForRun_PostgresDatabaseOverride(t *testing.T) {
	base := &domain.TargetConfig{
		Name:     "pg",
		Kind:     "postgres",
		DSN:      "postgres://user:pass@localhost:5432/postgres?sslmode=disable",
		Database: "postgres",
	}
	got := resolveTargetForRun(base, "tenant_a")
	if got.Database != "tenant_a" {
		t.Fatalf("expected database override, got %#v", got)
	}
	if !strings.Contains(got.DSN, "/tenant_a") {
		t.Fatalf("expected overridden db in dsn, got %q", got.DSN)
	}
}
