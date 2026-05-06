package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"netcheck/internal/config"
	"netcheck/internal/monitor"
	"netcheck/internal/report"
)

const defaultUIAddr = "0.0.0.0:8765"

func Run(args []string) error {
	if len(args) == 0 {
		return runDefault()
	}
	switch args[0] {
	case "monitor":
		return runMonitor(args[1:])
	case "report":
		return runReport(args[1:])
	case "ui":
		return runUI(args[1:])
	case "clear":
		return runClear(args[1:])
	case "init-config":
		return runInitConfig(args[1:])
	case "-h", "--help", "help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("未知命令 %q", args[0])
	}
}

func runDefault() error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 2)
	go func() {
		errCh <- monitor.Run(ctx, cfg)
	}()
	go func() {
		errCh <- report.Serve(ctx, cfg.DBPath, defaultUIAddr, func(actualAddr string) {
			fmt.Println(buildUIReadyMessage(defaultUIAddr, actualAddr))
		})
	}()

	firstErr := <-errCh
	cancel()
	secondErr := <-errCh
	if firstErr != nil && !errors.Is(firstErr, context.Canceled) {
		return firstErr
	}
	if secondErr != nil && !errors.Is(secondErr, context.Canceled) {
		return secondErr
	}
	return nil
}

func runMonitor(args []string) error {
	fs := flag.NewFlagSet("monitor", flag.ContinueOnError)
	configPath := fs.String("config", "", "配置文件路径")
	duration := fs.Duration("duration", 0, "运行时长，默认持续运行直到收到退出信号")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *duration > 0 {
		var innerCancel context.CancelFunc
		ctx, innerCancel = context.WithTimeout(ctx, *duration)
		defer innerCancel()
	}
	return monitor.Run(ctx, cfg)
}

func runReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	configPath := fs.String("config", "", "配置文件路径")
	since := fs.String("since", "24h", "时间范围，例如 24h、7d")
	startRaw := fs.String("start", "", "开始时间，支持 RFC3339 或 2006-01-02T15:04")
	endRaw := fs.String("end", "", "结束时间，支持 RFC3339 或 2006-01-02T15:04")
	output := fs.String("output", "report.html", "报表输出路径")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	start, end, err := resolveReportRange(*since, *startRaw, *endRaw)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("创建报表目录失败: %w", err)
	}
	return report.Generate(cfg.DBPath, start, end, *output)
}

func runUI(args []string) error {
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	configPath := fs.String("config", "", "配置文件路径")
	addr := fs.String("addr", defaultUIAddr, "监听地址")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return report.Serve(ctx, cfg.DBPath, *addr, func(actualAddr string) {
		fmt.Println(buildUIReadyMessage(*addr, actualAddr))
	})
}

func runInitConfig(args []string) error {
	fs := flag.NewFlagSet("init-config", flag.ContinueOnError)
	output := fs.String("output", "", "配置输出路径")
	force := fs.Bool("force", false, "覆盖已有配置")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := config.WriteDefault(*output, *force); err != nil {
		return err
	}
	path := *output
	if path == "" {
		var err error
		path, err = config.DefaultPath()
		if err != nil {
			return err
		}
	}
	fmt.Printf("已写入默认配置: %s\n", path)
	return nil
}

func parseSince(raw string) (time.Duration, error) {
	if raw == "" {
		return 24 * time.Hour, nil
	}
	if raw[len(raw)-1] == 'd' {
		var days int
		if _, err := fmt.Sscanf(raw, "%dd", &days); err != nil || days <= 0 {
			return 0, fmt.Errorf("无法解析 since 参数: %s", raw)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("无法解析 since 参数: %w", err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("since 参数必须大于 0")
	}
	return duration, nil
}

func resolveReportRange(sinceRaw, startRaw, endRaw string) (time.Time, time.Time, error) {
	if startRaw == "" && endRaw == "" {
		window, err := parseSince(sinceRaw)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		end := time.Now()
		return end.Add(-window), end, nil
	}
	if startRaw == "" || endRaw == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("使用自定义时间时，必须同时提供 --start 和 --end")
	}
	start, err := parseAbsoluteTime(startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("解析开始时间失败: %w", err)
	}
	end, err := parseAbsoluteTime(endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("解析结束时间失败: %w", err)
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("结束时间不能早于开始时间")
	}
	return start, end, nil
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

func printUsage() {
	fmt.Println(`netcheck 用法:

  netcheck monitor [--config path] [--duration 5m]
  netcheck report [--config path] [--since 24h] [--start 2026-04-10T09:00 --end 2026-04-10T18:00] [--output report.html]
  netcheck ui [--config path] [--addr 0.0.0.0:8765]
  netcheck clear [--config path]
  netcheck init-config [--output path] [--force]`)
}

func buildUIReadyMessage(requestedAddr, actualAddr string) string {
	requestedDisplay := normalizeAnnounceAddr(requestedAddr)
	actualDisplay := normalizeAnnounceAddr(actualAddr)
	localAccessURL := buildLocalAccessURL(actualDisplay)

	if requestedDisplay == actualDisplay {
		if localAccessURL == "" {
			return fmt.Sprintf("netcheck UI 已启动，监听地址: %s", actualDisplay)
		}
		return fmt.Sprintf("netcheck UI 已启动，监听地址: %s，本机访问: %s", actualDisplay, localAccessURL)
	}
	if localAccessURL == "" {
		return fmt.Sprintf("netcheck UI 已启动，默认监听地址 %s 已被占用，已切换到: %s", requestedDisplay, actualDisplay)
	}
	return fmt.Sprintf(
		"netcheck UI 已启动，默认监听地址 %s 已被占用，已切换到: %s，本机访问: %s",
		requestedDisplay,
		actualDisplay,
		localAccessURL,
	)
}

func normalizeAnnounceAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "::" {
		host = "0.0.0.0"
	}
	return net.JoinHostPort(host, port)
}

func buildLocalAccessURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	switch host {
	case "", "0.0.0.0":
		return fmt.Sprintf("http://127.0.0.1:%s", port)
	case "::":
		return fmt.Sprintf("http://[::1]:%s", port)
	default:
		return fmt.Sprintf("http://%s", net.JoinHostPort(host, port))
	}
}
