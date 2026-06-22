package report

import (
	"sort"
	"strings"

	"netcheck/internal/i18n"
)

func isNetworkCodexKind(kind string) bool {
	return kind == "stream_retry" || kind == "network_candidate"
}

func (r *codexParseResult) addOtherEvent(item codexParsedLine, class codexLineClass) {
	nameKey := codexOtherNameKey(item, class)
	key := class.kind + "|" + item.level + "|" + nameKey
	row := r.other[key]
	if row == nil {
		row = &codexIssueRow{
			Kind:    class.kind,
			Level:   item.level,
			NameKey: nameKey,
			Sample:  item.message,
		}
		r.other[key] = row
	}
	row.Count++
}

func (r *codexParseResult) otherRows(localizer i18n.Localizer) []codexIssueRow {
	rows := make([]codexIssueRow, 0, len(r.other))
	for _, row := range r.other {
		row.Name = codexOtherName(row.NameKey, localizer)
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func codexOtherNameKey(item codexParsedLine, class codexLineClass) string {
	switch class.kind {
	case "unknown_warning", "unknown_error":
		return compactUnknownCodexMessage(item.message)
	default:
		return class.labelKey
	}
}

func codexOtherName(key string, localizer i18n.Localizer) string {
	if strings.HasPrefix(key, "codex.") {
		return localizer.T(key)
	}
	return key
}

func compactUnknownCodexMessage(message string) string {
	message = skippedPattern.ReplaceAllString(message, "skipped=<n>")
	if index := strings.Index(message, " error="); index > 0 {
		message = message[:index]
	}
	return truncateText(message, 120)
}
