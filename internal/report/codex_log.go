package report

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxCodexWindow = 24 * time.Hour

var (
	logPrefixPattern      = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)\s+(TRACE|DEBUG|INFO|WARN|ERROR)\s+`)
	trueRetryPattern      = regexp.MustCompile(`^\S+\s+WARN\s+.*: codex_core::session::turn: stream disconnected - retrying sampling request \((\d+)/(\d+) in ([0-9.]+)(ms|s)\)`)
	turnIDPattern         = regexp.MustCompile(`turn\.id=([0-9a-f-]{36})`)
	threadIDPattern       = regexp.MustCompile(`thread_id=([0-9a-f-]{36})`)
	threadDotIDPattern    = regexp.MustCompile(`thread\.id=([0-9a-f-]{36})`)
	modelPattern          = regexp.MustCompile(`model=(gpt-[^}\s]+)`)
	idPattern             = regexp.MustCompile(`019[0-9a-f-]{33}`)
	uuidPattern           = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f-]{27}`)
	urlPattern            = regexp.MustCompile(`https?://[^\s)]+`)
	skippedPattern        = regexp.MustCompile(`skipped=\d+`)
	turnIdlePattern       = regexp.MustCompile(`time\.idle=([0-9.]+)(µs|us|ms|s|m)`)
	inputTokenPattern     = regexp.MustCompile(`codex\.turn\.token_usage\.input_tokens=(\d+)`)
	outputTokenPattern    = regexp.MustCompile(`codex\.turn\.token_usage\.output_tokens=(\d+)`)
	reasoningTokenPattern = regexp.MustCompile(`codex\.turn\.token_usage\.reasoning_output_tokens=(\d+)`)
)

type codexParsedLine struct {
	ts       time.Time
	level    string
	lineNo   int
	raw      string
	message  string
	threadID string
	turnID   string
	model    string
}

type codexTurnStats struct {
	durationSec  float64
	inputTokens  int
	outputTokens int
	reasonTokens int
}

func buildCodexReport(start, end time.Time) codexReport {
	return buildCodexReportFromLog(defaultCodexLogPath(), start, end)
}

func buildCodexReportFromLog(logPath string, start, end time.Time) codexReport {
	codexStart := start
	clamped := false
	if end.Sub(start) > maxCodexWindow {
		codexStart = end.Add(-maxCodexWindow)
		clamped = true
	}
	report := codexReport{
		LogPath:    logPath,
		RangeLabel: fmt.Sprintf("%s ~ %s", codexStart.Local().Format("2006-01-02 15:04"), end.Local().Format("2006-01-02 15:04")),
		RangeStart: codexStart.Local().Format("2006-01-02 15:04"),
		RangeEnd:   end.Local().Format("2006-01-02 15:04"),
		Clamped:    clamped,
	}
	if strings.TrimSpace(logPath) == "" {
		report.Error = "Codex 日志路径为空"
		return report
	}
	reader, err := openCodexLogWindow(logPath, codexStart)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			report.Error = "未检测到 Codex 日志"
		} else {
			report.Error = fmt.Sprintf("读取 Codex 日志失败: %v", err)
		}
		return report
	}
	defer reader.Close()

	parsed, err := parseCodexLog(reader, codexStart, end)
	if err != nil {
		report.Error = err.Error()
		return report
	}
	return parsed.finalize(report, codexStart, end)
}

func defaultCodexLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex", "log", "codex-tui.log")
}

type codexParseResult struct {
	streamRequests int
	completedTurns map[string]codexTurnStats
	events         []codexEventRow
	other          map[string]*codexIssueRow
	noise          map[string]int
	timeline       map[time.Time]*codexTimelinePoint
	retryTurns     map[string]struct{}
	maxRetry       string
	counts         map[string]int
}

