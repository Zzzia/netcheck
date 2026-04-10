package probe

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var pingRTTRE = regexp.MustCompile(`time[=<]([\d.]+)\s*ms`)

type PingResult struct {
	Target    string
	LatencyMs float64
	Success   bool
	Err       error
}

func PingOnce(parent context.Context, target string, timeout time.Duration) PingResult {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	command := exec.CommandContext(ctx, "ping", "-n", "-c", "1", target)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return PingResult{
			Target:  target,
			Success: false,
			Err:     fmt.Errorf("ping 超时: %w", ctx.Err()),
		}
	}
	if err != nil {
		return PingResult{
			Target:  target,
			Success: false,
			Err:     fmt.Errorf("ping 失败: %w, 输出: %s", err, strings.TrimSpace(string(output))),
		}
	}
	matches := pingRTTRE.FindStringSubmatch(string(output))
	if len(matches) != 2 {
		return PingResult{
			Target:  target,
			Success: false,
			Err:     fmt.Errorf("无法从 ping 输出解析延迟: %s", strings.TrimSpace(string(output))),
		}
	}
	latencyMs, parseErr := strconv.ParseFloat(matches[1], 64)
	if parseErr != nil {
		return PingResult{
			Target:  target,
			Success: false,
			Err:     fmt.Errorf("解析 ping 延迟失败: %w", parseErr),
		}
	}
	return PingResult{
		Target:    target,
		LatencyMs: latencyMs,
		Success:   true,
	}
}
