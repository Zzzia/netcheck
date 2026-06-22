package report

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"netcheck/internal/i18n"
	"netcheck/internal/model"
)

var (
	englishLocalSummaryRE  = regexp.MustCompile(`^gateway avg=([0-9.]+)ms p95=([0-9.]+)ms jitter=([0-9.]+)ms loss=([0-9.]+)%$`)
	chineseLocalSummaryRE  = regexp.MustCompile(`^网关 avg=([0-9.]+)ms p95=([0-9.]+)ms jitter=([0-9.]+)ms loss=([0-9.]+)%$`)
	englishRemoteSummaryRE = regexp.MustCompile(`^(Domestic|International) latency avg=([0-9.]+)ms failure=([0-9.]+)% dl=([0-9.]+)Mbps$`)
	chineseRemoteSummaryRE = regexp.MustCompile(`^(国内|国外) 延迟 avg=([0-9.]+)ms 失败率=([0-9.]+)% dl=([0-9.]+)Mbps$`)
)

func buildCauseRows(events []model.Event, start, end time.Time) []causeRow {
	return buildCauseRowsForLang(events, start, end, i18n.New(i18n.English))
}

func buildCauseRowsForLang(events []model.Event, start, end time.Time, localizer i18n.Localizer) []causeRow {
	intervals := map[string][]timeInterval{}
	for _, item := range events {
		if item.Name != "local" && item.Name != "domestic" && item.Name != "international" {
			continue
		}
		interval, ok := clippedInterval(item, start, end)
		if !ok {
			continue
		}
		intervals[item.Name] = append(intervals[item.Name], interval)
	}
	var rows []causeRow
	for _, name := range []string{"local", "domestic", "international"} {
		duration := mergedDuration(intervals[name])
		if duration == 0 {
			continue
		}
		rows = append(rows, causeRow{
			Name:        causeName(name, localizer),
			NameKey:     causeNameKey(name),
			DurationSec: duration,
			Duration:    formatDuration(duration),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].DurationSec > rows[j].DurationSec })
	return rows
}

func buildEventRows(events []model.Event, start, end time.Time) []eventRow {
	return buildEventRowsForLang(events, start, end, i18n.New(i18n.English))
}

func buildEventRowsForLang(events []model.Event, start, end time.Time, localizer i18n.Localizer) []eventRow {
	type visibleEvent struct {
		item     model.Event
		interval timeInterval
	}
	var visible []visibleEvent
	for _, item := range events {
		if item.Name != "local" && item.Name != "domestic" && item.Name != "international" {
			continue
		}
		interval, ok := clippedInterval(item, start, end)
		if !ok || !interval.end.After(interval.start) {
			continue
		}
		visible = append(visible, visibleEvent{item: item, interval: interval})
	}
	sort.Slice(visible, func(i, j int) bool {
		if visible[i].interval.end.Equal(visible[j].interval.end) {
			return visible[i].interval.start.After(visible[j].interval.start)
		}
		return visible[i].interval.end.After(visible[j].interval.end)
	})
	if len(visible) > 10 {
		visible = visible[:10]
	}

	var rows []eventRow
	for _, visibleItem := range visible {
		item := visibleItem.item
		summary := localizeMonitorSummary(item.Summary, localizer)
		evidence := localizeMonitorEvidence(item.Evidence, localizer)
		rows = append(rows, eventRow{
			Name:         causeName(item.Name, localizer),
			NameKey:      causeNameKey(item.Name),
			Status:       causeName(item.Status, localizer),
			StatusKey:    causeNameKey(item.Status),
			Summary:      summary.Current,
			SummaryI18N:  summary.Variants,
			Evidence:     evidence.Current,
			EvidenceI18N: evidence.Variants,
			StartedAt:    formatVisibleEventStart(item, visibleItem.interval, start, localizer),
			EndedAt:      formatVisibleEventEnd(item, visibleItem.interval, end, localizer),
			Duration:     formatDuration(int64(visibleItem.interval.end.Sub(visibleItem.interval.start).Seconds())),
		})
	}
	return rows
}

func buildEventSummary(events []model.Event, start, end time.Time) eventSummary {
	total := 0
	var longest int64
	for _, item := range events {
		if item.Name != "local" && item.Name != "domestic" && item.Name != "international" {
			continue
		}
		duration := clippedDuration(item, start, end)
		if duration == 0 {
			continue
		}
		total++
		if duration > longest {
			longest = duration
		}
	}
	return eventSummary{
		Count:   total,
		Longest: formatDuration(longest),
	}
}

