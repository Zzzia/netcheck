package monitor

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"netcheck/internal/config"
	"netcheck/internal/i18n"
	"netcheck/internal/model"
	"netcheck/internal/probe"
	"netcheck/internal/storage"
)

type monitorSample struct {
	sample model.Sample
}

func Run(ctx context.Context, cfg config.Config) error {
	return RunForLang(ctx, cfg, i18n.English)
}

func RunForLang(ctx context.Context, cfg config.Config, lang i18n.Lang) error {
	localizer := i18n.New(lang)
	store, err := storage.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := printStartupSummary(cfg, localizer); err != nil {
		return err
	}

	httpClient := &http.Client{
		Timeout: time.Duration(cfg.Sampling.HTTPTimeoutMs) * time.Millisecond,
	}
	sampleCh := make(chan monitorSample, 256)
	errCh := make(chan error, 32)

	var gateway struct {
		sync.RWMutex
		value string
	}
	refreshGateway := func() {
		value, gatewayErr := probe.DefaultGateway()
		if gatewayErr != nil {
			errCh <- gatewayErr
			return
		}
		gateway.Lock()
		gateway.value = value
		gateway.Unlock()
	}
	refreshGateway()

	startTask(ctx, time.Second*30, func(taskCtx context.Context) {
		refreshGateway()
	})
	startTask(ctx, time.Duration(cfg.Sampling.GatewayIntervalSec)*time.Second, func(taskCtx context.Context) {
		gateway.RLock()
		target := gateway.value
		gateway.RUnlock()
		if strings.TrimSpace(target) == "" {
			refreshGateway()
			return
		}
		result := probe.PingOnce(taskCtx, target, time.Duration(cfg.Sampling.PingTimeoutSec)*time.Second)
		emitSample(taskCtx, sampleCh, model.Sample{
			Timestamp: time.Now(),
			Layer:     "local",
			Metric:    "ping",
			Target:    result.Target,
			Success:   result.Success,
			LatencyMs: result.LatencyMs,
			Detail:    "gateway",
			ErrorText: errString(result.Err),
		})
	})
	startRemoteLatencyTask(ctx, cfg.Sampling.DomesticLatencyIntervalSec, cfg.Targets.DomesticLatency, "domestic", cfg, sampleCh)
	startRemoteLatencyTask(ctx, cfg.Sampling.InternationalLatencySeconds, cfg.Targets.InternationalLatency, "international", cfg, sampleCh)
	startDownloadTask(ctx, 5*time.Second, cfg.Sampling.DomesticDownloadIntervalSec, cfg.Targets.DomesticDownloads, "domestic", cfg.Sampling.DomesticDownloadBytes, httpClient, sampleCh, localizer)
	startDownloadTask(ctx, 45*time.Second, cfg.Sampling.InternationalDownloadSec, cfg.Targets.InternationalDownloads, "international", cfg.Sampling.InternationalDownloadBytes, httpClient, sampleCh, localizer)

	trackers := map[string]*stateTracker{
		"local":         trackerPtr(newTracker(cfg)),
		"domestic":      trackerPtr(newTracker(cfg)),
		"international": trackerPtr(newTracker(cfg)),
	}
	if err := restoreOpenEvents(store, trackers); err != nil {
		return err
	}
	lastEvaluated := map[string]time.Time{}
	stateWindows := newWindows()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	for {
		select {
		case <-ctx.Done():
			now := time.Now()
			for _, tracker := range trackers {
				if tracker.eventID != 0 {
					if err := store.EndEvent(tracker.eventID, now); err != nil {
						return err
					}
					tracker.eventID = 0
				}
			}
			return nil
		case err := <-errCh:
			fmt.Printf(localizer.T("monitor.background_error"), time.Now().Format(time.RFC3339), err)
		case item := <-sampleCh:
			if err := store.InsertSample(item.sample); err != nil {
				return err
			}
			stateWindows.add(item.sample)
			if shouldEvaluateLayer(item.sample.Layer, item.sample.Timestamp, lastEvaluated, cfg) {
				lastEvaluated[item.sample.Layer] = item.sample.Timestamp
				snapshots := stateWindows.evaluateForLang(cfg, item.sample.Timestamp, localizer)
				snapshot := snapshots[item.sample.Layer]
				change := trackers[item.sample.Layer].step(item.sample.Layer, snapshot)
				if change.Started {
					eventID, beginErr := store.BeginEvent(item.sample.Layer, "degraded", change.Summary, change.Evidence, change.Timestamp)
					if beginErr != nil {
						return beginErr
					}
					trackers[item.sample.Layer].eventID = eventID
					if cfg.Alerts.PrintStateChanges {
						fmt.Printf(localizer.T("monitor.degraded_start"), change.Timestamp.Format(time.RFC3339), layerDisplayName(item.sample.Layer, localizer), change.Summary, safeEvidence(change.Evidence, localizer))
					}
				}
				if change.Resolved {
					if trackers[item.sample.Layer].eventID != 0 {
						if endErr := store.EndEvent(trackers[item.sample.Layer].eventID, change.Timestamp); endErr != nil {
							return endErr
						}
					}
					trackers[item.sample.Layer].eventID = 0
					if cfg.Alerts.PrintStateChanges {
						fmt.Printf(localizer.T("monitor.degraded_resolved"), change.Timestamp.Format(time.RFC3339), layerDisplayName(item.sample.Layer, localizer), change.Summary)
					}
				}
			}
		}
	}
}

