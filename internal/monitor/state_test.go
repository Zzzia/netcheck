package monitor

import (
	"testing"
	"time"

	"netcheck/internal/config"
	"netcheck/internal/model"
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
	latencyWindow := rollingWindow{limit: 24}
	for _, latency := range []float64{30, 28, 26, 27} {
		latencyWindow.add(point{Timestamp: time.Now(), Success: true, LatencyMs: latency})
	}
	downloadWindow := rollingWindow{limit: 10}
	for _, throughput := range []float64{2.5, 3.0, 2.8} {
		downloadWindow.add(point{Timestamp: time.Now(), Success: true, Value: throughput})
	}
	snapshot := evaluateRemoteLayer("international", cfg, time.Now(), latencyWindow.stats(), downloadWindow.stats())
	if !snapshot.Degraded {
		t.Fatal("期望国外链路被判定为异常")
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
