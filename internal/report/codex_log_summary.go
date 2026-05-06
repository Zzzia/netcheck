package report

import (
	"sort"
	"strings"
)

func isNetworkCodexKind(kind string) bool {
	return kind == "stream_retry" || kind == "network_candidate"
}

func (r *codexParseResult) addOtherEvent(item codexParsedLine, class codexLineClass) {
	key := class.kind + "|" + item.level + "|" + codexOtherName(item, class)
	row := r.other[key]
	if row == nil {
		row = &codexIssueRow{
			Kind:   class.kind,
			Level:  item.level,
			Name:   codexOtherName(item, class),
			Sample: item.message,
		}
		r.other[key] = row
	}
	row.Count++
}

func (r *codexParseResult) otherRows() []codexIssueRow {
	rows := make([]codexIssueRow, 0, len(r.other))
	for _, row := range r.other {
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

func codexOtherName(item codexParsedLine, class codexLineClass) string {
	switch class.kind {
	case "unknown_warning", "unknown_error":
		return compactUnknownCodexMessage(item.message)
	default:
		return class.label
	}
}

func compactUnknownCodexMessage(message string) string {
	message = skippedPattern.ReplaceAllString(message, "skipped=<n>")
	if index := strings.Index(message, " error="); index > 0 {
		message = message[:index]
	}
	return truncateText(message, 120)
}
