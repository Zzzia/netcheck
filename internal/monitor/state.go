package monitor

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Zzzia/netcheck/internal/config"
	"github.com/Zzzia/netcheck/internal/i18n"
	"github.com/Zzzia/netcheck/internal/model"
)

type point struct {
	Timestamp time.Time
	Success   bool
	LatencyMs float64
	Value     float64
}

type rollingWindow struct {
	limit  int
	points []point
}

type metricStats struct {
	Count        int
	SuccessCount int
	FailureRatio float64
	AvgLatencyMs float64
	P95LatencyMs float64
	JitterMs     float64
	AvgValue     float64
}

const (
	localPingWindowLimit             = 20
	remoteLatencyWindowLimit         = 24
	domesticDownloadWindowLimit      = 6
	internationalDownloadWindowLimit = 3
)

type windows struct {
	localPing        rollingWindow
	domesticTCP      rollingWindow
	domesticDL       rollingWindow
	internationalTCP rollingWindow
	internationalDL  rollingWindow
}

type transition struct {
	Name      string
	Started   bool
	Resolved  bool
	Summary   string
	Evidence  string
	Timestamp time.Time
}

type stateTracker struct {
	enterAfter   int
	recoverAfter int
	badCount     int
	goodCount    int
	active       bool
	eventID      int64
	lastSummary  string
	lastEvidence string
}

func (t *stateTracker) restoreOpenEvent(event model.Event) {
	t.active = true
	t.eventID = event.ID
	t.badCount = t.enterAfter
	t.goodCount = 0
	t.lastSummary = event.Summary
	t.lastEvidence = event.Evidence
}

func newWindows() *windows {
	return &windows{
		localPing:        rollingWindow{limit: localPingWindowLimit},
		domesticTCP:      rollingWindow{limit: remoteLatencyWindowLimit},
		domesticDL:       rollingWindow{limit: domesticDownloadWindowLimit},
		internationalTCP: rollingWindow{limit: remoteLatencyWindowLimit},
		internationalDL:  rollingWindow{limit: internationalDownloadWindowLimit},
	}
}

func (w *rollingWindow) add(item point) {
	w.points = append(w.points, item)
	if len(w.points) > w.limit {
		w.points = append([]point(nil), w.points[len(w.points)-w.limit:]...)
	}
}

func (w *rollingWindow) stats() metricStats {
	stats := metricStats{Count: len(w.points)}
	if len(w.points) == 0 {
		return stats
	}
	var (
		latencies []float64
		sumLat    float64
		sumValue  float64
	)
	for _, item := range w.points {
		if item.Success {
			stats.SuccessCount++
			if item.LatencyMs > 0 {
				latencies = append(latencies, item.LatencyMs)
				sumLat += item.LatencyMs
			}
			if item.Value > 0 {
				sumValue += item.Value
			}
		}
	}
	stats.FailureRatio = 1 - float64(stats.SuccessCount)/float64(stats.Count)
	if len(latencies) > 0 {
		stats.AvgLatencyMs = sumLat / float64(len(latencies))
		stats.P95LatencyMs = percentile(latencies, 0.95)
		stats.JitterMs = jitter(latencies)
	}
	if stats.SuccessCount > 0 {
		stats.AvgValue = sumValue / float64(stats.SuccessCount)
	}
	return stats
}

