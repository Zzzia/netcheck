package report

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func Serve(ctx context.Context, dbPath, addr string, onReady func(string)) error {
	page, err := RenderLivePage()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("/api/report-data", func(w http.ResponseWriter, r *http.Request) {
		start, end, err := parseTimeRange(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload, err := LoadData(dbPath, start, end)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			http.Error(w, fmt.Sprintf("序列化响应失败: %v", err), http.StatusInternalServerError)
			return
		}
	})

	listener, actualAddr, err := listenWithFallback(addr)
	if err != nil {
		return err
	}
	if onReady != nil {
		onReady(actualAddr)
	}
	server := &http.Server{Handler: mux}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("启动 Web UI 失败: %w", err)
	}
	return nil
}

func listenWithFallback(addr string) (net.Listener, string, error) {
	listener, err := net.Listen("tcp", addr)
	if err == nil {
		return listener, normalizeLocalAddr(listener.Addr().String()), nil
	}

	host, portText, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		return nil, "", fmt.Errorf("解析监听地址失败: %w", splitErr)
	}
	port, parseErr := strconv.Atoi(portText)
	if parseErr != nil {
		return nil, "", fmt.Errorf("解析监听端口失败: %w", parseErr)
	}
	if !isAddrInUse(err) {
		return nil, "", fmt.Errorf("监听地址 %s 失败: %w", addr, err)
	}

	for offset := 1; offset <= 20; offset++ {
		candidate := net.JoinHostPort(host, strconv.Itoa(port+offset))
		fallback, listenErr := net.Listen("tcp", candidate)
		if listenErr == nil {
			return fallback, normalizeLocalAddr(fallback.Addr().String()), nil
		}
		if !isAddrInUse(listenErr) {
			return nil, "", fmt.Errorf("监听地址 %s 失败: %w", candidate, listenErr)
		}
	}
	return nil, "", fmt.Errorf("默认地址 %s 已被占用，且后续 20 个端口也不可用", addr)
}

func normalizeLocalAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func isAddrInUse(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "address already in use")
}

func parseTimeRange(r *http.Request) (time.Time, time.Time, error) {
	query := r.URL.Query()
	if startRaw := query.Get("start"); startRaw != "" {
		start, err := parseAbsoluteTime(startRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("解析 start 失败: %w", err)
		}
		endRaw := query.Get("end")
		if endRaw == "" {
			return time.Time{}, time.Time{}, fmt.Errorf("缺少 end 参数")
		}
		end, err := parseAbsoluteTime(endRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("解析 end 失败: %w", err)
		}
		return start, end, nil
	}
	rangeRaw := query.Get("range")
	if rangeRaw == "" {
		rangeRaw = "1h"
	}
	duration, err := parseSince(rangeRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end := time.Now()
	return end.Add(-duration), end, nil
}

func parseSince(raw string) (time.Duration, error) {
	if raw == "" {
		return time.Hour, nil
	}
	if raw[len(raw)-1] == 'd' {
		var days int
		if _, err := fmt.Sscanf(raw, "%dd", &days); err != nil || days <= 0 {
			return 0, fmt.Errorf("无法解析时间范围: %s", raw)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("无法解析时间范围: %w", err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("时间范围必须大于 0")
	}
	return duration, nil
}

func parseAbsoluteTime(raw string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if value, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return value, nil
		}
	}
	return time.Time{}, fmt.Errorf("不支持的时间格式: %s", raw)
}
