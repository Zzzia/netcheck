package report

import (
	"fmt"
	"time"

	"github.com/Zzzia/netcheck/internal/i18n"
	"github.com/Zzzia/netcheck/internal/model"
	"github.com/Zzzia/netcheck/internal/storage"
)

func LoadData(dbPath string, start, end time.Time) (reportData, error) {
	return LoadDataForLang(dbPath, start, end, i18n.English)
}

func LoadDataForLang(dbPath string, start, end time.Time, lang i18n.Lang) (reportData, error) {
	localizer := i18n.New(lang)
	if end.Before(start) {
		return reportData{}, fmt.Errorf(localizer.T("report.error.end_before_start"), start.Format(time.RFC3339), end.Format(time.RFC3339))
	}
	store, err := storage.Open(dbPath)
	if err != nil {
		return reportData{}, err
	}
	defer store.Close()

	samples, err := store.LoadSamplesSince(start)
	if err != nil {
		return reportData{}, err
	}
	events, err := store.LoadEventsSince(start)
	if err != nil {
		return reportData{}, err
	}
	filteredSamples := filterSamples(samples, start, end)
	filteredEvents := filterEvents(events, start, end)
	return buildReportDataForLang(filteredSamples, filteredEvents, start, end, localizer), nil
}

func buildReportData(samples []model.Sample, events []model.Event, start, end time.Time) reportData {
	return buildReportDataForLang(samples, events, start, end, i18n.New(i18n.English))
}

func buildReportDataForLang(samples []model.Sample, events []model.Event, start, end time.Time, localizer i18n.Localizer) reportData {
	return reportData{
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		RangeLabel:  fmt.Sprintf("%s ~ %s", start.Local().Format("2006-01-02 15:04"), end.Local().Format("2006-01-02 15:04")),
		RangeStart:  start.Local().Format("2006-01-02 15:04"),
		RangeEnd:    end.Local().Format("2006-01-02 15:04"),
		Summary:     buildSummaryCardsForLang(samples, localizer),
		Groups:      buildChartGroupsForLang(samples, start, end, localizer),
		Causes:      buildCauseRowsForLang(events, start, end, localizer),
		Events:      buildEventRowsForLang(events, start, end, localizer),
		EventMeta:   buildEventSummary(events, start, end),
	}
}

func buildSummaryCards(samples []model.Sample) []summaryCard {
	return buildSummaryCardsForLang(samples, i18n.New(i18n.English))
}

func buildSummaryCardsForLang(samples []model.Sample, localizer i18n.Localizer) []summaryCard {
	localPing := collectMetric(samples, "local", "ping", metricLatency)
	domesticLatency := collectMetric(samples, "domestic", "tcp_connect", metricLatency)
	domesticDownload := collectMetric(samples, "domestic", "download", metricValue)
	internationalLatency := collectMetric(samples, "international", "tcp_connect", metricLatency)
	internationalDownload := collectMetric(samples, "international", "download", metricValue)

	return []summaryCard{
		{
			Title:    localizer.T("summary.gateway"),
			TitleKey: "summary.gateway",
			Metrics: []summaryMetric{
				{
					Label:    localizer.T("metric.rtt"),
					LabelKey: "metric.rtt",
					Lines: []summaryMetricLine{
						{Label: localizer.T("metric.average"), LabelKey: "metric.average", Value: formatMetric(localPing.avg(), "ms", localizer)},
						{Label: "P95", Value: formatMetric(localPing.p95(), "ms", localizer)},
					},
				},
				{Label: localizer.T("metric.average_jitter"), LabelKey: "metric.average_jitter", Value: formatMetric(jitter(localPing.Values), "ms", localizer)},
				{Label: localizer.T("metric.loss_rate"), LabelKey: "metric.loss_rate", Value: formatPercent(localPing.failureRatio())},
			},
		},
		{
			Title:    localizer.T("summary.domestic"),
			TitleKey: "summary.domestic",
			Metrics: []summaryMetric{
				{
					Label:    localizer.T("metric.latency"),
					LabelKey: "metric.latency",
					Lines: []summaryMetricLine{
						{Label: localizer.T("metric.average"), LabelKey: "metric.average", Value: formatMetric(domesticLatency.avg(), "ms", localizer)},
						{Label: "P95", Value: formatMetric(domesticLatency.p95(), "ms", localizer)},
					},
				},
				{
					Label:    localizer.T("metric.download"),
					LabelKey: "metric.download",
					Lines: []summaryMetricLine{
						{Label: localizer.T("metric.average"), LabelKey: "metric.average", Value: formatMetric(domesticDownload.avg(), "Mbps", localizer)},
						{Label: "P5", Value: formatMetric(domesticDownload.p05(), "Mbps", localizer)},
					},
				},
				{Label: localizer.T("metric.failure_rate"), LabelKey: "metric.failure_rate", Value: formatPercent(domesticLatency.failureRatio())},
			},
		},
		{
			Title:    localizer.T("summary.international"),
			TitleKey: "summary.international",
			Metrics: []summaryMetric{
				{
					Label:    localizer.T("metric.latency"),
					LabelKey: "metric.latency",
					Lines: []summaryMetricLine{
						{Label: localizer.T("metric.average"), LabelKey: "metric.average", Value: formatMetric(internationalLatency.avg(), "ms", localizer)},
						{Label: "P95", Value: formatMetric(internationalLatency.p95(), "ms", localizer)},
					},
				},
				{
					Label:    localizer.T("metric.download"),
					LabelKey: "metric.download",
					Lines: []summaryMetricLine{
						{Label: localizer.T("metric.average"), LabelKey: "metric.average", Value: formatMetric(internationalDownload.avg(), "Mbps", localizer)},
						{Label: "P5", Value: formatMetric(internationalDownload.p05(), "Mbps", localizer)},
					},
				},
				{Label: localizer.T("metric.failure_rate"), LabelKey: "metric.failure_rate", Value: formatPercent(internationalLatency.failureRatio())},
			},
		},
	}
}

