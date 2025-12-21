package config

import (
	"os"
)

type Config struct {
	ScenariosDir string
	TargetsDir   string
	RunsDBPath   string
	LogLevel     string
	BindAddr     string
	DefaultMode  string
}

func Load() *Config {
	return &Config{
		ScenariosDir: getEnv("SDGEN_SCENARIOS_DIR", "./scenarios"),
		TargetsDir:   getEnv("SDGEN_TARGETS_DIR", "./targets"),
		RunsDBPath:   getEnv("SDGEN_RUNS_DB", "./sdgen-runs.sqlite"),
		LogLevel:     getEnv("SDGEN_LOG_LEVEL", "info"),
		BindAddr:     getEnv("SDGEN_BIND_ADDR", ":8080"),
		DefaultMode:  getEnv("SDGEN_DEFAULT_MODE", "create_if_missing"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
