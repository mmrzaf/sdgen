package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLoggerStructuredOutput(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoggerWithWriter("debug", &buf).WithComponent("test")
	l.Infow("event.happened", map[string]any{"run_id": "r1", "count": 2})

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected log output")
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("expected JSON log line: %v", err)
	}
	if rec["level"] != "info" {
		t.Fatalf("unexpected level: %#v", rec["level"])
	}
	if rec["msg"] != "event.happened" {
		t.Fatalf("unexpected msg: %#v", rec["msg"])
	}
	if rec["component"] != "test" {
		t.Fatalf("unexpected component: %#v", rec["component"])
	}
	if rec["run_id"] != "r1" {
		t.Fatalf("unexpected field run_id: %#v", rec["run_id"])
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoggerWithWriter("error", &buf)
	l.Info("should_not_log")
	l.Error("should_log")
	out := strings.TrimSpace(buf.String())
	lines := strings.Split(out, "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one line, got %d: %q", len(lines), out)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("expected JSON log line: %v", err)
	}
	if rec["level"] != "error" {
		t.Fatalf("unexpected level: %#v", rec["level"])
	}
}
