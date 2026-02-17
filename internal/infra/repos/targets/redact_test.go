package targets

import "testing"

func TestRedactDSN(t *testing.T) {
	if got := RedactDSN(""); got != "" {
		t.Fatalf("expected empty DSN to stay empty, got %q", got)
	}
	if got := RedactDSN("/tmp/dev.sqlite"); got != "****" {
		t.Fatalf("expected sqlite path to be fully redacted, got %q", got)
	}
	if got := RedactDSN("postgres://user:pass@localhost:5432/db?sslmode=disable"); got == "" || got == "postgres://user:pass@localhost:5432/db?sslmode=disable" {
		t.Fatalf("expected URL DSN redaction, got %q", got)
	}
}
