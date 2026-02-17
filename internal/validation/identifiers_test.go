package validation

import "testing"

func TestIsValidIdentifier(t *testing.T) {
	ok := []string{"a", "A", "_a", "a1", "a_b2", "snake_case_123"}
	bad := []string{"", "1a", "a-b", "a b", "a;b", "a\"b", "a.b", "a/b", "a--", "select", "from", "order", "table", "group", "user", "returning"}

	for _, s := range ok {
		if !IsValidIdentifier(s) {
			t.Fatalf("expected valid: %q", s)
		}
	}
	for _, s := range bad {
		if IsValidIdentifier(s) {
			t.Fatalf("expected invalid: %q", s)
		}
	}
}

func TestIsValidMode(t *testing.T) {
	if !IsValidMode("create") || !IsValidMode("truncate") || !IsValidMode("append") {
		t.Fatal("expected valid table modes")
	}
	if IsValidMode("") || IsValidMode("create_if_missing") || IsValidMode("foo") {
		t.Fatal("expected invalid legacy or unknown mode")
	}
}
