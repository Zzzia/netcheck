package probe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type DownloadResult struct {
	Target         string
	DurationMs     float64
	BytesRead      int64
	ThroughputMbps float64
	StatusCode     int
	Success        bool
	Err            error
}

func DownloadOnce(ctx context.Context, client *http.Client, target string, sampleBytes int64) DownloadResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return DownloadResult{
			Target:  target,
			Success: false,
			Err:     fmt.Errorf("create download request failed: %w", err),
		}
	}
	if sampleBytes > 0 {
		req.Header.Set("Range", "bytes=0-"+strconv.FormatInt(sampleBytes-1, 10))
	}
	// Some mirrors reject an empty User-Agent or compressed responses.
	req.Header.Set("User-Agent", "netcheck/0.1")
	req.Header.Set("Accept-Encoding", "identity")
	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return DownloadResult{
			Target:  target,
			Success: false,
			Err:     fmt.Errorf("download request failed: %w", err),
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 && resp.StatusCode != http.StatusPartialContent {
		return DownloadResult{
			Target:     target,
			StatusCode: resp.StatusCode,
			Success:    false,
			Err:        fmt.Errorf("download returned non-success status code: %d", resp.StatusCode),
		}
	}
	limited := io.LimitReader(resp.Body, sampleBytes)
	written, copyErr := io.Copy(io.Discard, limited)
	elapsed := time.Since(started)
	if copyErr != nil {
		return DownloadResult{
			Target:     target,
			StatusCode: resp.StatusCode,
			BytesRead:  written,
			DurationMs: float64(elapsed.Microseconds()) / 1000.0,
			Success:    false,
			Err:        fmt.Errorf("read download data failed: %w", copyErr),
		}
	}
	if written <= 0 || elapsed <= 0 {
		return DownloadResult{
			Target:     target,
			StatusCode: resp.StatusCode,
			BytesRead:  written,
			DurationMs: float64(elapsed.Microseconds()) / 1000.0,
			Success:    false,
			Err:        fmt.Errorf("download produced no valid bytes: bytes=%d", written),
		}
	}
	throughputMbps := (float64(written) * 8.0 / 1_000_000.0) / elapsed.Seconds()
	return DownloadResult{
		Target:         target,
		StatusCode:     resp.StatusCode,
		BytesRead:      written,
		DurationMs:     float64(elapsed.Microseconds()) / 1000.0,
		ThroughputMbps: throughputMbps,
		Success:        true,
	}
}
