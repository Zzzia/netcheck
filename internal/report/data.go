package report

import (
	"fmt"
	"time"

	"netcheck/internal/model"
	"netcheck/internal/storage"
)

func LoadData(dbPath string, start, end time.Time) (reportData, error) {
	if end.Before(start) {
		return reportData{}, fmt.Errorf("结束时间早于开始时间: start=%s end=%s", start.Format(time.RFC3339), end.Format(time.RFC3339))
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
	return buildReportData(filteredSamples, filteredEvents, start, end), nil
}

func buildReportData(samples []model.Sample, events []model.Event, start, end time.Time) reportData {
	return reportData{
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		RangeLabel:  fmt.Sprintf("%s ~ %s", start.Local().Format("2006-01-02 15:04"), end.Local().Format("2006-01-02 15:04")),
		RangeStart:  start.Local().Format("2006-01-02 15:04"),
		RangeEnd:    end.Local().Format("2006-01-02 15:04"),
		Summary:     buildSummaryCards(samples),
		Groups:      buildChartGroups(samples, start, end),
		Causes:      buildCauseRows(events, start, end),
		Events:      buildEventRows(events, start, end),
		EventMeta:   buildEventSummary(events, start, end),
	}
}

func buildSummaryCards(samples []model.Sample) []summaryCard {
	localPing := collectMetric(samples, "local", "ping", metricLatency)
	domesticLatency := collectMetric(samples, "domestic", "tcp_connect", metricLatency)
	domesticDownload := collectMetric(samples, "domestic", "download", metricValue)
	internationalLatency := collectMetric(samples, "international", "tcp_connect", metricLatency)
	internationalDownload := collectMetric(samples, "international", "download", metricValue)

	return []summaryCard{
		{
			Title: "网关",
			Metrics: []summaryMetric{
				{
					Label: "RTT",
					Lines: []summaryMetricLine{
						{Label: "平均", Value: formatMetric(localPing.avg(), "ms")},
						{Label: "P95", Value: formatMetric(localPing.p95(), "ms")},
					},
				},
				{Label: "平均抖动", Value: formatMetric(jitter(localPing.Values), "ms")},
				{Label: "丢包率", Value: formatPercent(localPing.failureRatio())},
			},
		},
		{
			Title: "国内",
			Metrics: []summaryMetric{
				{
					Label: "延迟",
					Lines: []summaryMetricLine{
						{Label: "平均", Value: formatMetric(domesticLatency.avg(), "ms")},
						{Label: "P95", Value: formatMetric(domesticLatency.p95(), "ms")},
					},
				},
				{
					Label: "下载",
					Lines: []summaryMetricLine{
						{Label: "平均", Value: formatMetric(domesticDownload.avg(), "Mbps")},
						{Label: "P5", Value: formatMetric(domesticDownload.p05(), "Mbps")},
					},
				},
				{Label: "失败率", Value: formatPercent(domesticLatency.failureRatio())},
			},
		},
		{
			Title: "国外",
			Metrics: []summaryMetric{
				{
					Label: "延迟",
					Lines: []summaryMetricLine{
						{Label: "平均", Value: formatMetric(internationalLatency.avg(), "ms")},
						{Label: "P95", Value: formatMetric(internationalLatency.p95(), "ms")},
					},
				},
				{
					Label: "下载",
					Lines: []summaryMetricLine{
						{Label: "平均", Value: formatMetric(internationalDownload.avg(), "Mbps")},
						{Label: "P5", Value: formatMetric(internationalDownload.p05(), "Mbps")},
					},
				},
				{Label: "失败率", Value: formatPercent(internationalLatency.failureRatio())},
			},
		},
	}
}

func buildChartGroups(samples []model.Sample, start, end time.Time) []chartGroup {
	bucket := chooseChartBucket(end.Sub(start))
	keys := []chartKey{
		{Group: "网关质量", Layer: "local", Metric: "ping", ID: "gateway-rtt", Title: "网关 RTT", Unit: "ms", Color: "#e76f51", StartAtZero: true, Mode: "latency"},
		{Group: "网关质量", Layer: "local", Metric: "ping", ID: "gateway-loss", Title: "网关丢包率", Unit: "比例", Color: "#264653", StartAtZero: true, Mode: "loss"},
		{Group: "国内质量", Layer: "domestic", Metric: "tcp_connect", ID: "domestic-rtt", Title: "国内访问延迟", Unit: "ms", Color: "#2a9d8f", StartAtZero: true, Mode: "latency"},
		{Group: "国内质量", Layer: "domestic", Metric: "tcp_connect", ID: "domestic-failure", Title: "国内失败率", Unit: "比例", Color: "#264653", StartAtZero: true, Mode: "loss"},
		{Group: "国内质量", Layer: "domestic", Metric: "download", ID: "domestic-download", Title: "国内下载速率", Unit: "Mbps", Color: "#f4a261", StartAtZero: true, Mode: "value"},
		{Group: "国外质量", Layer: "international", Metric: "tcp_connect", ID: "international-rtt", Title: "国外访问延迟", Unit: "ms", Color: "#457b9d", StartAtZero: true, Mode: "latency"},
		{Group: "国外质量", Layer: "international", Metric: "tcp_connect", ID: "international-failure", Title: "国外失败率", Unit: "比例", Color: "#6d597a", StartAtZero: true, Mode: "loss"},
		{Group: "国外质量", Layer: "international", Metric: "download", ID: "international-download", Title: "国外下载速率", Unit: "Mbps", Color: "#8d99ae", StartAtZero: true, Mode: "value"},
	}

	groupOrder := []string{"网关质量", "国内质量", "国外质量"}
	grouped := map[string][]chart{}
	for _, key := range keys {
		grouped[key.Group] = append(grouped[key.Group], buildChart(samples, key, bucket))
	}

	var groups []chartGroup
	for _, name := range groupOrder {
		groups = append(groups, chartGroup{Title: name, Charts: grouped[name]})
	}
	return groups
}

func buildChart(samples []model.Sample, key chartKey, bucket chartBucket) chart {
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

	return chart{
		ID:          key.ID,
		Title:       key.Title,
		Unit:        key.Unit,
		TimeFormat:  bucket.TimeFormat,
		StartAtZero: key.StartAtZero,
		Series: []chartSeries{{
			Name:   key.Title,
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
