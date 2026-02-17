package targets

import (
	"net/url"
	"strings"

	"github.com/mmrzaf/sdgen/internal/domain"
)

func RedactDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return ""
	}

	// URL form (postgres://user:pass@host/db?password=...)
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" && u.Host != "" {
		if u.User != nil {
			user := u.User.Username()
			u.User = url.UserPassword(user, "****")
		}
		q := u.Query()
		for _, k := range []string{"password", "pass", "pwd"} {
			if q.Has(k) {
				q.Set(k, "****")
			}
		}
		u.RawQuery = q.Encode()
		return u.String()
	}

	// Keyword form: host=... user=... password=...
	parts := strings.Fields(dsn)
	redacted := false
	for i := range parts {
		l := strings.ToLower(parts[i])
		if strings.HasPrefix(l, "password=") || strings.HasPrefix(l, "pwd=") || strings.HasPrefix(l, "pass=") {
			k := parts[i][:strings.IndexByte(parts[i], '=')+1]
			parts[i] = k + "****"
			redacted = true
		}
	}
	if redacted {
		return strings.Join(parts, " ")
	}

	return "****"
}

func RedactTarget(t *domain.TargetConfig) *domain.TargetConfig {
	if t == nil {
		return nil
	}
	cp := *t
	cp.DSN = RedactDSN(cp.DSN)
	return &cp
}

func RedactTargets(list []*domain.TargetConfig) []*domain.TargetConfig {
	out := make([]*domain.TargetConfig, 0, len(list))
	for _, t := range list {
		out = append(out, RedactTarget(t))
	}
	return out
}
