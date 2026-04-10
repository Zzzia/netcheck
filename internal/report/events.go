package report

import (
	"sort"
	"time"

	"netcheck/internal/model"
)

func buildCauseRows(events []model.Event, start, end time.Time) []causeRow {
	durations := map[string]int64{}
	for _, item := range events {
		if item.Name != "local" && item.Name != "domestic" && item.Name != "international" {
			continue
		}
		durations[item.Name] += clippedDuration(item, start, end)
	}
	var rows []causeRow
	for _, name := range []string{"local", "domestic", "international"} {
		if durations[name] == 0 {
			continue
		}
		rows = append(rows, causeRow{
			Name:        causeName(name),
			DurationSec: durations[name],
			Duration:    formatDuration(durations[name]),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].DurationSec > rows[j].DurationSec })
	return rows
}

func buildEventRows(events []model.Event, start, end time.Time) []eventRow {
	var visible []model.Event
	for _, item := range events {
		if item.Name != "local" && item.Name != "domestic" && item.Name != "international" {
			continue
		}
		if clippedDuration(item, start, end) == 0 {
			continue
		}
		visible = append(visible, item)
	}
	sort.Slice(visible, func(i, j int) bool {
		return visible[i].StartedAt.After(visible[j].StartedAt)
	})
	if len(visible) > 10 {
		visible = visible[:10]
	}

	var rows []eventRow
	for _, item := range visible {
		duration := clippedDuration(item, start, end)
		endedAt := "进行中"
		if item.EndedAt != nil {
			endedAt = item.EndedAt.Local().Format("2006-01-02 15:04:05")
		}
		rows = append(rows, eventRow{
			Name:      causeName(item.Name),
			Status:    causeName(item.Status),
			Summary:   item.Summary,
			Evidence:  item.Evidence,
			StartedAt: item.StartedAt.Local().Format("2006-01-02 15:04:05"),
			EndedAt:   endedAt,
			Duration:  formatDuration(duration),
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
	eventStart := item.StartedAt
	eventEnd := end
	if item.EndedAt != nil {
		eventEnd = *item.EndedAt
	}
	if eventEnd.Before(start) || eventStart.After(end) {
		return 0
	}
	if eventStart.Before(start) {
		eventStart = start
	}
	if eventEnd.After(end) {
		eventEnd = end
	}
	if eventEnd.Before(eventStart) {
		return 0
	}
	return int64(eventEnd.Sub(eventStart).Seconds())
}
