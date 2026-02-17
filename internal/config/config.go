package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	SDGenDBDSN   string
	BindAddr     string
	LogLevel     string
	BatchSize    int
}

func Load() *Config {
	loadDotEnv(".env")

	return &Config{
		ScenariosDir: getEnv("SDGEN_SCENARIOS_DIR", "./scenarios"),
		SDGenDBDSN:   getEnv("SDGEN_DB", ""),
		BindAddr:     getEnv("SDGEN_BIND", "127.0.0.1:8080"),
		LogLevel:     getEnv("SDGEN_LOG_LEVEL", "info"),
		BatchSize:    getEnvInt("SDGEN_BATCH_SIZE", 1000),
	}
}

func loadDotEnv(path string) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		val := strings.TrimSpace(v)
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, val)
	}
}