func parseCodexLog(reader io.Reader, start, end time.Time) (codexParseResult, error) {
	result := codexParseResult{
		completedTurns: map[string]codexTurnStats{},
		other:          map[string]*codexIssueRow{},
		noise:          map[string]int{},
		timeline:       map[time.Time]*codexTimelinePoint{},
		retryTurns:     map[string]struct{}{},
		counts:         map[string]int{},
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	bucket := chooseChartBucket(end.Sub(start))
	for lineNo := 1; scanner.Scan(); lineNo++ {
		item, ok := parseCodexLine(scanner.Text(), lineNo)
		if !ok || item.ts.Before(start) || item.ts.After(end) {
			continue
		}
		if isStreamClose(item.raw) {
			result.streamRequests++
		}
		if duration, ok := parseTurnDuration(item.raw); ok && item.turnID != "" {
			result.completedTurns[item.turnID] = codexTurnStats{
				durationSec:  duration,
				inputTokens:  firstIntMatch(inputTokenPattern, item.raw),
				outputTokens: firstIntMatch(outputTokenPattern, item.raw),
				reasonTokens: firstIntMatch(reasoningTokenPattern, item.raw),
			}
			result.addTimelineCompletedTurn(item.ts.Local().Truncate(bucket.Duration))
		}
		if item.level != "WARN" && item.level != "ERROR" {
			continue
		}
		classification := classifyCodexLine(item)
		if classification.kind == "noise" {
			result.noise[classification.label]++
			continue
		}
		result.counts[classification.kind]++
		if !isNetworkCodexKind(classification.kind) {
			result.addOtherEvent(item, classification)
			continue
		}
		event := buildCodexEventRow(item, classification)
		if event.Kind == "stream_retry" {
			if item.turnID != "" {
				result.retryTurns[item.turnID] = struct{}{}
			}
			if event.Attempt > result.maxRetry {
				result.maxRetry = event.Attempt
			}
		}
		result.events = append(result.events, event)
		result.addTimelineEvent(item.ts.Local().Truncate(bucket.Duration), event.Kind)
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("扫描 Codex 日志失败: %w", err)
	}
	return result, nil
}

func parseCodexLine(raw string, lineNo int) (codexParsedLine, bool) {
	match := logPrefixPattern.FindStringSubmatch(raw)
	if match == nil {
		return codexParsedLine{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, match[1])
	if err != nil {
		return codexParsedLine{}, false
	}
	return codexParsedLine{
		ts:       ts,
		level:    match[2],
		lineNo:   lineNo,
		raw:      raw,
		message:  normalizeCodexMessage(raw),
		threadID: firstStringMatch(threadIDPattern, raw, firstStringMatch(threadDotIDPattern, raw, "")),
		turnID:   firstStringMatch(turnIDPattern, raw, ""),
		model:    firstStringMatch(modelPattern, raw, ""),
	}, true
}

func classifyCodexLine(item codexParsedLine) codexLineClass {
	if match := trueRetryPattern.FindStringSubmatch(item.raw); match != nil {
		return codexLineClass{kind: "stream_retry", label: "响应流断开重试", attempt: match[1] + "/" + match[2], backoff: match[3] + match[4]}
	}
	lower := strings.ToLower(item.raw)
	switch {
	case strings.Contains(lower, "write_stdin failed") || strings.Contains(lower, "apply_patch verification failed"):
		return codexLineClass{kind: "tool_error", label: "本地工具错误"}
	case strings.Contains(lower, "failed to record rollout items"):
		return codexLineClass{kind: "rollout_record_error", label: "会话记录错误"}
	case strings.Contains(lower, "request failed with status 403") || strings.Contains(lower, "403 forbidden"):
		return codexLineClass{kind: "apps_or_tool_suggestion_403", label: "Apps/工具建议 403"}
	case containsAny(lower, []string{"timed out", "timeout", "deadline", "dns", "tls", "503 service unavailable", "502 bad gateway", "504 gateway timeout", "connection reset", "error sending request", "rate limit"}):
		return codexLineClass{kind: "network_candidate", label: "网络错误"}
	case isCodexNoise(lower):
		return codexLineClass{kind: "noise", label: noiseName(lower)}
	case item.level == "ERROR":
		return codexLineClass{kind: "unknown_error", label: "未知错误"}
	default:
		return codexLineClass{kind: "unknown_warning", label: "未知警告"}
	}
}

type codexLineClass struct {
	kind    string
	label   string
	attempt string
	backoff string
}

func buildCodexEventRow(item codexParsedLine, class codexLineClass) codexEventRow {
	return codexEventRow{
		Kind:      class.kind,
		KindLabel: class.label,
		Level:     item.level,
		Time:      item.ts.Local().Format("2006-01-02 15:04:05"),
		Ts:        item.ts.Local().Format(time.RFC3339),
		Line:      item.lineNo,
		Model:     item.model,
		ThreadID:  item.threadID,
		TurnID:    item.turnID,
		Attempt:   class.attempt,
		Backoff:   class.backoff,
		Summary:   item.message,
		Evidence:  truncateText(item.raw, 360),
	}
}

func (r *codexParseResult) finalize(report codexReport, start, end time.Time) codexReport {
	events := r.events
	sort.Slice(events, func(i, j int) bool { return events[i].Ts > events[j].Ts })
	for index := range events {
		if events[index].TurnID == "" {
			continue
		}
		stats, ok := r.completedTurns[events[index].TurnID]
		if !ok {
			continue
		}
		events[index].TurnDuration = formatCodexSeconds(stats.durationSec)
		events[index].InputTokens = stats.inputTokens
		events[index].OutputTokens = stats.outputTokens
		events[index].ReasonTokens = stats.reasonTokens
	}
	if len(events) > 80 {
		events = events[:80]
	}
	report.Available = true
	report.Summary = r.summary()
	report.Timeline = r.timelineRows(start, end)
	report.Events = events
	report.NoiseSummary = r.noiseRows()
	report.OtherSummary = r.otherRows()
	return report
}

func (r *codexParseResult) summary() codexSummary {
	noiseWarnings := 0
	for _, count := range r.noise {
		noiseWarnings += count
	}
	completed := len(r.completedTurns)
	retryEvents := r.counts["stream_retry"]
	return codexSummary{
		StreamRequests:        r.streamRequests,
		CompletedTurns:        completed,
		RetryEvents:           retryEvents,
		RetryAffectedTurns:    len(r.retryTurns),
		RetryEventRate:        formatPercentRatio(retryEvents, r.streamRequests),
		RetryAffectedTurnRate: formatPercentRatio(len(r.retryTurns), completed),
		MaxRetryAttempt:       defaultString(r.maxRetry, "0/5"),
		ToolErrors:            r.counts["tool_error"],
		NetworkCandidates:     r.counts["network_candidate"],
		AppsErrors:            r.counts["apps_or_tool_suggestion_403"],
		RolloutErrors:         r.counts["rollout_record_error"],
		UnknownEvents:         r.counts["unknown_warning"] + r.counts["unknown_error"],
		NoiseWarnings:         noiseWarnings,
	}
}

func (r *codexParseResult) addTimelineEvent(bucket time.Time, kind string) {
	point := r.timeline[bucket]
	if point == nil {
		point = &codexTimelinePoint{Ts: bucket.Format(time.RFC3339)}
		r.timeline[bucket] = point
	}
	switch kind {
	case "stream_retry":
		point.StreamRetry++
	case "tool_error":
		point.ToolError++
	case "network_candidate":
		point.NetworkCandidate++
	case "apps_or_tool_suggestion_403":
		point.AppsError++
	case "rollout_record_error":
		point.RolloutError++
	default:
		point.Unknown++
	}
	point.Total++
}

func (r *codexParseResult) addTimelineCompletedTurn(bucket time.Time) {
	point := r.timeline[bucket]
	if point == nil {
		point = &codexTimelinePoint{Ts: bucket.Format(time.RFC3339)}
		r.timeline[bucket] = point
	}
	point.CompletedTurns++
}

func (r *codexParseResult) timelineRows(start, end time.Time) []codexTimelinePoint {
	bucket := chooseChartBucket(end.Sub(start))
	first := start.Local().Truncate(bucket.Duration)
	last := end.Local().Truncate(bucket.Duration)
	if first.After(last) {
		first = last
	}
	var rows []codexTimelinePoint
	for current := first; !current.After(last); current = current.Add(bucket.Duration) {
		point := r.timeline[current]
		if point == nil {
			rows = append(rows, codexTimelinePoint{Ts: current.Format(time.RFC3339)})
			continue
		}
		rows = append(rows, *point)
	}
	return rows
}

func (r *codexParseResult) noiseRows() []codexNoiseRow {
	rows := make([]codexNoiseRow, 0, len(r.noise))
	for name, count := range r.noise {
		rows = append(rows, codexNoiseRow{Name: name, Count: count})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Count > rows[j].Count })
	return rows
}

func isStreamClose(raw string) bool {
	return strings.Contains(raw, "codex_core::client: close") &&
		strings.Contains(raw, "model_client.stream_responses_websocket{") &&
		!strings.Contains(raw, "model_client.websocket_connection{") &&
		!strings.Contains(raw, "ToolCall:")
}

func parseTurnDuration(raw string) (float64, bool) {
	if !strings.Contains(raw, "codex_core::tasks: close") || !strings.Contains(raw, "session_task.turn") || strings.Contains(raw, "ToolCall:") {
		return 0, false
	}
	match := turnIdlePattern.FindStringSubmatch(raw)
	if match == nil {
		return 0, false
	}
	return parseCodexDuration(match[1], match[2]), true
}

func parseCodexDuration(valueRaw, unit string) float64 {
	value, _ := strconv.ParseFloat(valueRaw, 64)
	switch unit {
	case "µs", "us":
		return value / 1_000_000
	case "ms":
		return value / 1000
	case "m":
		return value * 60
	default:
		return value
	}
}

func normalizeCodexMessage(raw string) string {
	message := raw
	if index := strings.LastIndex(message, "}: "); index >= 0 {
		message = message[index+3:]
	} else {
		fields := strings.Fields(message)
		if len(fields) >= 4 {
			message = strings.Join(fields[3:], " ")
		}
	}
	message = urlPattern.ReplaceAllString(message, "<url>")
	message = uuidPattern.ReplaceAllString(message, "<uuid>")
	message = idPattern.ReplaceAllString(message, "<id>")
	message = strings.Join(strings.Fields(message), " ")
	message = strings.TrimSpace(message)
	return truncateText(message, 260)
}

func isCodexNoise(lower string) bool {
	return strings.Contains(lower, "ignoring interface.icon_") ||
		strings.Contains(lower, "ignoring interface.defaultprompt") ||
		strings.Contains(lower, "failed to unwatch") ||
		strings.Contains(lower, "failed to paste image: no image on clipboard")
}

func noiseName(lower string) string {
	switch {
	case strings.Contains(lower, "ignoring interface.icon_"):
		return "插件/Skill 图标配置告警"
	case strings.Contains(lower, "ignoring interface.defaultprompt"):
		return "插件默认 Prompt 配置告警"
	case strings.Contains(lower, "failed to unwatch"):
		return "文件监听释放告警"
	case strings.Contains(lower, "failed to paste image"):
		return "剪贴板图片读取告警"
	default:
		return "其他降噪告警"
	}
}

func formatPercentRatio(numerator, denominator int) string {
	if denominator == 0 {
		return "0%"
	}
	return formatPercent(float64(numerator) / float64(denominator))
}

func formatCodexSeconds(seconds float64) string {
	if seconds <= 0 {
		return ""
	}
	return formatDuration(int64(seconds + 0.5))
}

func firstStringMatch(pattern *regexp.Regexp, raw, fallback string) string {
	match := pattern.FindStringSubmatch(raw)
	if match == nil {
		return fallback
	}
	return match[1]
}

func firstIntMatch(pattern *regexp.Regexp, raw string) int {
	value, _ := strconv.Atoi(firstStringMatch(pattern, raw, "0"))
	return value
}

func containsAny(raw string, items []string) bool {
	for _, item := range items {
		if strings.Contains(raw, item) {
			return true
		}
	}
	return false
}

func truncateText(raw string, limit int) string {
	if len(raw) <= limit {
		return raw
	}
	return raw[:limit] + "..."
}

func defaultString(raw, fallback string) string {
	if raw == "" {
		return fallback
	}
	return raw
}