func buildChartGroups(samples []model.Sample, start, end time.Time) []chartGroup {
	return buildChartGroupsForLang(samples, start, end, i18n.New(i18n.English))
}

func buildChartGroupsForLang(samples []model.Sample, start, end time.Time, localizer i18n.Localizer) []chartGroup {
	bucket := chooseChartBucket(end.Sub(start))
	keys := []chartKey{
		{GroupKey: "group.gateway_quality", Layer: "local", Metric: "ping", ID: "gateway-rtt", TitleKey: "chart.gateway_rtt", Unit: "ms", Color: "#e76f51", StartAtZero: true, Mode: "latency"},
		{GroupKey: "group.gateway_quality", Layer: "local", Metric: "ping", ID: "gateway-loss", TitleKey: "chart.gateway_loss", UnitKey: "unit.ratio", Color: "#264653", StartAtZero: true, Mode: "loss"},
		{GroupKey: "group.domestic_quality", Layer: "domestic", Metric: "tcp_connect", ID: "domestic-rtt", TitleKey: "chart.domestic_rtt", Unit: "ms", Color: "#2a9d8f", StartAtZero: true, Mode: "latency"},
		{GroupKey: "group.domestic_quality", Layer: "domestic", Metric: "tcp_connect", ID: "domestic-failure", TitleKey: "chart.domestic_failure", UnitKey: "unit.ratio", Color: "#264653", StartAtZero: true, Mode: "loss"},
		{GroupKey: "group.domestic_quality", Layer: "domestic", Metric: "download", ID: "domestic-download", TitleKey: "chart.domestic_download", Unit: "Mbps", Color: "#f4a261", StartAtZero: true, Mode: "value"},
		{GroupKey: "group.international_quality", Layer: "international", Metric: "tcp_connect", ID: "international-rtt", TitleKey: "chart.international_rtt", Unit: "ms", Color: "#457b9d", StartAtZero: true, Mode: "latency"},
		{GroupKey: "group.international_quality", Layer: "international", Metric: "tcp_connect", ID: "international-failure", TitleKey: "chart.international_failure", UnitKey: "unit.ratio", Color: "#6d597a", StartAtZero: true, Mode: "loss"},
		{GroupKey: "group.international_quality", Layer: "international", Metric: "download", ID: "international-download", TitleKey: "chart.international_download", Unit: "Mbps", Color: "#8d99ae", StartAtZero: true, Mode: "value"},
	}

	groupOrder := []string{"group.gateway_quality", "group.domestic_quality", "group.international_quality"}
	grouped := map[string][]chart{}
	for _, key := range keys {
		grouped[key.GroupKey] = append(grouped[key.GroupKey], buildChart(samples, key, bucket, localizer))
	}

	var groups []chartGroup
	for _, key := range groupOrder {
		groups = append(groups, chartGroup{Title: localizer.T(key), TitleKey: key, Charts: grouped[key]})
	}
	return groups
}

func buildChart(samples []model.Sample, key chartKey, bucket chartBucket, localizer i18n.Localizer) chart {
	aggregates := map[time.Time]*aggregate{}
	for _, item := range samples {
		if item.Layer != key.Layer || item.Metric != key.Metric {
			continue
		}
		bucketTime := item.Timestamp.Local().Truncate(bucket.Duration)
		agg := aggregates[bucketTime]
		if agg == nil {
			agg = &aggregate{}
			aggregates[bucketTime] = agg
		}
		agg.count++
		if !item.Success {
			agg.fail++
			continue
		}
		switch key.Mode {
		case "latency":
			agg.sum += item.LatencyMs
			agg.vals = append(agg.vals, item.LatencyMs)
		case "value":
			agg.sum += item.Value
			agg.vals = append(agg.vals, item.Value)
		}
	}

	title := localizer.T(key.TitleKey)
	unit := key.Unit
	if key.UnitKey != "" {
		unit = localizer.T(key.UnitKey)
	}
	return chart{
		ID:          key.ID,
		Title:       title,
		TitleKey:    key.TitleKey,
		Unit:        unit,
		UnitKey:     key.UnitKey,
		TimeFormat:  bucket.TimeFormat,
		StartAtZero: key.StartAtZero,
		Series: []chartSeries{{
			Name:   title,
			Color:  key.Color,
			Points: buildSeriesPoints(aggregates, key.Mode),
		}},
	}
}

type chartBucket struct {
	Duration   time.Duration
	TimeFormat string
}

func chooseChartBucket(span time.Duration) chartBucket {
	switch {
	case span <= 5*time.Minute:
		return chartBucket{Duration: time.Second, TimeFormat: "second"}
	case span > 12*time.Hour:
		return chartBucket{Duration: time.Hour, TimeFormat: "hour"}
	default:
		return chartBucket{Duration: time.Minute, TimeFormat: "minute"}
	}
}
