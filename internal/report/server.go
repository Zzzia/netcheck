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

	"github.com/Zzzia/netcheck/internal/i18n"
)

func Serve(ctx context.Context, dbPath, addr string, onReady func(string)) error {
	return ServeForLang(ctx, dbPath, addr, i18n.English, onReady)
}

func ServeForLang(ctx context.Context, dbPath, addr string, lang i18n.Lang, onReady func(string)) error {
	localizer := i18n.New(lang)
	page, err := RenderLivePageForLang(localizer.Lang())
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
		requestLocalizer := localizerFromRequest(r)
		start, end, err := parseTimeRangeForLang(r, requestLocalizer.Lang())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload, err := LoadDataForLang(dbPath, start, end, requestLocalizer.Lang())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, payload, requestLocalizer)
	})
	mux.HandleFunc("/api/codex-data", func(w http.ResponseWriter, r *http.Request) {
		requestLocalizer := localizerFromRequest(r)
		start, end, err := parseTimeRangeForLang(r, requestLocalizer.Lang())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, buildCodexReportForLang(start, end, requestLocalizer.Lang()), requestLocalizer)
	})

	listener, actualAddr, err := listenWithFallbackForLang(addr, localizer.Lang())
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
		return fmt.Errorf(localizer.T("report.error.start_ui"), err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, payload any, localizer i18n.Localizer) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, fmt.Sprintf(localizer.T("report.error.encode_response"), err), http.StatusInternalServerError)
		return
	}
}

func listenWithFallback(addr string) (net.Listener, string, error) {
	return listenWithFallbackForLang(addr, i18n.English)
}

func listenWithFallbackForLang(addr string, lang i18n.Lang) (net.Listener, string, error) {
	localizer := i18n.New(lang)
	listener, err := net.Listen("tcp", addr)
	if err == nil {
		return listener, normalizeListenerAddr(listener.Addr().String()), nil
	}

	host, portText, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		return nil, "", fmt.Errorf(localizer.T("report.error.split_addr"), splitErr)
	}
	port, parseErr := strconv.Atoi(portText)
	if parseErr != nil {
		return nil, "", fmt.Errorf(localizer.T("report.error.parse_port"), parseErr)
	}
	if !isAddrInUse(err) {
		return nil, "", fmt.Errorf(localizer.T("report.error.listen_addr"), addr, err)
	}

	for offset := 1; offset <= 20; offset++ {
		candidate := net.JoinHostPort(host, strconv.Itoa(port+offset))
		fallback, listenErr := net.Listen("tcp", candidate)
		if listenErr == nil {
			return fallback, normalizeListenerAddr(fallback.Addr().String()), nil
		}
		if !isAddrInUse(listenErr) {
			return nil, "", fmt.Errorf(localizer.T("report.error.listen_addr"), candidate, listenErr)
		}
	}
	return nil, "", fmt.Errorf(localizer.T("report.error.no_fallback_port"), addr)
}

func normalizeListenerAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" {
		host = "0.0.0.0"
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
	return parseTimeRangeForLang(r, i18n.English)
}

func parseTimeRangeForLang(r *http.Request, lang i18n.Lang) (time.Time, time.Time, error) {
	localizer := i18n.New(lang)
	query := r.URL.Query()
	if startRaw := query.Get("start"); startRaw != "" {
		start, err := parseAbsoluteTimeForLang(startRaw, localizer.Lang())
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf(localizer.T("report.error.parse_start"), err)
		}
		endRaw := query.Get("end")
		if endRaw == "" {
			return time.Time{}, time.Time{}, errors.New(localizer.T("report.error.missing_end"))
		}
		end, err := parseAbsoluteTimeForLang(endRaw, localizer.Lang())
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf(localizer.T("report.error.parse_end"), err)
		}
		return start, end, nil
	}
	rangeRaw := query.Get("range")
	if rangeRaw == "" {
		rangeRaw = "1h"
	}
	duration, err := parseSinceForLang(rangeRaw, localizer.Lang())
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end := time.Now()
	return end.Add(-duration), end, nil
}

func parseSince(raw string) (time.Duration, error) {
	return parseSinceForLang(raw, i18n.English)
}

func parseSinceForLang(raw string, lang i18n.Lang) (time.Duration, error) {
	localizer := i18n.New(lang)
	if raw == "" {
		return time.Hour, nil
	}
	if raw[len(raw)-1] == 'd' {
		var days int
		if _, err := fmt.Sscanf(raw, "%dd", &days); err != nil || days <= 0 {
			return 0, fmt.Errorf(localizer.T("report.error.invalid_range"), raw)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf(localizer.T("report.error.invalid_range_duration"), err)
	}
	if duration <= 0 {
		return 0, errors.New(localizer.T("report.error.positive_range"))
	}
	return duration, nil
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
	return time.Time{}, fmt.Errorf(localizer.T("report.error.unsupported_time_format"), raw)
}

func localizerFromRequest(r *http.Request) i18n.Localizer {
	return i18n.New(i18n.Parse(r.URL.Query().Get("lang")))
}
