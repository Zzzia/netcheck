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
	"strings"
	"time"

	"netcheck/internal/i18n"
)

const maxCodexWindow = 24 * time.Hour

var (
	logPrefixPattern      = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)\s+(TRACE|DEBUG|INFO|WARN|ERROR)\s+`)
	trueRetryPattern      = regexp.MustCompile(`^\S+\s+(?:TRACE|DEBUG|INFO|WARN|ERROR)\s+.*stream disconnected - retrying sampling request \((\d+)/(\d+) in ([0-9.]+)(ms|s)\)`)
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
	target   string
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
	return buildCodexReportForLang(start, end, i18n.English)
}

func buildCodexReportForLang(start, end time.Time, lang i18n.Lang) codexReport {
	source, ok := defaultCodexLogSource()
	if !ok {
		return buildCodexReportFromLogForLang(defaultCodexLogPath(), start, end, lang)
	}
	if source.kind == codexLogSourceSQLite {
		return buildCodexReportFromSQLiteForLang(source.path, start, end, lang)
	}
	return buildCodexReportFromLogForLang(source.path, start, end, lang)
}

func buildCodexReportFromLog(logPath string, start, end time.Time) codexReport {
	return buildCodexReportFromLogForLang(logPath, start, end, i18n.English)
}

func buildCodexReportFromLogForLang(logPath string, start, end time.Time, lang i18n.Lang) codexReport {
	localizer := i18n.New(lang)
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
		report.Error = localizer.T("codex.error.empty_path")
		return report
	}
	reader, exactLineNumbers, err := openCodexLogReader(logPath, codexStart)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			report.Error = localizer.T("codex.error.missing_log")
		} else {
			report.Error = fmt.Sprintf(localizer.T("codex.error.read_log"), err)
		}
		return report
	}
	defer reader.Close()

	parsed, err := parseCodexLogWithLineMode(reader, codexStart, end, exactLineNumbers)
	if err != nil {
		report.Error = fmt.Sprintf(localizer.T("codex.error.scan_log"), err)
		return report
	}
	return parsed.finalizeForLang(report, codexStart, end, localizer)
}

func defaultCodexLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex", "log", "codex-tui.log")
}

type codexParseResult struct {
	modernStreamRequests int
	legacyStreamRequests int
	completedTurns       map[string]codexTurnStats
	sampledTurns         map[string]codexTurnStats
	events               []codexEventRow
	other                map[string]*codexIssueRow
	noise                map[string]int
	timeline             map[time.Time]*codexTimelinePoint
	modernStreamTimeline map[time.Time]int
	legacyStreamTimeline map[time.Time]int
	retryTurns           map[string]struct{}
	maxRetry             string
	counts               map[string]int
}

func parseCodexLog(reader io.Reader, start, end time.Time) (codexParseResult, error) {
	return parseCodexLogWithLineMode(reader, start, end, true)
}

func parseCodexLogWithLineMode(reader io.Reader, start, end time.Time, exactLineNumbers bool) (codexParseResult, error) {
	result := codexParseResult{
		completedTurns:       map[string]codexTurnStats{},
		sampledTurns:         map[string]codexTurnStats{},
		other:                map[string]*codexIssueRow{},
		noise:                map[string]int{},
		timeline:             map[time.Time]*codexTimelinePoint{},
		modernStreamTimeline: map[time.Time]int{},
		legacyStreamTimeline: map[time.Time]int{},
		retryTurns:           map[string]struct{}{},
		counts:               map[string]int{},
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	bucket := chooseChartBucket(end.Sub(start))
	for lineNo := 1; scanner.Scan(); lineNo++ {
		eventLineNo := lineNo
		if !exactLineNumbers {
			eventLineNo = 0
		}
		item, ok := parseCodexLine(scanner.Text(), eventLineNo)
		if !ok || item.ts.Before(start) {
			continue
		}
		if item.ts.After(end) {
			break
		}
		if isModernStreamRequest(item.raw) {
			result.addModernStreamRequest(item.ts.Local().Truncate(bucket.Duration))
		} else if isLegacyStreamRequest(item.raw) {
			result.addLegacyStreamRequest(item.ts.Local().Truncate(bucket.Duration))
		}
		if stats, ok := parseResponseCompletedTurnStats(item.raw); ok && item.turnID != "" {
			result.sampledTurns[item.turnID] = stats
		}
		if stats, ok := parseCompletedTurnStats(item.raw, result.sampledTurns[item.turnID]); ok && item.turnID != "" {
			result.addCompletedTurn(item.turnID, item.ts, bucket.Duration, stats)
		}
		classification := classifyCodexLine(item)
		if classification.kind != "stream_retry" && item.level != "WARN" && item.level != "ERROR" {
			continue
		}
		if classification.kind == "noise" {
			result.noise[classification.labelKey]++
			continue
		}
		result.counts[classification.kind]++
		if !isNetworkCodexKind(classification.kind) {
			result.addOtherEvent(item, classification)
			continue
		}
		event := buildCodexEventRow(item, classification, i18n.New(i18n.English))
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
		return result, err
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
		target:   parseCodexTarget(raw),
		message:  normalizeCodexMessage(raw),
		threadID: firstStringMatch(threadIDPattern, raw, firstStringMatch(threadDotIDPattern, raw, "")),
		turnID:   firstStringMatch(turnIDPattern, raw, ""),
		model:    firstStringMatch(modelPattern, raw, ""),
	}, true
}

func classifyCodexLine(item codexParsedLine) codexLineClass {
	if match := trueRetryPattern.FindStringSubmatch(item.raw); match != nil && isTrueRetryLog(item) {
		return codexLineClass{kind: "stream_retry", labelKey: "codex.kind.stream_retry", attempt: match[1] + "/" + match[2], backoff: match[3] + match[4]}
	}
	lower := strings.ToLower(item.raw)
	switch {
	case strings.Contains(lower, "write_stdin failed") || strings.Contains(lower, "apply_patch verification failed"):
		return codexLineClass{kind: "tool_error", labelKey: "codex.kind.tool_error"}
	case strings.Contains(lower, "failed to record rollout items"):
		return codexLineClass{kind: "rollout_record_error", labelKey: "codex.kind.rollout_record_error"}
	case strings.Contains(lower, "request failed with status 403") || strings.Contains(lower, "403 forbidden"):
		return codexLineClass{kind: "apps_or_tool_suggestion_403", labelKey: "codex.kind.apps_or_tool_403"}
	case containsAny(lower, []string{"timed out", "timeout", "deadline", "dns", "tls", "503 service unavailable", "502 bad gateway", "504 gateway timeout", "connection reset", "error sending request", "rate limit"}):
		return codexLineClass{kind: "network_candidate", labelKey: "codex.kind.network_candidate"}
	case isCodexNoise(lower):
		return codexLineClass{kind: "noise", labelKey: noiseNameKey(lower)}
	case item.level == "ERROR":
		return codexLineClass{kind: "unknown_error", labelKey: "codex.kind.unknown_error"}
	default:
		return codexLineClass{kind: "unknown_warning", labelKey: "codex.kind.unknown_warning"}
	}
}

type codexLineClass struct {
	kind     string
	labelKey string
	attempt  string
	backoff  string
}

func buildCodexEventRow(item codexParsedLine, class codexLineClass, localizer i18n.Localizer) codexEventRow {
	return codexEventRow{
		Kind:         class.kind,
		KindLabel:    localizer.T(class.labelKey),
		KindLabelKey: class.labelKey,
		Level:        item.level,
		Time:         item.ts.Local().Format("2006-01-02 15:04:05"),
		Ts:           item.ts.Local().Format(time.RFC3339),
		Line:         item.lineNo,
		Model:        item.model,
		ThreadID:     item.threadID,
		TurnID:       item.turnID,
		Attempt:      class.attempt,
		Backoff:      class.backoff,
		Summary:      item.message,
		Evidence:     truncateText(item.raw, 360),
	}
}

func (r *codexParseResult) finalize(report codexReport, start, end time.Time) codexReport {
	return r.finalizeForLang(report, start, end, i18n.New(i18n.English))
}

func (r *codexParseResult) finalizeForLang(report codexReport, start, end time.Time, localizer i18n.Localizer) codexReport {
	events := r.events
	sort.Slice(events, func(i, j int) bool { return events[i].Ts > events[j].Ts })
	for index := range events {
		if events[index].KindLabelKey != "" {
			events[index].KindLabel = localizer.T(events[index].KindLabelKey)
		}
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
	report.NoiseSummary = r.noiseRows(localizer)
	report.OtherSummary = r.otherRows(localizer)
	return report
}

func (r *codexParseResult) summary() codexSummary {
	noiseWarnings := 0
	for _, count := range r.noise {
		noiseWarnings += count
	}
	completed := len(r.completedTurns)
	retryEvents := r.counts["stream_retry"]
	streamRequests := r.streamRequests()
	return codexSummary{
		StreamRequests:        streamRequests,
		CompletedTurns:        completed,
		RetryEvents:           retryEvents,
		RetryAffectedTurns:    len(r.retryTurns),
		RetryEventRate:        formatPercentRatio(retryEvents, streamRequests),
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
			rows = append(rows, codexTimelinePoint{
				Ts:             current.Format(time.RFC3339),
				StreamRequests: r.streamRequestsForBucket(current),
			})
			continue
		}
		row := *point
		row.StreamRequests = r.streamRequestsForBucket(current)
		rows = append(rows, row)
	}
	return rows
}

func (r *codexParseResult) noiseRows(localizer i18n.Localizer) []codexNoiseRow {
	rows := make([]codexNoiseRow, 0, len(r.noise))
	for key, count := range r.noise {
		rows = append(rows, codexNoiseRow{Name: localizer.T(key), NameKey: key, Count: count})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Count > rows[j].Count })
	return rows
}

func (r *codexParseResult) addCompletedTurn(turnID string, ts time.Time, bucket time.Duration, stats codexTurnStats) {
	previous, counted := r.completedTurns[turnID]
	if stats == (codexTurnStats{}) && counted {
		stats = previous
	}
	r.completedTurns[turnID] = stats
	if !counted {
		r.addTimelineCompletedTurn(ts.Local().Truncate(bucket))
	}
}

func isStreamClose(raw string) bool {
	return strings.Contains(raw, "codex_core::client: close") &&
		strings.Contains(raw, "model_client.stream_responses_websocket{") &&
		!strings.Contains(raw, "model_client.websocket_connection{") &&
		!strings.Contains(raw, "ToolCall:")
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

func noiseNameKey(lower string) string {
	switch {
	case strings.Contains(lower, "ignoring interface.icon_"):
		return "codex.noise.plugin_icon"
	case strings.Contains(lower, "ignoring interface.defaultprompt"):
		return "codex.noise.default_prompt"
	case strings.Contains(lower, "failed to unwatch"):
		return "codex.noise.file_watcher"
	case strings.Contains(lower, "failed to paste image"):
		return "codex.noise.clipboard"
	default:
		return "codex.noise.other"
	}
}

func isCodexToolEcho(raw string) bool {
	return strings.Contains(raw, "ToolCall:") || strings.Contains(raw, `event.name="codex.tool_result"`)
}