func filterSamples(samples []model.Sample, start, end time.Time) []model.Sample {
	var filtered []model.Sample
	for _, item := range samples {
		if item.Timestamp.Before(start) || item.Timestamp.After(end) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func filterEvents(events []model.Event, start, end time.Time) []model.Event {
	var filtered []model.Event
	for _, item := range events {
		if clippedDuration(item, start, end) == 0 {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func clippedDuration(item model.Event, start, end time.Time) int64 {
	interval, ok := clippedInterval(item, start, end)
	if !ok {
		return 0
	}
	return int64(interval.end.Sub(interval.start).Seconds())
}

func formatVisibleEventStart(item model.Event, interval timeInterval, windowStart time.Time, localizer i18n.Localizer) string {
	value := interval.start.Local().Format("2006-01-02 15:04:05")
	if item.StartedAt.Before(windowStart) {
		return value + localizer.T("event.before_window")
	}
	return value
}

func formatVisibleEventEnd(item model.Event, interval timeInterval, windowEnd time.Time, localizer i18n.Localizer) string {
	value := interval.end.Local().Format("2006-01-02 15:04:05")
	if item.EndedAt == nil {
		return value + localizer.T("event.ongoing")
	}
	if item.EndedAt.After(windowEnd) {
		return value + localizer.T("event.after_window")
	}
	return value
}

type timeInterval struct {
	start time.Time
	end   time.Time
}

func clippedInterval(item model.Event, start, end time.Time) (timeInterval, bool) {
	eventStart := item.StartedAt
	eventEnd := end
	if item.EndedAt != nil {
		eventEnd = *item.EndedAt
	}
	if eventEnd.Before(start) || eventStart.After(end) {
		return timeInterval{}, false
	}
	if eventStart.Before(start) {
		eventStart = start
	}
	if eventEnd.After(end) {
		eventEnd = end
	}
	if eventEnd.Before(eventStart) {
		return timeInterval{}, false
	}
	return timeInterval{start: eventStart, end: eventEnd}, true
}

func mergedDuration(intervals []timeInterval) int64 {
	if len(intervals) == 0 {
		return 0
	}
	items := append([]timeInterval(nil), intervals...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].start.Equal(items[j].start) {
			return items[i].end.Before(items[j].end)
		}
		return items[i].start.Before(items[j].start)
	})

	current := items[0]
	var total int64
	for _, item := range items[1:] {
		if item.start.After(current.end) {
			total += int64(current.end.Sub(current.start).Seconds())
			current = item
			continue
		}
		if item.end.After(current.end) {
			current.end = item.end
		}
	}
	total += int64(current.end.Sub(current.start).Seconds())
	return total
}

type localizedEventText struct {
	Current  string
	Variants map[string]string
}

func localizeMonitorSummary(raw string, localizer i18n.Localizer) localizedEventText {
	english, chinese, ok := monitorSummaryVariants(raw)
	if !ok {
		return localizedEventText{Current: raw}
	}
	return localizedTextForLang(english, chinese, localizer)
}

func monitorSummaryVariants(raw string) (string, string, bool) {
	if match := englishLocalSummaryRE.FindStringSubmatch(raw); len(match) == 5 {
		values, ok := parseFloatMatches(match[1:]...)
		if !ok {
			return "", "", false
		}
		return formatLocalSummary(values, i18n.English), formatLocalSummary(values, i18n.Chinese), true
	}
	if match := chineseLocalSummaryRE.FindStringSubmatch(raw); len(match) == 5 {
		values, ok := parseFloatMatches(match[1:]...)
		if !ok {
			return "", "", false
		}
		return formatLocalSummary(values, i18n.English), formatLocalSummary(values, i18n.Chinese), true
	}
	if match := englishRemoteSummaryRE.FindStringSubmatch(raw); len(match) == 5 {
		values, ok := parseFloatMatches(match[2:]...)
		if !ok {
			return "", "", false
		}
		layer := "domestic"
		if match[1] == "International" {
			layer = "international"
		}
		return formatRemoteSummary(layer, values, i18n.English), formatRemoteSummary(layer, values, i18n.Chinese), true
	}
	if match := chineseRemoteSummaryRE.FindStringSubmatch(raw); len(match) == 5 {
		values, ok := parseFloatMatches(match[2:]...)
		if !ok {
			return "", "", false
		}
		layer := "domestic"
		if match[1] == "国外" {
			layer = "international"
		}
		return formatRemoteSummary(layer, values, i18n.English), formatRemoteSummary(layer, values, i18n.Chinese), true
	}
	return "", "", false
}

func formatLocalSummary(values []float64, lang i18n.Lang) string {
	return fmt.Sprintf(i18n.New(lang).T("state.local_summary"), values[0], values[1], values[2], values[3])
}

func formatRemoteSummary(layer string, values []float64, lang i18n.Lang) string {
	localizer := i18n.New(lang)
	return fmt.Sprintf(localizer.T("state.remote_summary"), localizer.T("layer."+layer), values[0], values[1], values[2])
}

func localizeMonitorEvidence(raw string, localizer i18n.Localizer) localizedEventText {
	if strings.TrimSpace(raw) == "" {
		return localizedEventText{Current: raw}
	}
	parts := splitEvidence(raw)
	englishParts := make([]string, 0, len(parts))
	chineseParts := make([]string, 0, len(parts))
	known := false
	for _, part := range parts {
		english, chinese, ok := monitorEvidenceItemVariants(part)
		if ok {
			known = true
			englishParts = append(englishParts, english)
			chineseParts = append(chineseParts, chinese)
			continue
		}
		englishParts = append(englishParts, part)
		chineseParts = append(chineseParts, part)
	}
	if !known {
		return localizedEventText{Current: raw}
	}
	english := strings.Join(englishParts, "; ")
	chinese := strings.Join(chineseParts, "；")
	return localizedTextForLang(english, chinese, localizer)
}

func splitEvidence(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ';' || r == '；'
	})
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func monitorEvidenceItemVariants(raw string) (string, string, bool) {
	for _, pattern := range []struct {
		key string
		en  *regexp.Regexp
		zh  *regexp.Regexp
	}{
		{key: "evidence.jitter", en: regexp.MustCompile(`^jitter ([0-9.]+)ms > ([0-9.]+)ms$`), zh: regexp.MustCompile(`^抖动 ([0-9.]+)ms > ([0-9.]+)ms$`)},
		{key: "evidence.loss", en: regexp.MustCompile(`^packet loss ([0-9.]+)% > ([0-9.]+)%$`), zh: regexp.MustCompile(`^丢包率 ([0-9.]+)% > ([0-9.]+)%$`)},
		{key: "evidence.avg_latency", en: regexp.MustCompile(`^average latency ([0-9.]+)ms > ([0-9.]+)ms$`), zh: regexp.MustCompile(`^平均延迟 ([0-9.]+)ms > ([0-9.]+)ms$`)},
		{key: "evidence.failure_rate", en: regexp.MustCompile(`^failure rate ([0-9.]+)% > ([0-9.]+)%$`), zh: regexp.MustCompile(`^失败率 ([0-9.]+)% > ([0-9.]+)%$`)},
		{key: "evidence.avg_download", en: regexp.MustCompile(`^average download ([0-9.]+)Mbps < ([0-9.]+)Mbps$`), zh: regexp.MustCompile(`^平均下载速率 ([0-9.]+)Mbps < ([0-9.]+)Mbps$`)},
	} {
		if english, chinese, ok := evidenceVariantForPattern(raw, pattern.key, pattern.en, pattern.zh); ok {
			return english, chinese, true
		}
	}
	return "", "", false
}

func evidenceVariantForPattern(raw, key string, englishPattern, chinesePattern *regexp.Regexp) (string, string, bool) {
	match := englishPattern.FindStringSubmatch(raw)
	if len(match) != 3 {
		match = chinesePattern.FindStringSubmatch(raw)
	}
	if len(match) != 3 {
		return "", "", false
	}
	values, ok := parseFloatMatches(match[1], match[2])
	if !ok {
		return "", "", false
	}
	return fmt.Sprintf(i18n.New(i18n.English).T(key), values[0], values[1]),
		fmt.Sprintf(i18n.New(i18n.Chinese).T(key), values[0], values[1]),
		true
}

func localizedTextForLang(english, chinese string, localizer i18n.Localizer) localizedEventText {
	current := english
	if localizer.Lang() == i18n.Chinese {
		current = chinese
	}
	return localizedEventText{
		Current: current,
		Variants: map[string]string{
			i18n.English.Code(): english,
			i18n.Chinese.Code(): chinese,
		},
	}
}

func parseFloatMatches(values ...string) ([]float64, bool) {
	parsed := make([]float64, 0, len(values))
	for _, value := range values {
		item, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, false
		}
		parsed = append(parsed, item)
	}
	return parsed, true
}