func percentile(values []float64, ratio float64) float64 {
	if len(values) == 0 {
		return 0
	}
	items := append([]float64(nil), values...)
	sort.Float64s(items)
	index := int(float64(len(items)-1) * ratio)
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

func (w *windows) add(sample model.Sample) {
	item := point{
		Timestamp: sample.Timestamp,
		Success:   sample.Success,
		LatencyMs: sample.LatencyMs,
		Value:     sample.Value,
	}
	switch sample.Layer + "/" + sample.Metric {
	case "local/ping":
		w.localPing.add(item)
	case "domestic/tcp_connect":
		w.domesticTCP.add(item)
	case "domestic/download":
		w.domesticDL.add(item)
	case "international/tcp_connect":
		w.internationalTCP.add(item)
	case "international/download":
		w.internationalDL.add(item)
	}
}

func (w *windows) evaluate(cfg config.Config, now time.Time) map[string]model.LayerSnapshot {
	return w.evaluateForLang(cfg, now, i18n.New(i18n.English))
}

func (w *windows) evaluateForLang(cfg config.Config, now time.Time, localizer i18n.Localizer) map[string]model.LayerSnapshot {
	snapshots := map[string]model.LayerSnapshot{
		"local":         evaluateLocalForLang(cfg, now, w.localPing.stats(), localizer),
		"domestic":      evaluateRemoteLayerForLang("domestic", cfg, now, w.domesticTCP.stats(), w.domesticDL.stats(), localizer),
		"international": evaluateRemoteLayerForLang("international", cfg, now, w.internationalTCP.stats(), w.internationalDL.stats(), localizer),
	}
	return snapshots
}

func evaluateLocal(cfg config.Config, now time.Time, stats metricStats) model.LayerSnapshot {
	return evaluateLocalForLang(cfg, now, stats, i18n.New(i18n.English))
}

func evaluateLocalForLang(cfg config.Config, now time.Time, stats metricStats, localizer i18n.Localizer) model.LayerSnapshot {
	snapshot := model.LayerSnapshot{Layer: "local", LastEvaluated: now}
	if stats.Count < 5 {
		snapshot.Summary = localizer.T("state.local_insufficient")
		return snapshot
	}
	var evidence []string
	if stats.P95LatencyMs > cfg.Thresholds.GatewayRTTWarnMs {
		evidence = append(evidence, fmt.Sprintf("P95 RTT %.1fms > %.1fms", stats.P95LatencyMs, cfg.Thresholds.GatewayRTTWarnMs))
	}
	if stats.JitterMs > cfg.Thresholds.GatewayJitterWarnMs {
		evidence = append(evidence, fmt.Sprintf(localizer.T("evidence.jitter"), stats.JitterMs, cfg.Thresholds.GatewayJitterWarnMs))
	}
	if stats.FailureRatio > cfg.Thresholds.GatewayLossWarnRatio {
		evidence = append(evidence, fmt.Sprintf(localizer.T("evidence.loss"), stats.FailureRatio*100, cfg.Thresholds.GatewayLossWarnRatio*100))
	}
	snapshot.Degraded = len(evidence) > 0
	snapshot.Summary = fmt.Sprintf(localizer.T("state.local_summary"), stats.AvgLatencyMs, stats.P95LatencyMs, stats.JitterMs, stats.FailureRatio*100)
	snapshot.Evidence = joinEvidence(evidence, localizer)
	return snapshot
}

func evaluateRemoteLayer(layer string, cfg config.Config, now time.Time, latencyStats, downloadStats metricStats) model.LayerSnapshot {
	return evaluateRemoteLayerForLang(layer, cfg, now, latencyStats, downloadStats, i18n.New(i18n.English))
}

func evaluateRemoteLayerForLang(layer string, cfg config.Config, now time.Time, latencyStats, downloadStats metricStats, localizer i18n.Localizer) model.LayerSnapshot {
	snapshot := model.LayerSnapshot{Layer: layer, LastEvaluated: now}
	var evidence []string
	label := layerDisplayName(layer, localizer)
	var warnLatency float64
	var warnMbps float64
	if layer == "domestic" {
		warnLatency = cfg.Thresholds.DomesticLatencyWarnMs
		warnMbps = cfg.Thresholds.DomesticDownloadWarnMbps
	} else {
		warnLatency = cfg.Thresholds.InternationalLatencyWarnMs
		warnMbps = cfg.Thresholds.InternationalDownloadWarnMbps
	}
	if latencyStats.Count >= 4 {
		if latencyStats.AvgLatencyMs > warnLatency {
			evidence = append(evidence, fmt.Sprintf(localizer.T("evidence.avg_latency"), latencyStats.AvgLatencyMs, warnLatency))
		}
		if latencyStats.FailureRatio > cfg.Thresholds.RemoteFailureWarnRatio {
			evidence = append(evidence, fmt.Sprintf(localizer.T("evidence.failure_rate"), latencyStats.FailureRatio*100, cfg.Thresholds.RemoteFailureWarnRatio*100))
		}
	}
	if downloadStats.Count >= 2 && downloadStats.SuccessCount > 0 && downloadStats.AvgValue < warnMbps {
		evidence = append(evidence, fmt.Sprintf(localizer.T("evidence.avg_download"), downloadStats.AvgValue, warnMbps))
	}
	snapshot.Degraded = len(evidence) > 0
	snapshot.Summary = fmt.Sprintf(
		localizer.T("state.remote_summary"),
		label,
		latencyStats.AvgLatencyMs,
		latencyStats.FailureRatio*100,
		downloadStats.AvgValue,
	)
	snapshot.Evidence = joinEvidence(evidence, localizer)
	return snapshot
}

func layerDisplayName(layer string, localizer i18n.Localizer) string {
	switch layer {
	case "local":
		return localizer.T("layer.gateway")
	case "domestic":
		return localizer.T("layer.domestic")
	case "international":
		return localizer.T("layer.international")
	default:
		return layer
	}
}

func joinEvidence(evidence []string, localizer i18n.Localizer) string {
	if localizer.Lang() == i18n.Chinese {
		return strings.Join(evidence, "；")
	}
	return strings.Join(evidence, "; ")
}

func newTracker(cfg config.Config) stateTracker {
	return stateTracker{
		enterAfter:   cfg.Thresholds.EnterConsecutive,
		recoverAfter: cfg.Thresholds.RecoverConsecutive,
	}
}

func (t *stateTracker) step(name string, snapshot model.LayerSnapshot) transition {
	change := transition{
		Name:      name,
		Summary:   snapshot.Summary,
		Evidence:  snapshot.Evidence,
		Timestamp: snapshot.LastEvaluated,
	}
	if snapshot.Degraded {
		t.badCount++
		t.goodCount = 0
		t.lastSummary = snapshot.Summary
		t.lastEvidence = snapshot.Evidence
		if !t.active && t.badCount >= t.enterAfter {
			t.active = true
			change.Started = true
			return change
		}
		return transition{}
	}
	t.goodCount++
	t.badCount = 0
	if t.active && t.goodCount >= t.recoverAfter {
		t.active = false
		change.Resolved = true
		change.Summary = t.lastSummary
		change.Evidence = t.lastEvidence
		t.lastSummary = ""
		t.lastEvidence = ""
		return change
	}
	return transition{}
}
