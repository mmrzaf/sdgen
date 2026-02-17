package app

import (
	"net/url"
	"strings"

	"github.com/mmrzaf/sdgen/internal/domain"
)

func resolveTargetForRun(base *domain.TargetConfig, dbOverride string) *domain.TargetConfig {
	if base == nil {
		return nil
	}
	t := *base
	if dbOverride != "" {
		t.Database = dbOverride
	}
	switch t.Kind {
	case "postgres":
		if t.Database != "" {
			t.DSN = withPostgresDatabase(t.DSN, t.Database)
		}
	}
	return &t
}

func withPostgresDatabase(dsn, database string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return dsn
	}
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" && u.Host != "" {
		u.Path = "/" + database
		return u.String()
	}
	parts := strings.Fields(dsn)
	found := false
	for i := range parts {
		if strings.HasPrefix(strings.ToLower(parts[i]), "dbname=") {
			parts[i] = "dbname=" + database
			found = true
			break
		}
	}
	if !found {
		parts = append(parts, "dbname="+database)
	}
	return strings.Join(parts, " ")
}
