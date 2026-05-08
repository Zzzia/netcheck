package report

import (
	"sort"
	"time"

	"netcheck/internal/model"
)

func buildCauseRows(events []model.Event, start, end time.Time) []causeRow {
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
			Name:        causeName(name),
			DurationSec: duration,
			Duration:    formatDuration(duration),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].DurationSec > rows[j].DurationSec })
	return rows
}

func buildEventRows(events []model.Event, start, end time.Time) []eventRow {
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
		rows = append(rows, eventRow{
			Name:      causeName(item.Name),
			Status:    causeName(item.Status),
			Summary:   item.Summary,
			Evidence:  item.Evidence,
			StartedAt: formatVisibleEventStart(item, visibleItem.interval, start),
			EndedAt:   formatVisibleEventEnd(item, visibleItem.interval, end),
			Duration:  formatDuration(int64(visibleItem.interval.end.Sub(visibleItem.interval.start).Seconds())),
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

func formatVisibleEventStart(item model.Event, interval timeInterval, windowStart time.Time) string {
	value := interval.start.Local().Format("2006-01-02 15:04:05")
	if item.StartedAt.Before(windowStart) {
		return value + "（窗口前开始）"
	}
	return value
}

func formatVisibleEventEnd(item model.Event, interval timeInterval, windowEnd time.Time) string {
	value := interval.end.Local().Format("2006-01-02 15:04:05")
	if item.EndedAt == nil {
		return value + "（进行中）"
	}
	if item.EndedAt.After(windowEnd) {
		return value + "（窗口后结束）"
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
