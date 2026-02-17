package timeutil

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, errors.New("empty duration string")
	}

	if dur, err := time.ParseDuration(s); err == nil {
		return dur, nil
	}

	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration format: %s", s)
	}

	numStr := s[:len(s)-1]
	unit := s[len(s)-1:]

	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration number: %s", numStr)
	}

	switch unit {
	case "d":
		return time.Duration(num) * 24 * time.Hour, nil
	case "w":
		return time.Duration(num) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %s", unit)
	}
}

func ParseRelativeTime(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("empty time string")
	}

	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	if !strings.HasPrefix(s, "-") && !strings.HasPrefix(s, "+") {
		return time.Time{}, fmt.Errorf("relative time must start with + or -: %s", s)
	}

	isNegative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	s = strings.TrimPrefix(s, "+")

	dur, err := ParseDuration(s)
	if err != nil {
		return time.Time{}, err
	}

	if isNegative {
		return now.Add(-dur), nil
	}
	return now.Add(dur), nil
}
