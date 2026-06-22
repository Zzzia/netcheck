package report

import (
	"regexp"
	"strconv"
	"strings"
)

func parseCodexDuration(valueRaw, unit string) float64 {
	value, _ := strconv.ParseFloat(valueRaw, 64)
	switch unit {
	case "µs", "us":
		return value / 1_000_000
	case "ms":
		return value / 1000
	case "m":
		return value * 60
	default:
		return value
	}
}

func formatPercentRatio(numerator, denominator int) string {
	if denominator == 0 {
		return "0%"
	}
	return formatPercent(float64(numerator) / float64(denominator))
}

func formatCodexSeconds(seconds float64) string {
	if seconds <= 0 {
		return ""
	}
	return formatDuration(int64(seconds + 0.5))
}

func firstStringMatch(pattern *regexp.Regexp, raw, fallback string) string {
	match := pattern.FindStringSubmatch(raw)
	if match == nil {
		return fallback
	}
	return match[1]
}

func firstIntMatch(pattern *regexp.Regexp, raw string) int {
	value, _ := strconv.Atoi(firstStringMatch(pattern, raw, "0"))
	return value
}

func containsAny(raw string, items []string) bool {
	for _, item := range items {
		if strings.Contains(raw, item) {
			return true
		}
	}
	return false
}

func truncateText(raw string, limit int) string {
	if len(raw) <= limit {
		return raw
	}
	return raw[:limit] + "..."
}

func defaultString(raw, fallback string) string {
	if raw == "" {
		return fallback
	}
	return raw
}