func restoreOpenEvents(store *storage.Store, trackers map[string]*stateTracker) error {
	events, err := store.LoadOpenEvents()
	if err != nil {
		return err
	}
	for _, event := range events {
		tracker := trackers[event.Name]
		if tracker == nil {
			continue
		}
		if tracker.eventID != 0 {
			continue
		}
		tracker.restoreOpenEvent(event)
	}
	return nil
}

func startRemoteLatencyTask(
	ctx context.Context,
	intervalSec int,
	targets []config.Target,
	layer string,
	cfg config.Config,
	sampleCh chan<- monitorSample,
) {
	startTask(ctx, time.Duration(intervalSec)*time.Second, func(taskCtx context.Context) {
		httpClient := &http.Client{
			Timeout: time.Duration(cfg.Sampling.HTTPTimeoutMs) * time.Millisecond,
		}
		timeout := time.Duration(cfg.Sampling.ConnectTimeoutMs) * time.Millisecond
		for _, target := range targets {
			result := probe.TCPConnectOnce(taskCtx, target.Address, timeout)
			detail := target.Address
			if strings.TrimSpace(target.URL) != "" {
				result = probe.HTTPLatencyOnce(taskCtx, httpClient, target.URL)
				detail = target.URL
			}
			emitSample(taskCtx, sampleCh, model.Sample{
				Timestamp: time.Now(),
				Layer:     layer,
				Metric:    "tcp_connect",
				Target:    target.Name,
				Success:   result.Success,
				LatencyMs: result.LatencyMs,
				Detail:    detail,
				ErrorText: errString(result.Err),
			})
		}
	})
}

func startDownloadTask(
	ctx context.Context,
	initialDelay time.Duration,
	intervalSec int,
	targets []config.DownloadItem,
	layer string,
	sampleBytes int64,
	client *http.Client,
	sampleCh chan<- monitorSample,
	localizer i18n.Localizer,
) {
	startTaskWithDelay(ctx, initialDelay, time.Duration(intervalSec)*time.Second, func(taskCtx context.Context) {
		for _, target := range targets {
			result := probe.DownloadOnce(taskCtx, client, target.URL, sampleBytes)
			printDownloadResult(layer, result, localizer)
			emitSample(taskCtx, sampleCh, model.Sample{
				Timestamp: time.Now(),
				Layer:     layer,
				Metric:    "download",
				Target:    target.Name,
				Success:   result.Success,
				LatencyMs: result.DurationMs,
				Value:     result.ThroughputMbps,
				BytesRX:   result.BytesRead,
				Detail:    fmt.Sprintf("status=%d url=%s", result.StatusCode, target.URL),
				ErrorText: errString(result.Err),
			})
		}
	})
}

