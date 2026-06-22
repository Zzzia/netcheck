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
	"strings"
	"syscall"
	"time"

	"netcheck/internal/config"
	"netcheck/internal/i18n"
	"netcheck/internal/monitor"
	"netcheck/internal/report"
)

const defaultUIAddr = "0.0.0.0:8765"

func Run(args []string) error {
	lang, filteredArgs, err := extractGlobalLang(args)
	if err != nil {
		return err
	}
	localizer := i18n.New(lang)
	args = filteredArgs
	if len(args) == 0 {
		return runDefaultForLang(localizer.Lang())
	}
	switch args[0] {
	case "monitor":
		return runMonitorForLang(args[1:], localizer.Lang())
	case "report":
		return runReportForLang(args[1:], localizer.Lang())
	case "ui":
		return runUIForLang(args[1:], localizer.Lang())
	case "clear":
		return runClearForLang(args[1:], localizer.Lang())
	case "init-config":
		return runInitConfigForLang(args[1:], localizer.Lang())
	case "-h", "--help", "help":
		printUsage(localizer)
		return nil
	default:
		return fmt.Errorf("%s %q", localizer.T("cli.unknown_command"), args[0])
	}
}

func runDefault() error {
	return runDefaultForLang(i18n.English)
}

func runDefaultForLang(lang i18n.Lang) error {
	localizer := i18n.New(lang)
	cfg, err := config.Load("")
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 2)
	go func() {
		errCh <- monitor.RunForLang(ctx, cfg, localizer.Lang())
	}()
	go func() {
		errCh <- report.ServeForLang(ctx, cfg.DBPath, defaultUIAddr, localizer.Lang(), func(actualAddr string) {
			fmt.Println(buildUIReadyMessageForLang(defaultUIAddr, actualAddr, localizer.Lang()))
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
	return runMonitorForLang(args, i18n.English)
}

func runMonitorForLang(args []string, lang i18n.Lang) error {
	localizer := i18n.New(lang)
	fs := flag.NewFlagSet("monitor", flag.ContinueOnError)
	configPath := fs.String("config", "", localizer.T("cli.flag.config"))
	duration := fs.Duration("duration", 0, localizer.T("cli.flag.duration"))
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
	return monitor.RunForLang(ctx, cfg, localizer.Lang())
}

func runReport(args []string) error {
	return runReportForLang(args, i18n.English)
}

func runReportForLang(args []string, lang i18n.Lang) error {
	localizer := i18n.New(lang)
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	configPath := fs.String("config", "", localizer.T("cli.flag.config"))
	since := fs.String("since", "24h", localizer.T("cli.flag.since"))
	startRaw := fs.String("start", "", localizer.T("cli.flag.start"))
	endRaw := fs.String("end", "", localizer.T("cli.flag.end"))
	output := fs.String("output", "report.html", localizer.T("cli.flag.output"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	start, end, err := resolveReportRangeForLang(*since, *startRaw, *endRaw, localizer.Lang())
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf(localizer.T("cli.error.create_report_dir"), err)
	}
	return report.GenerateForLang(cfg.DBPath, start, end, *output, localizer.Lang())
}

func runUI(args []string) error {
	return runUIForLang(args, i18n.English)
}

func runUIForLang(args []string, lang i18n.Lang) error {
	localizer := i18n.New(lang)
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	configPath := fs.String("config", "", localizer.T("cli.flag.config"))
	addr := fs.String("addr", defaultUIAddr, localizer.T("cli.flag.addr"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return report.ServeForLang(ctx, cfg.DBPath, *addr, localizer.Lang(), func(actualAddr string) {
		fmt.Println(buildUIReadyMessageForLang(*addr, actualAddr, localizer.Lang()))
	})
}

func runInitConfig(args []string) error {
	return runInitConfigForLang(args, i18n.English)
}

func runInitConfigForLang(args []string, lang i18n.Lang) error {
	localizer := i18n.New(lang)
	fs := flag.NewFlagSet("init-config", flag.ContinueOnError)
	output := fs.String("output", "", localizer.T("cli.flag.output"))
	force := fs.Bool("force", false, localizer.T("cli.flag.force"))
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
	fmt.Printf(localizer.T("cli.config_written"), path)
	return nil
}

func parseSince(raw string) (time.Duration, error) {
	return parseSinceForLang(raw, i18n.English)
}

func parseSinceForLang(raw string, lang i18n.Lang) (time.Duration, error) {
	localizer := i18n.New(lang)
	if raw == "" {
		return 24 * time.Hour, nil
	}
	if raw[len(raw)-1] == 'd' {
		var days int
		if _, err := fmt.Sscanf(raw, "%dd", &days); err != nil || days <= 0 {
			return 0, fmt.Errorf(localizer.T("cli.error.invalid_since"), raw)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf(localizer.T("cli.error.invalid_since_duration"), err)
	}
	if duration <= 0 {
		return 0, errors.New(localizer.T("cli.error.positive_since"))
	}
	return duration, nil
}

func resolveReportRange(sinceRaw, startRaw, endRaw string) (time.Time, time.Time, error) {
	return resolveReportRangeForLang(sinceRaw, startRaw, endRaw, i18n.English)
}

func resolveReportRangeForLang(sinceRaw, startRaw, endRaw string, lang i18n.Lang) (time.Time, time.Time, error) {
	localizer := i18n.New(lang)
	if startRaw == "" && endRaw == "" {
		window, err := parseSinceForLang(sinceRaw, localizer.Lang())
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		end := time.Now()
		return end.Add(-window), end, nil
	}
	if startRaw == "" || endRaw == "" {
		return time.Time{}, time.Time{}, errors.New(localizer.T("cli.error.custom_range_pair"))
	}
	start, err := parseAbsoluteTimeForLang(startRaw, localizer.Lang())
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf(localizer.T("cli.error.parse_start"), err)
	}
	end, err := parseAbsoluteTimeForLang(endRaw, localizer.Lang())
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf(localizer.T("cli.error.parse_end"), err)
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, errors.New(localizer.T("cli.error.end_before_start"))
	}
	return start, end, nil
}

func parseAbsoluteTime(raw string) (time.Time, error) {
	return parseAbsoluteTimeForLang(raw, i18n.English)
}

func parseAbsoluteTimeForLang(raw string, lang i18n.Lang) (time.Time, error) {
	localizer := i18n.New(lang)
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
	return time.Time{}, fmt.Errorf(localizer.T("cli.error.unsupported_time_format"), raw)
}

func printUsage(localizer i18n.Localizer) {
	fmt.Println(localizer.T("app.usage"))
}

func buildUIReadyMessage(requestedAddr, actualAddr string) string {
	return buildUIReadyMessageForLang(requestedAddr, actualAddr, i18n.English)
}

func buildUIReadyMessageForLang(requestedAddr, actualAddr string, lang i18n.Lang) string {
	localizer := i18n.New(lang)
	requestedDisplay := normalizeAnnounceAddr(requestedAddr)
	actualDisplay := normalizeAnnounceAddr(actualAddr)
	localAccessURL := buildLocalAccessURL(actualDisplay)

	if requestedDisplay == actualDisplay {
		if localAccessURL == "" {
			return fmt.Sprintf(localizer.T("cli.ui_ready"), actualDisplay)
		}
		return fmt.Sprintf(localizer.T("cli.ui_ready_local"), actualDisplay, localAccessURL)
	}
	if localAccessURL == "" {
		return fmt.Sprintf(localizer.T("cli.ui_port_changed"), requestedDisplay, actualDisplay)
	}
	return fmt.Sprintf(
		localizer.T("cli.ui_port_changed_local"),
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

func extractGlobalLang(args []string) (i18n.Lang, []string, error) {
	lang := i18n.FromEnv()
	filtered := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--lang" {
			if index+1 >= len(args) {
				return lang, nil, errors.New(i18n.New(lang).T("cli.error.missing_lang_value"))
			}
			lang = i18n.Parse(args[index+1])
			index++
			continue
		}
		if strings.HasPrefix(arg, "--lang=") {
			lang = i18n.Parse(strings.TrimPrefix(arg, "--lang="))
			continue
		}
		filtered = append(filtered, arg)
	}
	return lang, filtered, nil
}
