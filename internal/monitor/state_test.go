package monitor

import (
	"testing"
	"time"

	"github.com/Zzzia/netcheck/internal/config"
	"github.com/Zzzia/netcheck/internal/model"
)

func TestEvaluateLocalDetectsHighJitter(t *testing.T) {
	cfg, err := config.Default()
	if err != nil {
		t.Fatalf("读取默认配置失败: %v", err)
	}
	window := rollingWindow{limit: 20}
	for _, latency := range []float64{10, 80, 12, 85, 11, 90} {
		window.add(point{Timestamp: time.Now(), Success: true, LatencyMs: latency})
	}
	snapshot := evaluateLocal(cfg, time.Now(), window.stats())
	if !snapshot.Degraded {
		t.Fatal("期望本地链路被判定为异常")
	}
}

func TestEvaluateRemoteLayerDetectsLowThroughput(t *testing.T) {
	cfg, err := config.Default()
	if err != nil {
		t.Fatalf("读取默认配置失败: %v", err)
	}
	latencyWindow := rollingWindow{limit: remoteLatencyWindowLimit}
	for _, latency := range []float64{30, 28, 26, 27} {
		latencyWindow.add(point{Timestamp: time.Now(), Success: true, LatencyMs: latency})
	}
	downloadWindow := rollingWindow{limit: internationalDownloadWindowLimit}
	for _, throughput := range []float64{2.5, 3.0, 2.8} {
		downloadWindow.add(point{Timestamp: time.Now(), Success: true, Value: throughput})
	}
	snapshot := evaluateRemoteLayer("international", cfg, time.Now(), latencyWindow.stats(), downloadWindow.stats())
	if !snapshot.Degraded {
		t.Fatal("期望国外链路被判定为异常")
	}
}

func TestNewWindowsUsesSamplingAlignedDownloadHorizon(t *testing.T) {
	windows := newWindows()
	if windows.domesticDL.limit != domesticDownloadWindowLimit {
		t.Fatalf("国内下载窗口应为 %d，实际为 %d", domesticDownloadWindowLimit, windows.domesticDL.limit)
	}
	if windows.internationalDL.limit != internationalDownloadWindowLimit {
		t.Fatalf("国外下载窗口应为 %d，实际为 %d", internationalDownloadWindowLimit, windows.internationalDL.limit)
	}
}

func TestEvaluateRemoteLayerUsesRecentDownloadSamples(t *testing.T) {
	cfg, err := config.Default()
	if err != nil {
		t.Fatalf("读取默认配置失败: %v", err)
	}
	latencyWindow := rollingWindow{limit: remoteLatencyWindowLimit}
	for _, latency := range []float64{30, 28, 26, 27} {
		latencyWindow.add(point{Timestamp: time.Now(), Success: true, LatencyMs: latency})
	}
	downloadWindow := rollingWindow{limit: internationalDownloadWindowLimit}
	for _, throughput := range []float64{4.58, 4.25, 10.80, 8.93, 12.28} {
		downloadWindow.add(point{Timestamp: time.Now(), Success: true, Value: throughput})
	}
	snapshot := evaluateRemoteLayer("international", cfg, time.Now(), latencyWindow.stats(), downloadWindow.stats())
	if snapshot.Degraded {
		t.Fatalf("最近下载样本均值已恢复到阈值以上，不应继续判定异常: %s", snapshot.Evidence)
	}
}

func TestStateTrackerDebouncesTransitions(t *testing.T) {
	cfg, err := config.Default()
	if err != nil {
		t.Fatalf("读取默认配置失败: %v", err)
	}
	tracker := newTracker(cfg)
	now := time.Now()
	for i := 0; i < cfg.Thresholds.EnterConsecutive-1; i++ {
		change := tracker.step("local", model.LayerSnapshot{
			Layer:         "local",
			Degraded:      true,
			Summary:       "bad",
			LastEvaluated: now.Add(time.Duration(i) * time.Second),
		})
		if change.Started {
			t.Fatal("未达到阈值前不应进入异常")
		}
	}
	change := tracker.step("local", model.LayerSnapshot{
		Layer:         "local",
		Degraded:      true,
		Summary:       "bad",
		LastEvaluated: now.Add(10 * time.Second),
	})
	if !change.Started {
		t.Fatal("达到阈值后应进入异常")
	}
}

func TestStateTrackerRestoresOpenEvent(t *testing.T) {
	cfg, err := config.Default()
	if err != nil {
		t.Fatalf("读取默认配置失败: %v", err)
	}
	tracker := newTracker(cfg)
	tracker.restoreOpenEvent(model.Event{
		ID:       42,
		Summary:  "old summary",
		Evidence: "old evidence",
	})
	if !tracker.active || tracker.eventID != 42 {
		t.Fatalf("期望恢复打开事件状态，实际为 %#v", tracker)
	}
	if tracker.badCount != cfg.Thresholds.EnterConsecutive {
		t.Fatalf("恢复后 badCount 应达到进入阈值，实际为 %d", tracker.badCount)
	}
}