func startTask(ctx context.Context, interval time.Duration, job func(context.Context)) {
	startTaskWithDelay(ctx, 0, interval, job)
}

func startTaskWithDelay(ctx context.Context, initialDelay, interval time.Duration, job func(context.Context)) {
	sem := make(chan struct{}, 1)
	run := func() {
		select {
		case sem <- struct{}{}:
		default:
			return
		}
		go func() {
			defer func() { <-sem }()
			job(ctx)
		}()
	}
	if initialDelay == 0 {
		run()
	} else {
		timer := time.NewTimer(initialDelay)
		go func() {
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				run()
			}
		}()
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

func printStartupSummary(cfg config.Config, localizer i18n.Localizer) error {
	domesticBudgetMB := float64(cfg.Sampling.DomesticDownloadBytes*int64((24*time.Hour)/(time.Duration(cfg.Sampling.DomesticDownloadIntervalSec)*time.Second))*int64(len(cfg.Targets.DomesticDownloads))) / 1024.0 / 1024.0
	internationalBudgetMB := float64(cfg.Sampling.InternationalDownloadBytes*int64((24*time.Hour)/(time.Duration(cfg.Sampling.InternationalDownloadSec)*time.Second))*int64(len(cfg.Targets.InternationalDownloads))) / 1024.0 / 1024.0
	fmt.Printf(localizer.T("monitor.started"), cfg.DBPath)
	fmt.Printf(localizer.T("monitor.sampling"),
		cfg.Sampling.GatewayIntervalSec,
		cfg.Sampling.DomesticDownloadIntervalSec,
		cfg.Sampling.InternationalDownloadSec,
	)
	fmt.Printf(localizer.T("monitor.download_budget"), domesticBudgetMB, internationalBudgetMB)
	return nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func trackerPtr(value stateTracker) *stateTracker {
	return &value
}

func safeEvidence(value string, localizer i18n.Localizer) string {
	if strings.TrimSpace(value) == "" {
		return localizer.T("monitor.no_evidence")
	}
	return value
}

func emitSample(ctx context.Context, sampleCh chan<- monitorSample, sample model.Sample) {
	select {
	case <-ctx.Done():
	case sampleCh <- monitorSample{sample: sample}:
	}
}

func shouldEvaluateLayer(layer string, now time.Time, lastEvaluated map[string]time.Time, cfg config.Config) bool {
	interval := evaluationInterval(layer, cfg)
	last := lastEvaluated[layer]
	if last.IsZero() {
		return true
	}
	return now.Sub(last) >= interval
}

func evaluationInterval(layer string, cfg config.Config) time.Duration {
	switch layer {
	case "local":
		return 10 * time.Second
	case "domestic":
		return minDuration(
			time.Duration(cfg.Sampling.DomesticLatencyIntervalSec)*time.Second,
			time.Duration(cfg.Sampling.DomesticDownloadIntervalSec)*time.Second,
		)
	case "international":
		return minDuration(
			time.Duration(cfg.Sampling.InternationalLatencySeconds)*time.Second,
			time.Duration(cfg.Sampling.InternationalDownloadSec)*time.Second,
		)
	default:
		return 10 * time.Second
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func printDownloadResult(layer string, result probe.DownloadResult, localizer i18n.Localizer) {
	label := layerDisplayName(layer, localizer)
	now := time.Now().Format(time.RFC3339)
	if !result.Success {
		fmt.Printf(localizer.T("monitor.speedtest_failed"), now, label, errString(result.Err))
		return
	}
	fmt.Printf(localizer.T("monitor.speedtest"), now, label, result.ThroughputMbps)
}
