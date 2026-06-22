package report

import (
	"fmt"
	"math"
	"sort"
	"time"

	"netcheck/internal/i18n"
	"netcheck/internal/model"
)

func buildSeriesPoints(items map[time.Time]*aggregate, mode string) []chartPoint {
	var buckets []time.Time
	for bucket := range items {
		buckets = append(buckets, bucket)
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Before(buckets[j]) })

	var points []chartPoint
	for _, bucket := range buckets {
		agg := items[bucket]
		if agg == nil || agg.count == 0 {
			continue
		}
		var value float64
		switch mode {
		case "loss":
			value = float64(agg.fail) / float64(agg.count)
		default:
			if len(agg.vals) == 0 {
				continue
			}
			value = agg.sum / float64(len(agg.vals))
		}
		points = append(points, chartPoint{
			Ts:    bucket.Format(time.RFC3339),
			Value: round(value, 2),
		})
	}
	return points
}

func collectMetric(samples []model.Sample, layer, metric string, selector func(model.Sample) float64) metricAggregate {
	var aggregate metricAggregate
	for _, item := range samples {
		if item.Layer != layer || item.Metric != metric {
			continue
		}
		if !item.Success {
			aggregate.Failures++
			continue
		}
		value := selector(item)
		if value <= 0 {
			continue
		}
		aggregate.Successes++
		aggregate.Values = append(aggregate.Values, value)
	}
	return aggregate
}

func metricLatency(item model.Sample) float64 { return item.LatencyMs }

func metricValue(item model.Sample) float64 { return item.Value }

func (m metricAggregate) avg() float64 { return average(m.Values) }

func (m metricAggregate) p95() float64 { return percentile(m.Values, 0.95) }

func (m metricAggregate) p05() float64 { return percentile(m.Values, 0.05) }

func (m metricAggregate) failureRatio() float64 {
	total := m.Successes + m.Failures
	if total == 0 {
		return 0
	}
	return float64(m.Failures) / float64(total)
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func percentile(values []float64, ratio float64) float64 {
	if len(values) == 0 {
		return 0
	}
	items := append([]float64(nil), values...)
	sort.Float64s(items)
	index := int(math.Round(float64(len(items)-1) * ratio))
	if index < 0 {
		index = 0
	}
	if index >= len(items) {
		index = len(items) - 1
	}
	return items[index]
}

func jitter(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	var total float64
	for index := 1; index < len(values); index++ {
		delta := values[index] - values[index-1]
		if delta < 0 {
			delta = -delta
		}
		total += delta
	}
	return total / float64(len(values)-1)
}

func formatMetric(value float64, unit string, localizer i18n.Localizer) string {
	if value == 0 {
		return localizer.T("common.no_data")
	}
	if unit == "Mbps" {
		return fmt.Sprintf("%.2f %s", value, unit)
	}
	return fmt.Sprintf("%.1f %s", value, unit)
}

func formatPercent(value float64) string {
	percent := value * 100
	switch {
	case percent == 0:
		return "0%"
	case percent < 0.1:
		return fmt.Sprintf("%.3f%%", percent)
	case percent < 1:
		return fmt.Sprintf("%.2f%%", percent)
	default:
		return fmt.Sprintf("%.1f%%", percent)
	}
}

func formatDuration(seconds int64) string {
	if seconds <= 0 {
		return "0s"
	}
	duration := time.Duration(seconds) * time.Second
	if duration >= time.Hour {
		return fmt.Sprintf("%.1fh", duration.Hours())
	}
	if duration >= time.Minute {
		return fmt.Sprintf("%.1fm", duration.Minutes())
	}
	return duration.String()
}

func causeName(raw string, localizer i18n.Localizer) string {
	return localizer.T(causeNameKey(raw))
}

func causeNameKey(raw string) string {
	switch raw {
	case "root_cause":
		return "cause.root"
	case "local":
		return "cause.local"
	case "domestic":
		return "cause.domestic"
	case "international":
		return "cause.international"
	case "global":
		return "cause.global"
	case "degraded":
		return "cause.degraded"
	default:
		return raw
	}
}

func round(value float64, digits int) float64 {
	pow := math.Pow10(digits)
	return math.Round(value*pow) / pow
}
