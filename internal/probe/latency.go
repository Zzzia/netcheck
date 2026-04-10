package probe

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

type LatencyResult struct {
	Target    string
	LatencyMs float64
	Success   bool
	Err       error
}

func TCPConnectOnce(ctx context.Context, target string, timeout time.Duration) LatencyResult {
	started := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return LatencyResult{
			Target:  target,
			Success: false,
			Err:     fmt.Errorf("连接 %s 失败: %w", target, err),
		}
	}
	defer conn.Close()
	return LatencyResult{
		Target:    target,
		LatencyMs: float64(time.Since(started).Microseconds()) / 1000.0,
		Success:   true,
	}
}

func HTTPLatencyOnce(ctx context.Context, client *http.Client, targetURL string) LatencyResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return LatencyResult{
			Target:  targetURL,
			Success: false,
			Err:     fmt.Errorf("创建 HTTP 延迟请求失败: %w", err),
		}
	}
	req.Header.Set("User-Agent", "netcheck/0.1")
	req.Header.Set("Accept-Encoding", "identity")
	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return LatencyResult{
			Target:  targetURL,
			Success: false,
			Err:     fmt.Errorf("HTTP 延迟请求失败: %w", err),
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return LatencyResult{
			Target:  targetURL,
			Success: false,
			Err:     fmt.Errorf("HTTP 延迟返回状态码异常: %d", resp.StatusCode),
		}
	}
	return LatencyResult{
		Target:    targetURL,
		LatencyMs: float64(time.Since(started).Microseconds()) / 1000.0,
		Success:   true,
	}
}
