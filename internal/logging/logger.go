package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

type Logger struct {
	level     Level
	component string
	out       io.Writer
	mu        *sync.Mutex
}

func NewLogger(levelStr string) *Logger {
	return NewLoggerWithWriter(levelStr, os.Stdout)
}

func NewLoggerWithWriter(levelStr string, out io.Writer) *Logger {
	level := LevelInfo
	switch strings.ToLower(levelStr) {
	case "debug":
		level = LevelDebug
	case "info":
		level = LevelInfo
	case "warning", "warn":
		level = LevelWarn
	case "error":
		level = LevelError
	}
	if out == nil {
		out = os.Stdout
	}

	return &Logger{
		level: level,
		out:   out,
		mu:    &sync.Mutex{},
	}
}

func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level <= LevelDebug {
		l.emit("debug", formatMessage(format, args...), nil)
	}
}

func (l *Logger) Info(format string, args ...interface{}) {
	if l.level <= LevelInfo {
		l.emit("info", formatMessage(format, args...), nil)
	}
}

func (l *Logger) Warn(format string, args ...interface{}) {
	if l.level <= LevelWarn {
		l.emit("warn", formatMessage(format, args...), nil)
	}
}

func (l *Logger) Error(format string, args ...interface{}) {
	if l.level <= LevelError {
		l.emit("error", formatMessage(format, args...), nil)
	}
}

func (l *Logger) Debugw(msg string, fields map[string]any) {
	if l.level <= LevelDebug {
		l.emit("debug", msg, fields)
	}
}

func (l *Logger) Infow(msg string, fields map[string]any) {
	if l.level <= LevelInfo {
		l.emit("info", msg, fields)
	}
}

func (l *Logger) Warnw(msg string, fields map[string]any) {
	if l.level <= LevelWarn {
		l.emit("warn", msg, fields)
	}
}

func (l *Logger) Errorw(msg string, fields map[string]any) {
	if l.level <= LevelError {
		l.emit("error", msg, fields)
	}
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	l.emit("fatal", formatMessage(format, args...), nil)
	os.Exit(1)
}

func (l *Logger) Printf(format string, args ...interface{}) {
	l.emit("info", formatMessage(format, args...), nil)
}

func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		level:     l.level,
		component: strings.TrimSpace(component),
		out:       l.out,
		mu:        l.mu,
	}
}

func (l *Logger) emit(level, message string, fields map[string]any) {
	record := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": level,
		"msg":   message,
	}
	if l.component != "" {
		record["component"] = l.component
	}
	for k, v := range fields {
		record[k] = v
	}

	b, err := json.Marshal(record)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.out.Write(append(b, '\n'))
}

func formatMessage(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
