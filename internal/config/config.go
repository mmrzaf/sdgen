package config

import (
	"os"
	"strconv"
)

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		n, err := strconv.Atoi(val)
		if err == nil {
			return n
		}
	}
	return defaultVal
}

type Config struct {
	ScenariosDir string
	RunsDBPath   string
	BindAddr     string
	LogLevel     string
	BatchSize    int
}

func Load() *Config {
	return &Config{
		ScenariosDir: getEnv("SDGEN_SCENARIOS_DIR", "./scenarios"),
		RunsDBPath:   getEnv("SDGEN_RUNS_DB", "./runs.db"),
		BindAddr:     getEnv("SDGEN_BIND", "127.0.0.1:8080"),
		LogLevel:     getEnv("SDGEN_LOG_LEVEL", "info"),
		BatchSize:    getEnvInt("SDGEN_BATCH_SIZE", 1000),
	}
}
