package cli

import (
	"fmt"
	"strings"
	"time"

	"walkline/internal/store"
)

func BuildDateFilter(since, until, at string) (store.CommitFilter, error) {
	count := 0
	if since != "" {
		count++
	}
	if until != "" {
		count++
	}
	if at != "" {
		count++
	}
	if count > 1 {
		return store.CommitFilter{}, fmt.Errorf("--since, --until, and --at are mutually exclusive, use only one")
	}

	if since != "" {
		ts, err := normalizeSince(since)
		if err != nil {
			return store.CommitFilter{}, fmt.Errorf("--since: %w", err)
		}
		return store.CommitFilter{Since: ts}, nil
	}
	if until != "" {
		ts, err := normalizeUntil(until)
		if err != nil {
			return store.CommitFilter{}, fmt.Errorf("--until: %w", err)
		}
		return store.CommitFilter{Until: ts}, nil
	}
	if at != "" {
		start, end, err := normalizeAt(at)
		if err != nil {
			return store.CommitFilter{}, fmt.Errorf("--at: %w", err)
		}
		return store.CommitFilter{Since: start, Until: end}, nil
	}
	return store.CommitFilter{}, nil
}

func normalizeSince(input string) (string, error) {
	if isBareDate(input) {
		return input + "T00:00:00Z", nil
	}
	t, err := time.Parse(time.RFC3339, input)
	if err != nil {
		return "", fmt.Errorf("invalid date %q: use YYYY-MM-DD or RFC3339 (e.g. 2024-01-15T00:00:00Z)", input)
	}
	return t.Format(time.RFC3339), nil
}

func normalizeUntil(input string) (string, error) {
	if isBareDate(input) {
		return input + "T23:59:59Z", nil
	}
	t, err := time.Parse(time.RFC3339, input)
	if err != nil {
		return "", fmt.Errorf("invalid date %q: use YYYY-MM-DD or RFC3339 (e.g. 2024-01-15T23:59:59Z)", input)
	}
	return t.Format(time.RFC3339), nil
}

func normalizeAt(input string) (start, end string, err error) {
	if isBareDate(input) {
		return input + "T00:00:00Z", input + "T23:59:59Z", nil
	}
	t, err := time.Parse(time.RFC3339, input)
	if err != nil {
		return "", "", fmt.Errorf("invalid date %q: use YYYY-MM-DD or RFC3339 (e.g. 2024-01-15T00:00:00Z)", input)
	}
	day := t.Format("2006-01-02")
	return day + "T00:00:00Z", day + "T23:59:59Z", nil
}

func isBareDate(s string) bool {
	return len(s) == 10 && strings.Count(s, "-") == 2
}
